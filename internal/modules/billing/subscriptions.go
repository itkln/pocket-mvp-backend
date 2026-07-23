package billing

import (
	"context"
	"regexp"

	"pocket-mvp-backend/internal/appfault"
)

var (
	planPattern         = regexp.MustCompile(`^(start|business|pro)$`)
	billingCyclePattern = regexp.MustCompile(`^(monthly|yearly)$`)
)

func (s *Service) GetSubscription(ctx context.Context, ownerID string) (*Subscription, error) {
	return s.repository.GetSubscription(ctx, ownerID)
}

func (s *Service) UpsertSubscription(ctx context.Context, ownerID string, input SubscriptionInput) (Subscription, error) {
	if !planPattern.MatchString(input.Plan) || !billingCyclePattern.MatchString(input.BillingCycle) {
		return Subscription{}, appfault.ErrInvalidInput
	}
	venueLimit := limitForPlan(input.Plan)
	venueCount, err := s.repository.CountVenues(ctx, ownerID)
	if err != nil {
		return Subscription{}, err
	}
	if venueLimit != nil && venueCount > *venueLimit {
		return Subscription{}, appfault.ErrConflict
	}
	return s.repository.ReplaceSubscription(ctx, ownerID, input, venueLimit)
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
