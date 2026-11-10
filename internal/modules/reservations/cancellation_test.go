package reservations

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCancellationEligibility(t *testing.T) {
	start := time.Date(2026, 10, 25, 2, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name, status, policy, preorder, reason string
		now                                    time.Time
		payment                                bool
	}{
		{"default plain future", "confirmed", "", "", "", start.Add(-time.Minute), false},
		{"default null policy", "confirmed", "null", "accepted", "", start.Add(-time.Minute), false},
		{"default draft", "confirmed", "", "draft", "", start.Add(-time.Minute), false},
		{"default new", "confirmed", "", "new", "", start.Add(-time.Minute), false},
		{"default cancelled preorder", "confirmed", "", "cancelled", "", start.Add(-time.Minute), false},
		{"default preparing", "confirmed", "", "preparing", "cancellation_preorder_locked", start.Add(-time.Minute), false},
		{"default ready", "confirmed", "", "ready", "cancellation_preorder_locked", start.Add(-time.Minute), false},
		{"default served", "confirmed", "", "served", "cancellation_preorder_locked", start.Add(-time.Minute), false},
		{"default completed", "confirmed", "", "completed", "cancellation_preorder_locked", start.Add(-time.Minute), false},
		{"default at visit", "confirmed", "", "", "cancellation_deadline_passed", start, false},
		{"default after visit", "confirmed", "", "", "cancellation_deadline_passed", start.Add(time.Minute), false},
		{"invalid explicit policy", "confirmed", "{}", "new", "cancellation_policy_unset", start.Add(-time.Minute), false},
		{"malformed explicit policy", "confirmed", `{"cutoff_minutes":"invalid"}`, "new", "cancellation_policy_unset", start.Add(-time.Minute), false},
		{"plain future", "confirmed", `{"cutoff_minutes":0,"preorder_until":"preparing"}`, "", "", start.Add(-time.Minute), false},
		{"at visit", "confirmed", `{"cutoff_minutes":0,"preorder_until":"preparing"}`, "", "cancellation_deadline_passed", start, false},
		{"venue cutoff", "confirmed", `{"cutoff_minutes":120,"preorder_until":"preparing"}`, "new", "cancellation_deadline_passed", start.Add(-time.Hour), false},
		{"before cutoff", "confirmed", `{"cutoff_minutes":120,"preorder_until":"preparing"}`, "accepted", "", start.Add(-3 * time.Hour), false},
		{"preparing", "confirmed", `{"cutoff_minutes":0,"preorder_until":"preparing"}`, "preparing", "cancellation_preorder_locked", start.Add(-time.Hour), false},
		{"accepted rule", "confirmed", `{"cutoff_minutes":0,"preorder_until":"accepted"}`, "accepted", "cancellation_preorder_locked", start.Add(-time.Hour), false},
		{"visit rule", "confirmed", `{"cutoff_minutes":0,"preorder_until":"visit"}`, "ready", "", start.Add(-time.Hour), false},
		{"payment started", "confirmed", `{"cutoff_minutes":0,"preorder_until":"preparing"}`, "new", "cancellation_payment_started", start.Add(-time.Hour), true},
		{"already seated", "seated", `{"cutoff_minutes":0,"preorder_until":"preparing"}`, "new", "cancellation_status_blocked", start.Add(-time.Hour), false},
		{"bad policy", "confirmed", `{"cutoff_minutes":-1,"preorder_until":"unknown"}`, "new", "cancellation_policy_unset", start.Add(-time.Hour), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			states := []string{}
			if test.preorder != "" {
				states = append(states, test.preorder)
			}
			result := cancellationEligibility(test.status, start, test.now, json.RawMessage(test.policy), states, test.payment)
			if result.Reason != test.reason || result.Allowed != (test.reason == "") {
				t.Fatalf("eligibility: %+v, want reason %q", result, test.reason)
			}
			if (test.policy == "" || test.policy == "null") && (result.Deadline == nil || !result.Deadline.Equal(start)) {
				t.Fatalf("default deadline: %v, want visit start %v", result.Deadline, start)
			}
		})
	}
}
