package reporting

import "context"

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

func (s *Service) Dashboard(ctx context.Context, ownerID, venueID string) (Dashboard, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return Dashboard{}, err
	}
	return s.repository.Dashboard(ctx, venueID)
}
