package ordering

import (
	"strings"

	"pocket-mvp-backend/internal/lifecycle"

	"pocket-mvp-backend/internal/access"
)

var (
	// Statuses a client may request, in workflow order; cancellation is allowed from any non-final status.
	settableStatuses = []string{lifecycle.OrderNew, lifecycle.OrderPreparing, lifecycle.OrderServed, lifecycle.OrderCompleted, lifecycle.OrderCancelled}
	activeStatuses   = []string{lifecycle.OrderNew, lifecycle.OrderPreparing, lifecycle.OrderServed}
	finalStatuses    = []string{lifecycle.OrderCompleted, lifecycle.OrderCancelled}
	channels         = []string{lifecycle.ChannelDineIn, lifecycle.ChannelPreorder, lifecycle.ChannelOnline}
	// The only status each status may advance to: the waiter marks the order as being prepared
	// (passed to the kitchen), served, then completes it.
	nextStatus = map[string]string{
		lifecycle.OrderNew:       lifecycle.OrderPreparing,
		lifecycle.OrderPreparing: lifecycle.OrderServed,
		lifecycle.OrderServed:    lifecycle.OrderCompleted,
	}
)

func isSettableStatus(status string) bool { return contains(settableStatuses, status) }
func isChannel(channel string) bool       { return contains(channels, channel) }
func isFinalStatus(status string) bool    { return contains(finalStatuses, status) }

func next(current string) string { return nextStatus[current] }

// Every role with the orders capability runs the whole workflow, cancellation included; there is
// no kitchen role: waiters pass orders to the kitchen outside Pocket.
func roleMaySet(role, _ string) bool { return access.Allows(role, access.ManageOrders) }

// sqlStatuses renders trusted status constants as a SQL IN list.
func sqlStatuses(statuses []string) string {
	return "'" + strings.Join(statuses, "','") + "'"
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
