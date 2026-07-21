package workforce

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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
	db         *pgxpool.Pool
	authorizer VenueAuthorizer
}

func NewService(db *pgxpool.Pool, authorizer VenueAuthorizer) *Service {
	return &Service{db: db, authorizer: authorizer}
}

func (s *Service) List(ctx context.Context, ownerID, venueID string) ([]StaffMember, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT id::text, COALESCE(display_name, ''), invited_email, role, status, invited_at, accepted_at
		FROM venue_staff
		WHERE venue_id = $1 AND status <> 'removed'
		ORDER BY created_at`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []StaffMember{}
	for rows.Next() {
		var member StaffMember
		if err := rows.Scan(&member.ID, &member.DisplayName, &member.Email, &member.Role, &member.Status, &member.InvitedAt, &member.AcceptedAt); err != nil {
			return nil, err
		}
		result = append(result, member)
	}
	return result, rows.Err()
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

	var member StaffMember
	err := s.db.QueryRow(ctx, `
		INSERT INTO venue_staff (venue_id, display_name, invited_email, role, status, invited_by_user_id)
		VALUES ($1, $2, $3, $4, 'invited', $5)
		RETURNING id::text, display_name, invited_email, role, status, invited_at, accepted_at`,
		venueID, input.DisplayName, input.Email, input.Role, ownerID,
	).Scan(&member.ID, &member.DisplayName, &member.Email, &member.Role, &member.Status, &member.InvitedAt, &member.AcceptedAt)
	return member, appfault.MapWriteError(err)
}

func (s *Service) Update(ctx context.Context, ownerID, venueID, staffID string, input StaffInput) (StaffMember, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return StaffMember{}, err
	}
	if !validRole(input.Role) {
		return StaffMember{}, appfault.ErrInvalidInput
	}
	status := input.Status
	if status == "" {
		status = "invited"
	}
	if !staffStatusPattern.MatchString(status) {
		return StaffMember{}, appfault.ErrInvalidInput
	}

	var member StaffMember
	err := s.db.QueryRow(ctx, `
		UPDATE venue_staff
		SET role = $3, status = $4
		WHERE id = $2 AND venue_id = $1 AND status <> 'removed'
		RETURNING id::text, COALESCE(display_name, ''), invited_email, role, status, invited_at, accepted_at`,
		venueID, staffID, input.Role, status,
	).Scan(&member.ID, &member.DisplayName, &member.Email, &member.Role, &member.Status, &member.InvitedAt, &member.AcceptedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return StaffMember{}, appfault.ErrNotFound
	}
	return member, appfault.MapWriteError(err)
}

func (s *Service) Delete(ctx context.Context, ownerID, venueID, staffID string) error {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return err
	}
	result, err := s.db.Exec(ctx, `
		UPDATE venue_staff
		SET status = 'removed', removed_at = now()
		WHERE id = $2 AND venue_id = $1 AND status <> 'removed'`, venueID, staffID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return appfault.ErrNotFound
	}
	return nil
}

func validRole(value string) bool {
	return staffRolePattern.MatchString(value)
}
