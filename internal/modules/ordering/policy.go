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
	if !roleMaySet(role, target) {
		return appfault.ErrForbidden
	}
	if !isSettableStatus(target) {
		return appfault.ErrInvalidInput
	}
	// Replaying an already-applied action is a no-op, even with its old token.
	if current == target {
		return nil
	}
	if expected != "" && current != expected {
		return &StatusConflict{current}
	}
	if isFinalStatus(current) || (target != StatusCancelled && next(channel, current) != target) {
		return &StatusConflict{current}
	}
	return nil
}
