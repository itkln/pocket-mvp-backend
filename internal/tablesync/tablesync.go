// Package tablesync keeps the normalized venue_tables rows in step with the floor plan the venue
// edits. The plan (venues.settings.floor_plan) is authoritative when present; orders, bills and
// reservations need a venue_tables row to refer to.
package tablesync

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
)

// Table creates or updates the venue_tables row of one table of the saved floor plan, inside the
// caller's transaction (the venue row is locked). A venue with a plan that no longer has the
// table gets appfault.ErrNotFound; a venue without a plan keeps its existing rows untouched.
func Table(ctx context.Context, tx pgx.Tx, venueID, identifier string) error {
	var raw []byte
	var hasPlan bool
	err := tx.QueryRow(ctx, `
 SELECT COALESCE(settings->'floor_plan', '[]'::jsonb), settings ? 'floor_plan'
 FROM venues WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, venueID).Scan(&raw, &hasPlan)
	if errors.Is(err, pgx.ErrNoRows) {
		return appfault.ErrNotFound
	}
	if err != nil {
		return err
	}
	var floors []struct {
		Name   string
		Tables []struct {
			ID    string
			Seats int
			X, Y  float64
		}
	}
	if err = json.Unmarshal(raw, &floors); err != nil {
		return err
	}
	found := false
	for _, floor := range floors {
		for _, table := range floor.Tables {
			if table.ID != identifier {
				continue
			}
			if found {
				return appfault.ErrInvalidInput
			}
			found = true
			var layoutID string
			err = tx.QueryRow(ctx, `
   SELECT id::text FROM venue_layouts
   WHERE venue_id = $1 AND name = $2 ORDER BY created_at LIMIT 1`, venueID, floor.Name).Scan(&layoutID)
			if errors.Is(err, pgx.ErrNoRows) {
				err = tx.QueryRow(ctx, `
    INSERT INTO venue_layouts (venue_id, name, is_active)
    VALUES ($1, $2, false) RETURNING id::text`, venueID, floor.Name).Scan(&layoutID)
			}
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `
   INSERT INTO venue_tables (venue_id, layout_id, identifier, seats, position_x, position_y, qr_token_hash)
   VALUES ($1, $2, $3, $4, $5, $6, 'unissued:' || gen_random_uuid()::text)
   ON CONFLICT (venue_id, identifier) DO UPDATE SET
     layout_id = EXCLUDED.layout_id, seats = EXCLUDED.seats,
     position_x = EXCLUDED.position_x, position_y = EXCLUDED.position_y`, venueID, layoutID, identifier, table.Seats, table.X, table.Y)
			if err != nil {
				return err
			}
		}
	}
	if hasPlan && !found {
		return appfault.ErrNotFound
	}
	return nil
}
