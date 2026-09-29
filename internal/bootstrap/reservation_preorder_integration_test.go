package bootstrap

import (
	"context"
	"errors"
	"testing"
	"time"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/availability"
	"pocket-mvp-backend/internal/modules/ordering"
	"pocket-mvp-backend/internal/modules/reservations"
)

// Exercises the Spots booking and Vendor order read model against an isolated database.
func TestReservationPreorderIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	var ownerID, venueID, categoryID, itemID, otherVenueID, foreignItemID string
	if err := db.QueryRow(ctx, `INSERT INTO users (email, password_hash, first_name, last_name, account_role)
		VALUES ('preorder-owner@example.test', 'test-only', 'Preorder', 'Owner', 'venue_owner') RETURNING id::text`).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO venues (owner_user_id, slug, name, address_line1, city, country_code, timezone, currency, status, settings)
		VALUES ($1, 'preorder-test', 'Preorder Test', 'Main 1', 'Bratislava', 'SK', 'Europe/Bratislava', 'EUR', 'active',
		'{"schedule_enabled":true,"same_day_booking":true,"preorder_enabled":true,"public_category":"restaurants"}') RETURNING id::text`, ownerID).Scan(&venueID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO venues (owner_user_id, slug, name, address_line1, city, country_code, timezone, currency, status)
		VALUES ($1, 'other-preorder-test', 'Other Test', 'Main 2', 'Bratislava', 'SK', 'Europe/Bratislava', 'EUR', 'active') RETURNING id::text`, ownerID).Scan(&otherVenueID); err != nil {
		t.Fatal(err)
	}
	var layoutID string
	if err := db.QueryRow(ctx, `INSERT INTO venue_layouts (venue_id, name) VALUES ($1, 'Main') RETURNING id::text`, venueID).Scan(&layoutID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO venue_tables (venue_id, layout_id, identifier, seats, qr_token_hash)
		VALUES ($1, $2, 'A1', 4, 'preorder-test-a1')`, venueID, layoutID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO menu_categories (venue_id, name) VALUES ($1, 'Food') RETURNING id::text`, venueID).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO menu_items (venue_id, category_id, name, price_minor, currency)
		VALUES ($1, $2, 'Pasta', 1200, 'EUR') RETURNING id::text`, venueID, categoryID).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	var foreignCategoryID string
	if err := db.QueryRow(ctx, `INSERT INTO menu_categories (venue_id, name) VALUES ($1, 'Other') RETURNING id::text`, otherVenueID).Scan(&foreignCategoryID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO menu_items (venue_id, category_id, name, price_minor, currency)
		VALUES ($1, $2, 'Foreign dish', 700, 'EUR') RETURNING id::text`, otherVenueID, foreignCategoryID).Scan(&foreignItemID); err != nil {
		t.Fatal(err)
	}

	availabilityRepository := availability.NewPostgresRepository(db)
	availabilityService := availability.NewService(availabilityRepository)
	booking := reservations.NewService(reservations.NewPostgresRepository(db), reservationAvailability{availabilityService, availabilityRepository})
	zone, _ := time.LoadLocation("Europe/Bratislava")
	selected := time.Now().In(zone).AddDate(0, 0, 2)
	start := time.Date(selected.Year(), selected.Month(), selected.Day(), 12, 0, 0, 0, zone)
	input := reservations.CreateInput{StartsAt: start, Guests: 2, GuestName: "Test Guest", GuestPhone: "+421900123456",
		Items: []reservations.PreorderItem{{MenuItemID: itemID, Quantity: 2, ExpectedPriceMinor: 1200}}, IdempotencyKey: "preorder-integration-1"}
	for _, invalid := range []struct {
		key  string
		item reservations.PreorderItem
		want error
	}{
		{"foreign", reservations.PreorderItem{MenuItemID: foreignItemID, Quantity: 1, ExpectedPriceMinor: 700}, reservations.ErrItemUnavailable},
		{"price", reservations.PreorderItem{MenuItemID: itemID, Quantity: 1, ExpectedPriceMinor: 1000}, reservations.ErrPriceChanged},
	} {
		candidate := input
		candidate.IdempotencyKey = invalid.key + "-preorder-test"
		candidate.Items = []reservations.PreorderItem{invalid.item}
		if _, err := booking.Create(ctx, venueID, candidate); !errors.Is(err, invalid.want) {
			t.Fatalf("%s: expected %v, got %v", invalid.key, invalid.want, err)
		}
	}
	var count int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM reservations WHERE venue_id = $1`, venueID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid preorder created reservation: %d, %v", count, err)
	}
	created, err := booking.Create(ctx, venueID, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.PreorderTotalMinor != 2400 {
		t.Fatalf("unexpected reservation: %+v", created)
	}
	replayed, err := booking.Create(ctx, venueID, input)
	if err != nil || replayed.ID != created.ID {
		t.Fatalf("idempotent retry: %+v, %v", replayed, err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM reservation_preorder_items WHERE reservation_id = $1 AND item_name = 'Pasta' AND unit_price_minor = 1200 AND quantity = 2`, created.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("price snapshot: %d, %v", count, err)
	}
	ownerOrders, err := ordering.NewPostgresRepository(db).List(ctx, venueID)
	if err != nil || len(ownerOrders) != 1 || ownerOrders[0].ReservationCode != created.Code || len(ownerOrders[0].Items) != 1 || ownerOrders[0].TotalMinor != 2400 {
		t.Fatalf("Vendor preorder: %+v, %v", ownerOrders, err)
	}
	conflicting := input
	conflicting.IdempotencyKey = "preorder-integration-2"
	if _, err := booking.Create(ctx, venueID, conflicting); !errors.Is(err, appfault.ErrConflict) {
		t.Fatalf("overlapping reservation: %v", err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM reservations WHERE venue_id = $1`, venueID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("unexpected reservation count: %d, %v", count, err)
	}
	accountBooking := reservations.CreateInput{
		StartsAt: start.Add(3 * time.Hour), Guests: 2, CustomerUserID: ownerID,
		GuestName: "Preorder Owner", GuestEmail: "preorder-owner@example.test", IdempotencyKey: "account-booking-test-1",
	}
	accountResult, err := booking.Create(ctx, venueID, accountBooking)
	if err != nil {
		t.Fatal(err)
	}
	var linkedUserID, linkedEmail string
	if err := db.QueryRow(ctx, `SELECT customer_user_id::text, guest_email FROM reservations WHERE id = $1`, accountResult.ID).
		Scan(&linkedUserID, &linkedEmail); err != nil || linkedUserID != ownerID || linkedEmail != accountBooking.GuestEmail {
		t.Fatalf("account booking link: %s, %s, %v", linkedUserID, linkedEmail, err)
	}
}
