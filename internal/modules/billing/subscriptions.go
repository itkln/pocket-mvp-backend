package billing

import (
	"context"
	"errors"
	"regexp"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
)

var (
	planPattern         = regexp.MustCompile(`^(start|business|pro)$`)
	billingCyclePattern = regexp.MustCompile(`^(monthly|yearly)$`)
)

func (s *Service) GetSubscription(ctx context.Context, ownerID string) (*Subscription, error) {
	var subscription Subscription
	err := s.db.QueryRow(ctx, `
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

func (s *Service) UpsertSubscription(ctx context.Context, ownerID string, input SubscriptionInput) (Subscription, error) {
	if !planPattern.MatchString(input.Plan) || !billingCyclePattern.MatchString(input.BillingCycle) {
		return Subscription{}, appfault.ErrInvalidInput
	}
	venueLimit := limitForPlan(input.Plan)
	var venueLimitValue any
	if venueLimit != nil {
		venueLimitValue = *venueLimit
	}

	var venueCount int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM venues WHERE owner_user_id = $1 AND deleted_at IS NULL`, ownerID).Scan(&venueCount); err != nil {
		return Subscription{}, err
	}
	if venueLimit != nil && venueCount > *venueLimit {
		return Subscription{}, appfault.ErrConflict
	}

	tx, err := s.db.Begin(ctx)
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

func limitForPlan(plan string) *int {
	switch plan {
	case "start":
		value := 1
		return &value
	case "business":
		value := 3
		return &value
	default:
		return nil
	}
}
