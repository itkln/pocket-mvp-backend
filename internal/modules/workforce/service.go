package workforce

import (
	"context"
	"net/mail"
	"regexp"
	"strings"

	"pocket-mvp-backend/internal/appfault"
)

var (
	staffRolePattern   = regexp.MustCompile(`^(manager|waiter)$`)
	staffStatusPattern = regexp.MustCompile(`^(invited|active|inactive)$`)
)

type VenueAuthorizer interface {
	RequireOwner(context.Context, string, string) error
}

type Service struct {
	repository Repository
	authorizer VenueAuthorizer
	invites    InvitationOptions
}

func NewService(repository Repository, authorizer VenueAuthorizer, options ...InvitationOptions) *Service {
	s := &Service{repository: repository, authorizer: authorizer}
	if len(options) > 0 {
		s.invites = options[0]
	}
	return s
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
	address, emailErr := mail.ParseAddress(input.Email)
	if input.DisplayName == "" || emailErr != nil || address.Address != input.Email || strings.ContainsAny(input.Email, "\r\n") || !validRole(input.Role) {
		return StaffMember{}, appfault.ErrInvalidInput
	}
	member, err := s.repository.Create(ctx, ownerID, venueID, input)
	if err == nil && s.invites.Sender != nil {
		member.InviteDelivery = "sent"
		if sendErr := s.SendInvitation(ctx, ownerID, venueID, member.ID); sendErr != nil {
			member.InviteDelivery = "failed"
		}
	}
	return member, err
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
