package favorites

import (
	"context"
	"regexp"

	"pocket-mvp-backend/internal/appfault"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Service struct {
	repository Repository
	venues     VenueAvailability
}

func NewService(repository Repository, venues VenueAvailability) *Service {
	return &Service{repository: repository, venues: venues}
}

func (s *Service) List(ctx context.Context, userID string) ([]string, error) {
	if !uuidPattern.MatchString(userID) {
		return nil, appfault.ErrInvalidInput
	}
	ids, err := s.repository.ListIDs(ctx, userID)
	if ids == nil && err == nil {
		ids = []string{}
	}
	return ids, err
}

func (s *Service) Add(ctx context.Context, userID, venueID string) error {
	if !uuidPattern.MatchString(userID) || !uuidPattern.MatchString(venueID) {
		return appfault.ErrInvalidInput
	}
	published, err := s.venues.IsPublished(ctx, venueID)
	if err != nil {
		return err
	}
	if !published {
		return appfault.ErrNotFound
	}
	return s.repository.Add(ctx, userID, venueID)
}

func (s *Service) Remove(ctx context.Context, userID, venueID string) error {
	if !uuidPattern.MatchString(userID) || !uuidPattern.MatchString(venueID) {
		return appfault.ErrInvalidInput
	}
	return s.repository.Remove(ctx, userID, venueID)
}
