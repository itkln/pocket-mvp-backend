package demodata

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func resetDemoWorkspace(ctx context.Context, tx pgx.Tx, ownerID string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM workspace_subscriptions WHERE owner_user_id = $1`, ownerID); err != nil {
		return fmt.Errorf("reset demo subscription: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM venues
		WHERE id IN ($1, $2)
		   OR (owner_user_id = $3 AND slug IN ('pocket-demo-bistro', 'pocket-demo-roastery'))`,
		mainVenueID, secondaryVenueID, ownerID,
	); err != nil {
		return fmt.Errorf("reset demo venues: %w", err)
	}
	return nil
}

func seedVenue(ctx context.Context, tx pgx.Tx, ownerID string, venue venueSeed) error {
	settings, err := json.Marshal(venue.Settings)
	if err != nil {
		return fmt.Errorf("encode settings for %s: %w", venue.Name, err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO venues (
			id, owner_user_id, slug, name, description, cuisine_type, phone, email,
			address_line1, city, postal_code, country_code, timezone, currency,
			status, settings, opened_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 'active', $15, now() - interval '6 months')`,
		venue.ID, ownerID, venue.Slug, venue.Name, venue.Description, venue.Cuisine,
		venue.Phone, venue.Email, venue.Address, venue.City, venue.PostalCode,
		venue.CountryCode, venue.Timezone, venue.Currency, settings,
	)
	if err != nil {
		return fmt.Errorf("seed venue %s: %w", venue.Name, err)
	}
	return nil
}
