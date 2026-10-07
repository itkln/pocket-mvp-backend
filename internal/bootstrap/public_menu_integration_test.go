package bootstrap

import (
	"context"
	"errors"
	"testing"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/catalog"
	"pocket-mvp-backend/internal/modules/venues"
)

// Opt-in end-to-end database check of the Vendor write model and Spots public projection.
func TestPublicMenuIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	var ownerID, venueID string
	if err := db.QueryRow(ctx, `INSERT INTO users (email, password_hash, first_name, last_name, account_role)
		VALUES ('menu-owner@example.test', 'test-only', 'Menu', 'Owner', 'venue_owner') RETURNING id::text`).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO venues (owner_user_id, slug, name, address_line1, city,
		country_code, timezone, currency, status) VALUES ($1, 'menu-test-venue', 'Menu Test',
		'Main 1', 'Bratislava', 'SK', 'Europe/Bratislava', 'EUR', 'active') RETURNING id::text`, ownerID).Scan(&venueID); err != nil {
		t.Fatal(err)
	}
	owner := catalog.NewService(catalog.NewPostgresRepository(db), access.NewVenueAuthorizer(db))
	spots := venues.NewService(venues.NewPostgresRepository(db))
	second, err := owner.CreateCategory(ctx, ownerID, venueID, catalog.CategoryInput{Name: "Drinks", SortOrder: 2})
	if err != nil {
		t.Fatal(err)
	}
	first, err := owner.CreateCategory(ctx, ownerID, venueID, catalog.CategoryInput{Name: "Food", SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	available := true
	weight, energy, protein, fat, carbs := 250, 196.5, 8.25, 0.0, 28.0
	base := catalog.MenuItemInput{CategoryID: first.ID, Name: "Soup", Description: "Tomato", PriceMinor: 750,
		Currency: "eur", IsAvailable: &available, SortOrder: 2, ImageURL: "https://example.test/old.jpg",
		PortionWeightG: &weight, EnergyKcal: &energy, ProteinG: &protein, FatG: &fat, CarbsG: &carbs}
	item, err := owner.CreateMenuItem(ctx, ownerID, venueID, base)
	if err != nil {
		t.Fatal(err)
	}
	if item.PortionWeightG == nil || *item.PortionWeightG != weight || item.FatG == nil || *item.FatG != 0 {
		t.Fatalf("Vendor create lost nutrition/zero: %+v", item)
	}
	base.Name, base.SortOrder, base.PriceMinor, base.ImageURL = "Salad", 1, 990, ""
	earlier, err := owner.CreateMenuItem(ctx, ownerID, venueID, base)
	if err != nil {
		t.Fatal(err)
	}
	base.CategoryID, base.Name, base.SortOrder = second.ID, "Tea", 0
	if _, err := owner.CreateMenuItem(ctx, ownerID, venueID, base); err != nil {
		t.Fatal(err)
	}

	menu, err := spots.ListPublishedMenu(ctx, venueID)
	if err != nil {
		t.Fatal(err)
	}
	if len(menu) != 2 || menu[0].ID != first.ID || menu[1].ID != second.ID ||
		len(menu[0].Items) != 2 || menu[0].Items[0].ID != earlier.ID || menu[0].Items[1].ID != item.ID {
		t.Fatalf("category/item order differs from Vendor: %+v", menu)
	}
	if got := menu[0].Items[1]; got.PriceMinor != 750 || got.Currency != "EUR" || got.ImageURL != "https://example.test/old.jpg" {
		t.Fatalf("public item differs from Vendor: %+v", got)
	}
	if got := menu[0].Items[1]; got.PortionWeightG == nil || *got.PortionWeightG != weight ||
		got.EnergyKcal == nil || *got.EnergyKcal != energy || got.ProteinG == nil || *got.ProteinG != protein ||
		got.FatG == nil || *got.FatG != 0 || got.CarbsG == nil || *got.CarbsG != carbs {
		t.Fatalf("public nutrition differs from Vendor: %+v", got)
	}
	owned, err := owner.ListMenuItems(ctx, ownerID, venueID)
	if err != nil || len(owned) != 3 || owned[1].EnergyKcal == nil || *owned[1].EnergyKcal != energy {
		t.Fatalf("Vendor list lost nutrition: %+v, %v", owned, err)
	}

	energy = 200
	base = catalog.MenuItemInput{CategoryID: first.ID, Name: "Soup", Description: "Updated",
		PriceMinor: 825, Currency: "usd", IsAvailable: &available, SortOrder: 2,
		ImageURL: "https://example.test/new.jpg", PortionWeightG: &weight, EnergyKcal: &energy,
		ProteinG: &protein, FatG: &fat, CarbsG: &carbs}
	updated, err := owner.UpdateMenuItem(ctx, ownerID, venueID, item.ID, base)
	if err != nil {
		t.Fatal(err)
	}
	menu, err = spots.ListPublishedMenu(ctx, venueID)
	if err != nil {
		t.Fatal(err)
	}
	if got := menu[0].Items[1]; got.ID != item.ID || got.Description != "Updated" ||
		got.PriceMinor != 825 || got.Currency != "USD" || got.ImageURL != updated.ImageURL ||
		got.ImageURL != "https://example.test/new.jpg" || got.EnergyKcal == nil || *got.EnergyKcal != 200 {
		t.Fatalf("Vendor update not reflected in Spots: %+v", got)
	}

	base.ImageURL = ""
	base.PortionWeightG, base.EnergyKcal, base.ProteinG, base.FatG, base.CarbsG = nil, nil, nil, nil, nil
	if _, err := owner.UpdateMenuItem(ctx, ownerID, venueID, item.ID, base); err != nil {
		t.Fatal(err)
	}
	menu, err = spots.ListPublishedMenu(ctx, venueID)
	if err != nil || len(menu) != 2 || len(menu[0].Items) != 2 || menu[0].Items[1].ImageURL != "" {
		t.Fatalf("removed image still public: %+v, %v", menu, err)
	}
	if got := menu[0].Items[1]; got.PortionWeightG != nil || got.EnergyKcal != nil || got.ProteinG != nil || got.FatG != nil || got.CarbsG != nil {
		t.Fatalf("cleared nutrition still public: %+v", got)
	}
	available = false
	if _, err := owner.UpdateMenuItem(ctx, ownerID, venueID, item.ID, base); err != nil {
		t.Fatal(err)
	}
	menu, err = spots.ListPublishedMenu(ctx, venueID)
	if err != nil || len(menu) != 2 || len(menu[0].Items) != 1 || menu[0].Items[0].ID != earlier.ID {
		t.Fatalf("unavailable item remains public: %+v, %v", menu, err)
	}
	available = true
	if _, err := owner.UpdateMenuItem(ctx, ownerID, venueID, item.ID, base); err != nil {
		t.Fatal(err)
	}
	if err := owner.DeleteMenuItem(ctx, ownerID, venueID, item.ID); err != nil {
		t.Fatal(err)
	}
	menu, err = spots.ListPublishedMenu(ctx, venueID)
	if err != nil || len(menu) != 2 || len(menu[0].Items) != 1 {
		t.Fatalf("deleted item remains public: %+v, %v", menu, err)
	}
	active := false
	if _, err := owner.UpdateCategory(ctx, ownerID, venueID, first.ID, catalog.CategoryInput{Name: "Food", SortOrder: 1, IsActive: &active}); err != nil {
		t.Fatal(err)
	}
	menu, err = spots.ListPublishedMenu(ctx, venueID)
	if err != nil || len(menu) != 1 || menu[0].ID != second.ID {
		t.Fatalf("hidden category remains public: %+v, %v", menu, err)
	}
	if _, err := db.Exec(ctx, `UPDATE venues SET status = 'paused' WHERE id = $1`, venueID); err != nil {
		t.Fatal(err)
	}
	if _, err := spots.ListPublishedMenu(ctx, venueID); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("unpublished venue menu is accessible: %v", err)
	}
}
