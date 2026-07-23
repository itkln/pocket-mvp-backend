package workforce

import (
	"context"
	"regexp"
	"strings"

	"pocket-mvp-backend/internal/appfault"
)

var (
	staffRolePattern   = regexp.MustCompile(`^(manager|waiter|kitchen|viewer)$`)
	staffStatusPattern = regexp.MustCompile(`^(invited|active|inactive)$`)
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

func (s *Service) List(ctx context.Context, ownerID, venueID string) ([]StaffMember, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, venueID)
}

func (s *Service) Create(ctx context.Context, ownerID, venueID string, input StaffInput) (StaffMember, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return StaffMember{}, err
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.DisplayName == "" || !strings.Contains(input.Email, "@") || !validRole(input.Role) {
		return StaffMember{}, appfault.ErrInvalidInput
	}
	return s.repository.Create(ctx, ownerID, venueID, input)
}

func (s *Service) Update(ctx context.Context, ownerID, venueID, staffID string, input StaffInput) (StaffMember, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return StaffMember{}, err
	}
	if !validRole(input.Role) {
		return StaffMember{}, appfault.ErrInvalidInput
	}
	if input.Status == "" {
		input.Status = "invited"
	}
	if !staffStatusPattern.MatchString(input.Status) {
		return StaffMember{}, appfault.ErrInvalidInput
	}
	return s.repository.Update(ctx, venueID, staffID, input)
}

func (s *Service) Delete(ctx context.Context, ownerID, venueID, staffID string) error {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return err
	}
	return s.repository.Delete(ctx, venueID, staffID)
}

func validRole(value string) bool {
	return staffRolePattern.MatchString(value)
}
