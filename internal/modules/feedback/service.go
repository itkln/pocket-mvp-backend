package feedback

import (
	"context"
	"pocket-mvp-backend/internal/access"
	"strings"
	"unicode/utf8"

	"pocket-mvp-backend/internal/appfault"
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

func (s *Service) List(ctx context.Context, ownerID, venueID string) ([]Review, error) {
	if err := s.authorizer.Require(ctx, ownerID, venueID, access.ManageVenue); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, venueID)
}

func (s *Service) Reply(ctx context.Context, ownerID, venueID, reviewID, reply string) (Review, error) {
	if err := s.authorizer.Require(ctx, ownerID, venueID, access.ManageVenue); err != nil {
		return Review{}, err
	}
	reply = strings.TrimSpace(reply)
	if reply == "" || utf8.RuneCountInString(reply) > 2000 {
		return Review{}, appfault.ErrInvalidInput
	}
	return s.repository.Reply(ctx, ownerID, venueID, reviewID, reply)
}
