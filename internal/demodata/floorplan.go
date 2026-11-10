package demodata

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func seedFloorPlan(ctx context.Context, tx pgx.Tx, venueID string) error {
	for _, layout := range floorLayouts(venueID) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO venue_layouts (id, venue_id, name, width, height, is_active)
			VALUES ($1, $2, $3, 100, 100, true)`, layout.ID, venueID, layout.Name,
		); err != nil {
			return fmt.Errorf("seed layout %s: %w", layout.Name, err)
		}
	}
	for _, table := range floorTables(venueID) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO venue_tables (
				id, venue_id, layout_id, identifier, seats, shape, position_x,
				position_y, width, height, chairs, status, qr_token_hash, qr_version
			)
			VALUES ($1, $2, $3, $4, $5, 'rectangle', $6, $7, $8, $9, '[]', $10, $11, 1)`,
			table.ID, venueID, table.LayoutID, table.Identifier, table.Seats,
			table.X, table.Y, table.Width, table.Height, table.Status,
			"demo-"+venueID+"-table-"+table.Identifier,
		); err != nil {
			return fmt.Errorf("seed table %s: %w", table.Identifier, err)
		}
	}
	for _, fixture := range floorFixtures(venueID) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO venue_layout_fixtures (
				id, venue_id, layout_id, client_key, fixture_type, position_x,
				position_y, rotation, size
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			fixture.ID, venueID, fixture.LayoutID, fixture.Key, fixture.Type,
			fixture.X, fixture.Y, fixture.Rotation, fixture.Size,
		); err != nil {
			return fmt.Errorf("seed fixture %s: %w", fixture.Key, err)
		}
	}
	return nil
}

type layoutSeed struct {
	ID   string
	Name string
}

func floorLayouts(venueID string) []layoutSeed {
	if venueID == secondaryVenueID {
		return []layoutSeed{{ID: "60000000-0000-4000-8000-000000000101", Name: "Основной зал"}}
	}
	return []layoutSeed{
		{ID: "60000000-0000-4000-8000-000000000001", Name: "1 этаж"},
		{ID: "60000000-0000-4000-8000-000000000002", Name: "2 этаж"},
		{ID: "60000000-0000-4000-8000-000000000003", Name: "Терраса"},
	}
}

func floorTables(venueID string) []tableSeed {
	if venueID == secondaryVenueID {
		layout := "60000000-0000-4000-8000-000000000101"
		return []tableSeed{
			{ID: "61000000-0000-4000-8000-000000000101", LayoutID: layout, Identifier: "C1", Seats: 2, X: 15, Y: 20, Width: 10, Height: 10, Status: "available"},
			{ID: "61000000-0000-4000-8000-000000000102", LayoutID: layout, Identifier: "C2", Seats: 2, X: 48, Y: 20, Width: 10, Height: 10, Status: "occupied"},
			{ID: "61000000-0000-4000-8000-000000000103", LayoutID: layout, Identifier: "C3", Seats: 4, X: 65, Y: 58, Width: 12, Height: 10, Status: "available"},
		}
	}
	return []tableSeed{
		{ID: "61000000-0000-4000-8000-000000000001", LayoutID: "60000000-0000-4000-8000-000000000001", Identifier: "01", Seats: 2, X: 8, Y: 13, Width: 10, Height: 10, Status: "available"},
		{ID: "61000000-0000-4000-8000-000000000002", LayoutID: "60000000-0000-4000-8000-000000000001", Identifier: "02", Seats: 4, X: 31, Y: 12, Width: 10, Height: 10, Status: "occupied"},
		{ID: "61000000-0000-4000-8000-000000000003", LayoutID: "60000000-0000-4000-8000-000000000001", Identifier: "03", Seats: 4, X: 59, Y: 14, Width: 10, Height: 10, Status: "reserved"},
		{ID: "61000000-0000-4000-8000-000000000004", LayoutID: "60000000-0000-4000-8000-000000000001", Identifier: "04", Seats: 6, X: 16, Y: 55, Width: 14, Height: 10, Status: "occupied"},
		{ID: "61000000-0000-4000-8000-000000000005", LayoutID: "60000000-0000-4000-8000-000000000001", Identifier: "05", Seats: 2, X: 63, Y: 57, Width: 10, Height: 10, Status: "available"},
		{ID: "61000000-0000-4000-8000-000000000011", LayoutID: "60000000-0000-4000-8000-000000000002", Identifier: "11", Seats: 2, X: 12, Y: 16, Width: 10, Height: 10, Status: "available"},
		{ID: "61000000-0000-4000-8000-000000000012", LayoutID: "60000000-0000-4000-8000-000000000002", Identifier: "12", Seats: 4, X: 43, Y: 17, Width: 10, Height: 10, Status: "reserved"},
		{ID: "61000000-0000-4000-8000-000000000013", LayoutID: "60000000-0000-4000-8000-000000000002", Identifier: "13", Seats: 6, X: 68, Y: 52, Width: 14, Height: 10, Status: "available"},
		{ID: "61000000-0000-4000-8000-000000000021", LayoutID: "60000000-0000-4000-8000-000000000003", Identifier: "21", Seats: 2, X: 15, Y: 25, Width: 10, Height: 10, Status: "available"},
		{ID: "61000000-0000-4000-8000-000000000022", LayoutID: "60000000-0000-4000-8000-000000000003", Identifier: "22", Seats: 2, X: 45, Y: 25, Width: 10, Height: 10, Status: "available"},
		{ID: "61000000-0000-4000-8000-000000000023", LayoutID: "60000000-0000-4000-8000-000000000003", Identifier: "23", Seats: 4, X: 68, Y: 58, Width: 10, Height: 10, Status: "occupied"},
	}
}

func floorFixtures(venueID string) []fixtureSeed {
	if venueID == secondaryVenueID {
		return []fixtureSeed{{ID: "62000000-0000-4000-8000-000000000101", LayoutID: "60000000-0000-4000-8000-000000000101", Key: "coffee-bar", Type: "bar", X: 28, Y: 82, Rotation: 0, Size: 130}}
	}
	return []fixtureSeed{
		{ID: "62000000-0000-4000-8000-000000000001", LayoutID: "60000000-0000-4000-8000-000000000001", Key: "bar-main", Type: "bar", X: 33, Y: 86, Rotation: 0, Size: 120},
		{ID: "62000000-0000-4000-8000-000000000002", LayoutID: "60000000-0000-4000-8000-000000000001", Key: "window-main", Type: "window", X: 96, Y: 18, Rotation: 0, Size: 100},
		{ID: "62000000-0000-4000-8000-000000000003", LayoutID: "60000000-0000-4000-8000-000000000001", Key: "entrance-main", Type: "entrance", X: 5, Y: 84, Rotation: 0, Size: 100},
		{ID: "62000000-0000-4000-8000-000000000011", LayoutID: "60000000-0000-4000-8000-000000000002", Key: "window-second", Type: "window", X: 94, Y: 20, Rotation: 0, Size: 110},
	}
}
