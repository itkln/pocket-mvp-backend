package ordering

import "strings"

// Order statuses. A status is also a monotonic concurrency token: final orders cannot be reopened.
const (
	StatusDraft     = "draft"
	StatusNew       = "new"
	StatusAccepted  = "accepted"
	StatusPreparing = "preparing"
	StatusReady     = "ready"
	StatusServed    = "served"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

// Order channels.
const (
	ChannelDineIn   = "dine_in"
	ChannelOnline   = "online"
	ChannelPickup   = "pickup"
	ChannelPreorder = "preorder"
)

// Venue roles that may act on orders. The owner is resolved from the venue, the rest from staff membership.
const (
	RoleOwner   = "owner"
	RoleManager = "manager"
	RoleWaiter  = "waiter"
	RoleKitchen = "kitchen"
)

var (
	// Statuses a client may request, in workflow order; cancellation is allowed from any non-final status.
	settableStatuses = []string{StatusNew, StatusAccepted, StatusPreparing, StatusReady, StatusServed, StatusCompleted, StatusCancelled}
	activeStatuses   = []string{StatusNew, StatusAccepted, StatusPreparing, StatusReady, StatusServed}
	finalStatuses    = []string{StatusCompleted, StatusCancelled}
	channels         = []string{ChannelDineIn, ChannelPreorder, ChannelOnline, ChannelPickup}
	// The only status each status may advance to.
	nextStatus = map[string]string{
		StatusNew:       StatusAccepted,
		StatusAccepted:  StatusPreparing,
		StatusPreparing: StatusReady,
		StatusReady:     StatusServed,
		StatusServed:    StatusCompleted,
	}
	// Staff roles limited to part of the workflow; owners and managers may set every status.
	roleTargets = map[string][]string{
		RoleWaiter:  {StatusAccepted, StatusServed},
		RoleKitchen: {StatusPreparing, StatusReady},
	}
)

func isSettableStatus(status string) bool { return contains(settableStatuses, status) }
func isChannel(channel string) bool       { return contains(channels, channel) }
func isFinalStatus(status string) bool    { return contains(finalStatuses, status) }

func next(channel, current string) string {
	// Pickup is handed over at ready; it has no table service stage.
	if channel == ChannelPickup && current == StatusReady {
		return StatusCompleted
	}
	return nextStatus[current]
}

func roleMaySet(role, target string) bool {
	if role == RoleOwner || role == RoleManager {
		return true
	}
	return contains(roleTargets[role], target)
}

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
