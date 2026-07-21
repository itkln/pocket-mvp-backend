package access

import (
	"context"

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
