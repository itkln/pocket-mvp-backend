package floorplan

import (
	"context"
	"encoding/json"

	"pocket-mvp-backend/internal/appfault"
)

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

func (s *Service) Get(ctx context.Context, ownerID, venueID string) (json.RawMessage, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	return s.repository.Get(ctx, venueID)
}

func (s *Service) Save(ctx context.Context, ownerID, venueID string, plan json.RawMessage) (json.RawMessage, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	if len(plan) == 0 || !json.Valid(plan) {
		return nil, appfault.ErrInvalidInput
	}
	if err := s.repository.Save(ctx, ownerID, venueID, plan); err != nil {
		return nil, err
	}
	return plan, nil
}
