package access

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/appfault"
)

// VenueAuthorizer is the single access policy used by venue-scoped modules.
// A future service can replace it with token claims or a local membership projection.
type VenueAuthorizer struct {
	db *pgxpool.Pool
}

func NewVenueAuthorizer(db *pgxpool.Pool) *VenueAuthorizer {
	return &VenueAuthorizer{db: db}
}

// Require checks that the user's role at the venue grants the capability. A user without it gets
// ErrNotFound, exactly like a user with no relation to the venue, so the venue is not disclosed.
func (a *VenueAuthorizer) Require(ctx context.Context, userID, venueID string, capability Capability) error {
	role, err := ResolveOrderRole(ctx, a.db, userID, venueID, false)
	if err != nil {
		return err
	}
	if !Allows(role, capability) {
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
		return RoleOwner, nil
	}
	var role string
	err = db.QueryRow(ctx, "SELECT role FROM venue_staff WHERE venue_id=$1 AND user_id=$2 AND status='active' AND removed_at IS NULL"+suffix, venueID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", appfault.ErrNotFound
	}
	return role, err
}
