package feedback

import (
	"context"
	"strings"
	"unicode/utf8"

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

func (s *Service) List(ctx context.Context, ownerID, venueID string) ([]Review, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, venueID)
}

func (s *Service) Reply(ctx context.Context, ownerID, venueID, reviewID, reply string) (Review, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return Review{}, err
	}
	reply = strings.TrimSpace(reply)
	if reply == "" || utf8.RuneCountInString(reply) > 2000 {
		return Review{}, appfault.ErrInvalidInput
	}
	return s.repository.Reply(ctx, ownerID, venueID, reviewID, reply)
}
