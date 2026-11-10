package reporting

import (
	"context"
	"pocket-mvp-backend/internal/access"
)

type VenueAuthorizer interface {
	Require(context.Context, string, string, access.Capability) error
}

type Service struct {
	repository Repository
	authorizer VenueAuthorizer
}

func NewService(repository Repository, authorizer VenueAuthorizer) *Service {
	return &Service{repository: repository, authorizer: authorizer}
}

func (s *Service) Dashboard(ctx context.Context, ownerID, venueID string) (Dashboard, error) {
	if err := s.authorizer.Require(ctx, ownerID, venueID, access.ManageVenue); err != nil {
		return Dashboard{}, err
	}
	return s.repository.Dashboard(ctx, venueID)
}
