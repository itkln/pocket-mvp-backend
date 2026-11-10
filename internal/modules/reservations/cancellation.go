package reservations

import (
	"encoding/json"
	"time"

	"pocket-mvp-backend/internal/lifecycle"
	"pocket-mvp-backend/internal/validate"
)

// Cancellation is computed by the server and checked again under row locks.
type Cancellation struct {
	Allowed  bool       `json:"allowed"`
	Reason   string     `json:"reason,omitempty"`
	Deadline *time.Time `json:"deadline,omitempty"`
}

type CancellationDenied struct{ Reason string }

func (e *CancellationDenied) Error() string { return e.Reason }

// How long a booking with a preorder may be cancelled: until the waiter starts preparing it, or
// until the visit. "accepted" is a legacy value from the kitchen workflow and means "preparing".
const (
	preorderUntilAccepted  = "accepted"
	preorderUntilPreparing = "preparing"
	preorderUntilVisit     = "visit"
)

type cancellationPolicy struct {
	CutoffMinutes int    `json:"cutoff_minutes"`
	PreorderUntil string `json:"preorder_until"`
}

// Approved on 2026-10-09: until the visit starts, before preorder preparation.
// A venue may override this policy through settings.guest_cancellation.
var defaultCancellationPolicy = &cancellationPolicy{
	CutoffMinutes: 0,
	PreorderUntil: preorderUntilPreparing,
}

func cancellationEligibility(status string, startsAt, now time.Time, rawPolicy json.RawMessage, preorderStatuses []string, paymentStarted bool) Cancellation {
	result := Cancellation{}
	if status != lifecycle.ReservationConfirmed {
		result.Reason = "cancellation_status_blocked"
		return result
	}
	policy := defaultCancellationPolicy
	if len(rawPolicy) > 0 && string(rawPolicy) != "null" {
		var configured cancellationPolicy
		if err := json.Unmarshal(rawPolicy, &configured); err != nil {
			result.Reason = "cancellation_policy_unset"
			return result
		}
		policy = &configured
	}
	if policy == nil || policy.CutoffMinutes < 0 || policy.CutoffMinutes > 43200 ||
		!validate.OneOf(policy.PreorderUntil, preorderUntilAccepted, preorderUntilPreparing, preorderUntilVisit) {
		result.Reason = "cancellation_policy_unset"
		return result
	}
	deadline := startsAt.Add(-time.Duration(policy.CutoffMinutes) * time.Minute)
	result.Deadline = &deadline
	if !now.Before(deadline) {
		result.Reason = "cancellation_deadline_passed"
		return result
	}
	if paymentStarted {
		result.Reason = "cancellation_payment_started"
		return result
	}
	// There is no accepted step any more: "accepted" and "preparing" both allow cancelling
	// until the waiter starts preparing the preorder. Both values stay valid for stored policies.
	for _, state := range preorderStatuses {
		allowed := validate.OneOf(state, lifecycle.OrderDraft, lifecycle.OrderNew, lifecycle.OrderCancelled) || policy.PreorderUntil == preorderUntilVisit
		if !allowed {
			result.Reason = "cancellation_preorder_locked"
			return result
		}
	}
	result.Allowed = true
	return result
}
