package access

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/appfault"
)

// VenueAuthorizer is the single ownership policy used by venue-scoped modules.
// A future service can replace it with token claims or a local ownership projection.
type VenueAuthorizer struct {
	db *pgxpool.Pool
}

func NewVenueAuthorizer(db *pgxpool.Pool) *VenueAuthorizer {
	return &VenueAuthorizer{db: db}
}

func (a *VenueAuthorizer) RequireOwner(ctx context.Context, userID, venueID string) error {
	var allowed bool
	err := a.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM venues
			WHERE id = $2 AND owner_user_id = $1 AND deleted_at IS NULL
		)`, userID, venueID).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return appfault.ErrNotFound
	}
	return nil
}

// OrderRole resolves only active membership, never a client-selected role.
func (a *VenueAuthorizer) OrderRole(ctx context.Context, userID, venueID string) (string, error) {
	return ResolveOrderRole(ctx, a.db, userID, venueID, false)
}

type orderQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func ResolveOrderRole(ctx context.Context, db orderQuerier, userID, venueID string, lock bool) (string, error) {
	suffix := ""
	if lock {
		suffix = " FOR SHARE"
	}
	var owner string
	err := db.QueryRow(ctx, "SELECT owner_user_id::text FROM venues WHERE id=$1 AND deleted_at IS NULL"+suffix, venueID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", appfault.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if owner == userID {
		return "owner", nil
	}
	var role string
	err = db.QueryRow(ctx, "SELECT role FROM venue_staff WHERE venue_id=$1 AND user_id=$2 AND status='active' AND removed_at IS NULL"+suffix, venueID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", appfault.ErrNotFound
	}
	return role, err
}
