package workforce

import (
	"context"
	"net/mail"
	"strings"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/lifecycle"
	"pocket-mvp-backend/internal/validate"

	"pocket-mvp-backend/internal/appfault"
)

type VenueAuthorizer interface {
	Require(context.Context, string, string, access.Capability) error
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
	if err := s.authorizer.Require(ctx, ownerID, venueID, access.ManageVenue); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, venueID)
}

func (s *Service) Create(ctx context.Context, ownerID, venueID string, input StaffInput) (StaffMember, error) {
	if err := s.authorizer.Require(ctx, ownerID, venueID, access.ManageVenue); err != nil {
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
		member.InviteDelivery = deliverySent
		if sendErr := s.SendInvitation(ctx, ownerID, venueID, member.ID); sendErr != nil {
			member.InviteDelivery = deliveryFailed
		}
	}
	return member, err
}

func (s *Service) Update(ctx context.Context, ownerID, venueID, staffID string, input StaffInput) (StaffMember, error) {
	if err := s.authorizer.Require(ctx, ownerID, venueID, access.ManageVenue); err != nil {
		return StaffMember{}, err
	}
	if !validRole(input.Role) {
		return StaffMember{}, appfault.ErrInvalidInput
	}
	if input.Status == "" {
		input.Status = lifecycle.StaffInvited
	}
	if !validate.OneOf(input.Status, lifecycle.StaffStatuses...) {
		return StaffMember{}, appfault.ErrInvalidInput
	}
	return s.repository.Update(ctx, venueID, staffID, input)
}

func (s *Service) Delete(ctx context.Context, ownerID, venueID, staffID string) error {
	if err := s.authorizer.Require(ctx, ownerID, venueID, access.ManageVenue); err != nil {
		return err
	}
	return s.repository.Delete(ctx, venueID, staffID)
}

// Whether the invitation email reached the mail server when a member was added.
const (
	deliverySent   = "sent"
	deliveryFailed = "failed"
)

func validRole(value string) bool {
	return validate.OneOf(value, access.RoleManager, access.RoleWaiter)
}
