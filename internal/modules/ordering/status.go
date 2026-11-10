package ordering

import (
	"strings"

	"pocket-mvp-backend/internal/access"
)

// Order statuses. A status is also a monotonic concurrency token: final orders cannot be reopened.
const (
	StatusDraft     = "draft"
	StatusNew       = "new"
	StatusPreparing = "preparing"
	StatusServed    = "served"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

// Order channels.
const (
	ChannelDineIn   = "dine_in"
	ChannelOnline   = "online"
	ChannelPreorder = "preorder"
)

var (
	// Statuses a client may request, in workflow order; cancellation is allowed from any non-final status.
	settableStatuses = []string{StatusNew, StatusPreparing, StatusServed, StatusCompleted, StatusCancelled}
	activeStatuses   = []string{StatusNew, StatusPreparing, StatusServed}
	finalStatuses    = []string{StatusCompleted, StatusCancelled}
	channels         = []string{ChannelDineIn, ChannelPreorder, ChannelOnline}
	// The only status each status may advance to: the waiter marks the order as being prepared
	// (passed to the kitchen), served, then completes it.
	nextStatus = map[string]string{
		StatusNew:       StatusPreparing,
		StatusPreparing: StatusServed,
		StatusServed:    StatusCompleted,
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
