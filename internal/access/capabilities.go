package access

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CapabilityReader builds the account capability projection consumed by identity.
// It is intentionally behind an interface so identity does not own venue schemas.
type CapabilityReader struct {
	db *pgxpool.Pool
}

func NewCapabilityReader(db *pgxpool.Pool) *CapabilityReader {
	return &CapabilityReader{db: db}
}

func (r *CapabilityReader) ListCapabilities(ctx context.Context, userID string) ([]string, error) {
	capabilities := []string{"customer"}
	var ownsVenue, worksAtVenue bool
	if err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM venues WHERE owner_user_id = $1 AND deleted_at IS NULL
		)`, userID).Scan(&ownsVenue); err != nil {
		return nil, fmt.Errorf("load owner capability: %w", err)
	}
	if err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM venue_staff WHERE user_id = $1 AND status = 'active'
		)`, userID).Scan(&worksAtVenue); err != nil {
		return nil, fmt.Errorf("load staff capability: %w", err)
	}
	if ownsVenue {
		capabilities = append(capabilities, "owner")
	}
	if worksAtVenue {
		capabilities = append(capabilities, "staff")
	}
	return capabilities, nil
}
