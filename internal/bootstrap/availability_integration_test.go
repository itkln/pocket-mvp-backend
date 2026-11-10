package bootstrap

import (
	"context"
	"testing"
	"time"

	"pocket-mvp-backend/internal/modules/availability"
)

// Opt-in database check of published tables and overlapping reservations.
func TestPublicAvailabilityIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	var layoutID, tableID string
	ownerID := seedUser(t, db, "availability-owner@example.test", "venue_owner")
	venueID := seedVenue(t, db, ownerID, venueSeed{Slug: "availability-test", Name: "Availability Test", Settings: `{"schedule_enabled":true,"same_day_booking":true,"public_category":"restaurants"}`})
	if err := db.QueryRow(ctx, `INSERT INTO venue_layouts (venue_id, name) VALUES ($1, 'Main') RETURNING id::text`, venueID).Scan(&layoutID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO venue_tables (venue_id, layout_id, identifier, seats, qr_token_hash)
		VALUES ($1, $2, 'A1', 4, 'availability-test-a1') RETURNING id::text`, venueID, layoutID).Scan(&tableID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO venue_tables (venue_id, layout_id, identifier, seats, qr_token_hash)
		VALUES ($1, $2, 'A2', 2, 'availability-test-a2')`, venueID, layoutID); err != nil {
		t.Fatal(err)
	}
	zone, err := time.LoadLocation("Europe/Bratislava")
	if err != nil {
		t.Fatal(err)
	}
	selected := time.Now().In(zone).AddDate(0, 0, 1)
	start := time.Date(selected.Year(), selected.Month(), selected.Day(), 10, 0, 0, 0, zone)
	if _, err := db.Exec(ctx, `INSERT INTO reservations (venue_id, venue_table_id, reservation_code, guest_name,
		guest_count, starts_at, ends_at, status) VALUES ($1, $2, 'AVAIL-TEST', 'Guest', 4, $3, $4, 'confirmed')`,
		venueID, tableID, start, start.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	result, err := availability.NewService(availability.NewPostgresRepository(db)).Get(ctx, venueID, selected.Format("2006-01-02"), 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, slot := range result.Slots {
		if slot.Time == "10:00" {
			t.Fatal("overlapping reservation offered as available")
		}
		if slot.Time == "12:00" && (len(slot.Tables) != 1 || slot.Tables[0].ID != tableID) {
			t.Fatalf("booking end should release same operational table: %+v", slot)
		}
	}
}
