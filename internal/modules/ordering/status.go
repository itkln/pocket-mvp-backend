package ordering

import "strings"

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

// Venue roles that may act on orders. The owner is resolved from the venue, the rest from staff
// membership. There is no kitchen role: waiters pass orders to the kitchen outside Pocket.
const (
	RoleOwner   = "owner"
	RoleManager = "manager"
	RoleWaiter  = "waiter"
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
	// Every venue role may run the whole workflow, cancellation included.
	orderRoles = []string{RoleOwner, RoleManager, RoleWaiter}
)

func isSettableStatus(status string) bool { return contains(settableStatuses, status) }
func isChannel(channel string) bool       { return contains(channels, channel) }
func isFinalStatus(status string) bool    { return contains(finalStatuses, status) }

func next(current string) string { return nextStatus[current] }

func roleMaySet(role, _ string) bool { return contains(orderRoles, role) }

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
