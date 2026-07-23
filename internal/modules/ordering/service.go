package ordering

import (
	"context"
	"regexp"

	"pocket-mvp-backend/internal/appfault"
)

var orderStatusPattern = regexp.MustCompile(`^(new|accepted|preparing|ready|served|completed|cancelled)$`)

type VenueAuthorizer interface {
	RequireOwner(context.Context, string, string) error
}

type Service struct {
	repository Repository
	authorizer VenueAuthorizer
}

func NewService(repository Repository, authorizer VenueAuthorizer) *Service {
	return &Service{repository: repository, authorizer: authorizer}
}

func (s *Service) List(ctx context.Context, ownerID, venueID string) ([]Order, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, venueID)
}

func (s *Service) UpdateStatus(ctx context.Context, ownerID, venueID, orderID, status string) (Order, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return Order{}, err
	}
	if !orderStatusPattern.MatchString(status) {
		return Order{}, appfault.ErrInvalidInput
	}
	return s.repository.UpdateStatus(ctx, venueID, orderID, status)
}
