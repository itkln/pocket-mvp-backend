package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/availability"
	"pocket-mvp-backend/internal/modules/dinein"
	"pocket-mvp-backend/internal/modules/venues"
)

// V-05: what the owner publishes in Vendor is what Dish shows. Only active venues are listed,
// the name, contacts and opening hours match the profile, and a pause stops new bookings and
// orders while seated guests keep their bill.
func TestVenuePublicationIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	owner := seedUser(t, db, "owner@publication.test", "customer")
	venueID := seedVenue(t, db, owner, venueSeed{Slug: "publication", Status: "draft", Settings: `{"floor_plan":[{"id":"f1","name":"Main","tables":[{"id":"07","seats":4,"x":10,"y":10}]}]}`})
	authorizer := access.NewVenueAuthorizer(db)
	profiles := venues.NewService(venues.NewPostgresRepository(db), authorizer)
	spots := availability.NewService(availability.NewPostgresRepository(db))
	tables := dinein.NewService(dinein.NewPostgresRepository(db), authorizer, testProtector(t), "https://dish.example")

	hours := map[string]any{"sun": nil}
	for _, day := range []string{"mon", "tue", "wed", "thu", "fri"} {
		hours[day] = map[string]any{"open": "09:00", "close": "22:00"}
	}
	hours["sat"] = map[string]any{"open": "12:00", "close": "16:00"}
	input := venues.Input{
		Name: "Publication Bistro", City: "Bratislava", Address: "Main 1", CountryCode: "SK",
		Timezone: "Europe/Bratislava", Currency: "EUR", Status: "draft",
		Phone: "+421 900 123 456", Email: "hello@bistro.test",
		Settings: map[string]any{"opening_hours": hours, "preorder": false},
	}
	update := func(status string) {
		t.Helper()
		input.Status = status
		_, err := profiles.Update(ctx, owner, venueID, input)
		must(t, err)
	}

	// A draft is not in Dish.
	update("draft")
	if _, err := profiles.GetPublished(ctx, venueID); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("draft venue is public: %v", err)
	}

	// Published: Dish shows the profile as saved in Vendor.
	update("active")
	public, err := profiles.GetPublished(ctx, venueID)
	must(t, err)
	if public.Name != "Publication Bistro" || public.Phone != "+421 900 123 456" || public.Email != "hello@bistro.test" ||
		public.Timezone != "Europe/Bratislava" || public.PreorderEnabled || !public.BookingEnabled ||
		public.OpeningHours["sun"] != nil || public.OpeningHours["sat"] == nil || public.OpeningHours["sat"].Open != "12:00" {
		t.Fatalf("public venue does not match Vendor: %+v", public)
	}

	// Issuing the table's QR creates its row, so the table can be booked.
	qr, err := tables.OwnerQRCode(ctx, owner, venueID, "07", false)
	must(t, err)
	token := qr.URL[strings.LastIndex(qr.URL, "/")+1:]

	// Bookings follow the hours: none on Sunday, Saturday slots inside 12:00–16:00.
	zone, err := time.LoadLocation("Europe/Bratislava")
	must(t, err)
	day := time.Now().In(zone).AddDate(0, 0, 2)
	for day.Weekday() != time.Saturday {
		day = day.AddDate(0, 0, 1)
	}
	saturday, err := spots.Get(ctx, venueID, day.Format("2006-01-02"), 2)
	must(t, err)
	if len(saturday.Slots) == 0 || saturday.Slots[0].Time != "12:00" || saturday.Slots[len(saturday.Slots)-1].Time != "14:00" {
		t.Fatalf("saturday slots: %+v", saturday.Slots)
	}
	sunday, err := spots.Get(ctx, venueID, day.AddDate(0, 0, 1).Format("2006-01-02"), 2)
	must(t, err)
	if len(sunday.Slots) != 0 {
		t.Fatalf("closed sunday offers slots: %+v", sunday.Slots)
	}

	// Switching online booking off in Vendor closes it in Dish and in availability.
	input.Settings = map[string]any{"schedule_enabled": false}
	update("active")
	if public, err = profiles.GetPublished(ctx, venueID); err != nil || public.BookingEnabled {
		t.Fatalf("booking switched off: %+v %v", public, err)
	}
	if closed, err := spots.Get(ctx, venueID, day.Format("2006-01-02"), 2); err != nil || len(closed.Slots) != 0 {
		t.Fatalf("slots with booking off: %+v %v", closed.Slots, err)
	}
	input.Settings = map[string]any{"schedule_enabled": true}
	update("active")

	// Invalid hours are refused and nothing changes.
	input.Settings = map[string]any{"opening_hours": map[string]any{"mon": map[string]any{"open": "22:00", "close": "09:00"}}}
	if _, err = profiles.Update(ctx, owner, venueID, input); !errors.Is(err, appfault.ErrInvalidInput) {
		t.Fatalf("invalid hours: %v", err)
	}
	input.Settings = map[string]any{}

	// Paused: gone from Dish and bookings; the table QR still opens, without ordering.
	update("paused")
	if _, err = profiles.GetPublished(ctx, venueID); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("paused venue is public: %v", err)
	}
	if _, err = spots.Get(ctx, venueID, day.Format("2006-01-02"), 2); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("paused venue offers bookings: %v", err)
	}
	table, err := tables.Resolve(ctx, token)
	must(t, err)
	if table.OrderingEnabled {
		t.Fatal("paused venue takes table orders")
	}
	if _, err = tables.Create(ctx, dinein.CreateInput{Token: token, IdempotencyKey: "paused-venue-order-0001",
		Items: []dinein.ItemInput{{MenuItemID: "00000000-0000-4000-8000-000000000001", Quantity: 1, ExpectedPriceMinor: 100}}}); !errors.Is(err, dinein.ErrBillLocked) {
		t.Fatalf("order at a paused venue: %v", err)
	}

	// Back to draft: the QR stops working.
	update("draft")
	if _, err = tables.Resolve(ctx, token); !errors.Is(err, dinein.ErrQRInvalid) {
		t.Fatalf("draft venue QR: %v", err)
	}
	update("active")
	if table, err = tables.Resolve(ctx, token); err != nil || !table.OrderingEnabled {
		t.Fatalf("republished venue QR: %+v %v", table, err)
	}
}
