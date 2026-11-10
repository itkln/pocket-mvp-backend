package demodata

import (
	"context"
	"fmt"
	"pocket-mvp-backend/internal/money"
	"time"

	"github.com/jackc/pgx/v5"
)

type paymentSeed struct {
	ID          string
	OrderID     string
	ProviderID  string
	Status      string
	AmountMinor money.Minor
	Refunded    int
	CreatedAt   time.Time
}

func seedPayments(ctx context.Context, tx pgx.Tx, venueID string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO venue_payment_accounts (
			venue_id, provider, provider_account_id, onboarding_status,
			charges_enabled, payouts_enabled, metadata
		)
		VALUES ($1, 'stripe', $2, 'enabled', true, true, '{"demo":true}')`,
		venueID, "acct_demo_"+venueID,
	); err != nil {
		return fmt.Errorf("seed payment account: %w", err)
	}
	for _, payment := range demoPayments(venueID) {
		_, err := tx.Exec(ctx, `
			INSERT INTO payments (
				id, venue_id, order_id, provider, provider_payment_id, status,
				amount_minor, refunded_minor, currency, provider_payload,
				paid_at, created_at, updated_at
			)
			VALUES (
				$1, $2, $3, 'stripe', $4, $5, $6, $7, 'EUR', '{"demo":true}',
				CASE WHEN $5 IN ('paid', 'refunded') THEN $8::timestamptz ELSE NULL END,
				$8::timestamptz, $8::timestamptz
			)`,
			payment.ID, venueID, payment.OrderID, payment.ProviderID, payment.Status,
			payment.AmountMinor, payment.Refunded, payment.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("seed payment %s: %w", payment.ID, err)
		}
	}
	return nil
}

func demoPayments(venueID string) []paymentSeed {
	now := time.Now().UTC()
	if venueID == secondaryVenueID {
		return []paymentSeed{
			{ID: "80000000-0000-4000-8000-000000000101", OrderID: "70000000-0000-4000-8000-000000000102", ProviderID: "pi_demo_coffee_1", Status: "paid", AmountMinor: 1210, CreatedAt: now.Add(-60 * time.Minute)},
			{ID: "80000000-0000-4000-8000-000000000102", OrderID: "70000000-0000-4000-8000-000000000103", ProviderID: "pi_demo_coffee_2", Status: "processing", AmountMinor: 740, CreatedAt: now.Add(-4 * time.Minute)},
		}
	}
	return []paymentSeed{
		{ID: "80000000-0000-4000-8000-000000000001", OrderID: "70000000-0000-4000-8000-000000000004", ProviderID: "pi_demo_bistro_1", Status: "processing", AmountMinor: 7820, CreatedAt: now.Add(-40 * time.Minute)},
		{ID: "80000000-0000-4000-8000-000000000002", OrderID: "70000000-0000-4000-8000-000000000005", ProviderID: "pi_demo_bistro_2", Status: "paid", AmountMinor: 5370, CreatedAt: now.Add(-70 * time.Minute)},
		{ID: "80000000-0000-4000-8000-000000000003", OrderID: "70000000-0000-4000-8000-000000000006", ProviderID: "pi_demo_bistro_3", Status: "paid", AmountMinor: 9550, CreatedAt: now.Add(-125 * time.Minute)},
		{ID: "80000000-0000-4000-8000-000000000004", OrderID: "70000000-0000-4000-8000-000000000008", ProviderID: "pi_demo_bistro_4", Status: "paid", AmountMinor: 12140, CreatedAt: now.Add(-185 * time.Minute)},
		{ID: "80000000-0000-4000-8000-000000000005", OrderID: "70000000-0000-4000-8000-000000000009", ProviderID: "pi_demo_bistro_5", Status: "refunded", AmountMinor: 1790, Refunded: 1790, CreatedAt: now.Add(-24 * time.Hour)},
	}
}

func seedSubscription(ctx context.Context, tx pgx.Tx, ownerID string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO workspace_subscriptions (
			id, owner_user_id, provider, provider_customer_id, provider_subscription_id,
			plan, billing_cycle, status, venue_limit, trial_ends_at, current_period_ends_at
		)
		VALUES (
			'a0000000-0000-4000-8000-000000000001', $1, 'demo', 'cus_demo_owner',
			'sub_demo_business', 'business', 'monthly', 'trialing', 3,
			now() + interval '11 days', now() + interval '1 month'
		)`, ownerID)
	if err != nil {
		return fmt.Errorf("seed demo subscription: %w", err)
	}
	return nil
}
