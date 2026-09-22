package bootstrap

import (
	"context"
	"errors"
	"testing"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/favorites"
	"pocket-mvp-backend/internal/modules/venues"
)

// Opt-in PostgreSQL test; resetTestDatabase creates an isolated schema and
// applies every migration, including the favorites table.
func TestSavedVenuesIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	userIDs := make([]string, 2)
	for i, email := range []string{"favorites-first@example.test", "favorites-second@example.test"} {
		if err := db.QueryRow(ctx, `INSERT INTO users (email, password_hash, first_name, last_name, account_role)
			VALUES ($1, 'test-only', 'Test', 'User', 'customer') RETURNING id::text`, email).Scan(&userIDs[i]); err != nil {
			t.Fatal(err)
		}
	}
	var venueID string
	if err := db.QueryRow(ctx, `INSERT INTO venues (owner_user_id, slug, name, address_line1, city,
		country_code, timezone, currency, status) VALUES ($1, 'favorite-test-venue', 'Before rename',
		'Main 1', 'Bratislava', 'SK', 'Europe/Bratislava', 'EUR', 'active') RETURNING id::text`, userIDs[0]).Scan(&venueID); err != nil {
		t.Fatal(err)
	}
	venueService := venues.NewService(venues.NewPostgresRepository(db))
	service := favorites.NewService(favorites.NewPostgresRepository(db), venueService)
	for i := 0; i < 2; i++ {
		if err := service.Add(ctx, userIDs[0], venueID); err != nil {
			t.Fatal(err)
		}
	}
	first, err := service.List(ctx, userIDs[0])
	if err != nil || len(first) != 1 || first[0] != venueID {
		t.Fatalf("first favorites: %v, %v", first, err)
	}
	second, err := service.List(ctx, userIDs[1])
	if err != nil || len(second) != 0 {
		t.Fatalf("cross-account leak: %v, %v", second, err)
	}
	if _, err := db.Exec(ctx, `UPDATE venues SET name = 'After rename' WHERE id = $1`, venueID); err != nil {
		t.Fatal(err)
	}
	page, err := venueService.SearchPublished(ctx, venues.PublicVenueQuery{IDs: first})
	if err != nil || len(page.Venues) != 1 || page.Venues[0].Name != "After rename" {
		t.Fatalf("renamed catalog item: %v, %v", page.Venues, err)
	}
	detail, err := venueService.GetPublished(ctx, venueID)
	if err != nil || detail.Name != "After rename" {
		t.Fatalf("renamed detail: %v, %v", detail, err)
	}
	if _, err := db.Exec(ctx, `UPDATE venues SET status = 'paused' WHERE id = $1`, venueID); err != nil {
		t.Fatal(err)
	}
	if err := service.Add(ctx, userIDs[1], venueID); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("unpublished saved: %v", err)
	}
	if err := service.Remove(ctx, userIDs[0], venueID); err != nil {
		t.Fatal(err)
	}
	first, err = service.List(ctx, userIDs[0])
	if err != nil || len(first) != 0 {
		t.Fatalf("removed favorites: %v, %v", first, err)
	}
}
