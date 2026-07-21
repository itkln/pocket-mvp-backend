package billing

import "time"

type Payment struct {
	ID            string     `json:"id"`
	Provider      string     `json:"provider"`
	Status        string     `json:"status"`
	AmountMinor   int        `json:"amount_minor"`
	RefundedMinor int        `json:"refunded_minor"`
	Currency      string     `json:"currency"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type Subscription struct {
	ID                string     `json:"id"`
	Plan              string     `json:"plan"`
	BillingCycle      string     `json:"billing_cycle"`
	Status            string     `json:"status"`
	VenueLimit        *int       `json:"venue_limit,omitempty"`
	TrialEndsAt       *time.Time `json:"trial_ends_at,omitempty"`
	CurrentPeriodEnds *time.Time `json:"current_period_ends_at,omitempty"`
}

type SubscriptionInput struct {
	Plan         string `json:"plan"`
	BillingCycle string `json:"billing_cycle"`
}
