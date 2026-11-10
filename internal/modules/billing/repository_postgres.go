package billing

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/appfault"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) ListPayments(ctx context.Context, venueID string) ([]Payment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, provider, status, amount_minor, refunded_minor, currency, paid_at, created_at
		FROM payments
		WHERE venue_id = $1
		ORDER BY created_at DESC
		LIMIT 200`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Payment{}
	for rows.Next() {
		var payment Payment
		if err := rows.Scan(
			&payment.ID, &payment.Provider, &payment.Status, &payment.AmountMinor,
			&payment.RefundedMinor, &payment.Currency, &payment.PaidAt, &payment.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, payment)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) GetSubscription(ctx context.Context, ownerID string) (*Subscription, error) {
	var subscription Subscription
	err := r.db.QueryRow(ctx, `
		SELECT id::text, plan, billing_cycle, status, venue_limit, trial_ends_at, current_period_ends_at
		FROM workspace_subscriptions
		WHERE owner_user_id = $1 AND status IN ('trialing', 'active', 'past_due')
		ORDER BY created_at DESC
		LIMIT 1`, ownerID,
	).Scan(
		&subscription.ID, &subscription.Plan, &subscription.BillingCycle, &subscription.Status,
		&subscription.VenueLimit, &subscription.TrialEndsAt, &subscription.CurrentPeriodEnds,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &subscription, err
}

func (r *PostgresRepository) CountVenues(ctx context.Context, ownerID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM venues WHERE owner_user_id = $1 AND deleted_at IS NULL`, ownerID).Scan(&count)
	return count, err
}

func (r *PostgresRepository) ReplaceSubscription(ctx context.Context, ownerID string, input SubscriptionInput, venueLimit *int) (Subscription, error) {
	var venueLimitValue any
	if venueLimit != nil {
		venueLimitValue = *venueLimit
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Subscription{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `
		UPDATE workspace_subscriptions
		SET status = 'cancelled', cancelled_at = now()
		WHERE owner_user_id = $1 AND status IN ('trialing', 'active', 'past_due')`, ownerID); err != nil {
		return Subscription{}, err
	}

	var subscription Subscription
	err = tx.QueryRow(ctx, `
		INSERT INTO workspace_subscriptions (
			owner_user_id, provider, plan, billing_cycle, status, venue_limit,
			trial_ends_at, current_period_ends_at
		)
		VALUES (
			$1, 'manual', $2, $3, 'trialing', $4,
			now() + interval '14 days',
			now() + CASE WHEN $3 = 'yearly' THEN interval '1 year' ELSE interval '1 month' END
		)
		RETURNING id::text, plan, billing_cycle, status, venue_limit, trial_ends_at, current_period_ends_at`,
		ownerID, input.Plan, input.BillingCycle, venueLimitValue,
	).Scan(
		&subscription.ID, &subscription.Plan, &subscription.BillingCycle, &subscription.Status,
		&subscription.VenueLimit, &subscription.TrialEndsAt, &subscription.CurrentPeriodEnds,
	)
	if err != nil {
		return Subscription{}, appfault.MapWriteError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Subscription{}, err
	}
	return subscription, nil
}
