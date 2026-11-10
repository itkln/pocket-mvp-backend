package billing

import (
	"context"
	"pocket-mvp-backend/internal/lifecycle"
	"pocket-mvp-backend/internal/validate"

	"pocket-mvp-backend/internal/appfault"
)

func (s *Service) GetSubscription(ctx context.Context, ownerID string) (*Subscription, error) {
	return s.repository.GetSubscription(ctx, ownerID)
}

func (s *Service) UpsertSubscription(ctx context.Context, ownerID string, input SubscriptionInput) (Subscription, error) {
	if !validate.OneOf(input.Plan, lifecycle.Plans...) || !validate.OneOf(input.BillingCycle, lifecycle.BillingCycles...) {
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
	case lifecycle.PlanStart:
		value := 1
		return &value
	case lifecycle.PlanBusiness:
		value := 3
		return &value
	default:
		return nil
	}
}
