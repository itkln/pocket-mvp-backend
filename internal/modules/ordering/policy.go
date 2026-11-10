package ordering

import "pocket-mvp-backend/internal/appfault"

type StatusChange struct {
	ExpectedStatus string
	ActorID        string
	Reason         string `json:"reason"`
	Comment        string `json:"comment"`
}
type StatusConflict struct{ CurrentStatus string }

func (e *StatusConflict) Error() string { return "order status conflict" }
func (e *StatusConflict) Unwrap() error { return appfault.ErrConflict }

// Status is a monotonic concurrency token: final orders cannot be reopened.
func validateTransition(role, channel, current, target, expected string) error {
	allowed := role == "owner" || role == "manager" ||
		(role == "waiter" && (target == "accepted" || target == "served")) ||
		(role == "kitchen" && (target == "preparing" || target == "ready"))
	if !allowed {
		return appfault.ErrForbidden
	}
	if !orderStatusPattern.MatchString(target) {
		return appfault.ErrInvalidInput
	}
	// Replaying an already-applied action is a no-op, even with its old token.
	if current == target {
		return nil
	}
	if expected != "" && current != expected {
		return &StatusConflict{current}
	}
	next := map[string]string{"new": "accepted", "accepted": "preparing", "preparing": "ready", "ready": "served", "served": "completed"}
	// Pickup is handed over at ready; it has no table service stage.
	if channel == "pickup" {
		next["ready"] = "completed"
	}
	if current == "completed" || current == "cancelled" || (target != "cancelled" && next[current] != target) {
		return &StatusConflict{current}
	}
	return nil
}
