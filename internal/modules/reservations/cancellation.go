package reservations

import (
	"encoding/json"
	"time"
)

// Cancellation is computed by the server and checked again under row locks.
type Cancellation struct {
	Allowed  bool       `json:"allowed"`
	Reason   string     `json:"reason,omitempty"`
	Deadline *time.Time `json:"deadline,omitempty"`
}

type CancellationDenied struct{ Reason string }

func (e *CancellationDenied) Error() string { return e.Reason }

type cancellationPolicy struct {
	CutoffMinutes int    `json:"cutoff_minutes"`
	PreorderUntil string `json:"preorder_until"`
}

// Approved on 2026-10-09: until the visit starts, before preorder preparation.
// A venue may override this policy through settings.guest_cancellation.
var defaultCancellationPolicy = &cancellationPolicy{
	CutoffMinutes: 0,
	PreorderUntil: "preparing",
}

func cancellationEligibility(status string, startsAt, now time.Time, rawPolicy json.RawMessage, preorderStatuses []string, paymentStarted bool) Cancellation {
	result := Cancellation{}
	if status != "confirmed" {
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
		(policy.PreorderUntil != "accepted" && policy.PreorderUntil != "preparing" && policy.PreorderUntil != "visit") {
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
	for _, state := range preorderStatuses {
		allowed := state == "draft" || state == "new" || state == "cancelled" ||
			(policy.PreorderUntil == "preparing" && state == "accepted") || policy.PreorderUntil == "visit"
		if !allowed {
			result.Reason = "cancellation_preorder_locked"
			return result
		}
	}
	result.Allowed = true
	return result
}
