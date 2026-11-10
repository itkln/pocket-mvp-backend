package workforce

import (
	"context"
	"net/url"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/security"
	"strings"
	"time"
)

type Invitation struct {
	VenueID   string    `json:"venue_id"`
	VenueName string    `json:"venue_name"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}
type InvitationSender interface {
	SendStaffInvitation(context.Context, string, string) error
}
type InvitationOptions struct {
	Sender    InvitationSender
	BaseURL   string
	Protector *security.DataProtector
}
type invitationRepository interface {
	IssueInvitation(context.Context, string, string, string) (string, error)
	ClearInvitation(context.Context, string, string) error
	ReadInvitation(context.Context, string, string, []byte, string, bool) (Invitation, error)
}

func (s *Service) SendInvitation(ctx context.Context, ownerID, venueID, staffID string) error {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return err
	}
	repo, ok := s.repository.(invitationRepository)
	if !ok || s.invites.Sender == nil {
		return appfault.ErrConflict
	}
	token, err := security.NewSessionToken()
	if err != nil {
		return err
	}
	hash := security.HashSessionToken(token)
	email, err := repo.IssueInvitation(ctx, venueID, staffID, hash)
	if err != nil {
		return err
	}
	link := strings.TrimRight(s.invites.BaseURL, "/") + "/ru/invite?token=" + url.QueryEscape(token)
	if err = s.invites.Sender.SendStaffInvitation(ctx, email, link); err != nil {
		_ = repo.ClearInvitation(ctx, staffID, hash)
		return err
	}
	return nil
}
func (s *Service) Invitation(ctx context.Context, userID, email, token string, accept bool) (Invitation, error) {
	repo, ok := s.repository.(invitationRepository)
	if !ok || s.invites.Protector == nil || len(token) < 32 || len(token) > 256 {
		return Invitation{}, appfault.ErrInvalidInput
	}
	return repo.ReadInvitation(ctx, userID, email, s.invites.Protector.Lookup(security.NormalizeEmail(email)), security.HashSessionToken(token), accept)
}
