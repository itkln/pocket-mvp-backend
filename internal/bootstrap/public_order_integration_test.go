package bootstrap

import (
	"context"
	"errors"
	"testing"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/ordering"
)

func TestPublicOrderQuoteSubmitAndHistory(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	var ownerID, customerID, venueID, otherVenueID, categoryID, itemID, foreignCategoryID, foreignItemID string
	for _, row := range []struct {
		email, role string
		id          *string
	}{
		{"order-owner@example.test", "venue_owner", &ownerID}, {"order-customer@example.test", "customer", &customerID},
	} {
		if err := db.QueryRow(ctx, `INSERT INTO users (email, password_hash, first_name, last_name, account_role) VALUES ($1, 'test-only', 'Test', 'Guest', $2) RETURNING id::text`, row.email, row.role).Scan(row.id); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct {
		slug string
		id   *string
	}{{"order-venue", &venueID}, {"foreign-order-venue", &otherVenueID}} {
		if err := db.QueryRow(ctx, `INSERT INTO venues (owner_user_id, slug, name, address_line1, city, country_code, currency, status)
			VALUES ($1, $2, $2, 'Main 1', 'Bratislava', 'SK', 'EUR', 'active') RETURNING id::text`, ownerID, row.slug).Scan(row.id); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct {
		venue          string
		category, item *string
	}{{venueID, &categoryID, &itemID}, {otherVenueID, &foreignCategoryID, &foreignItemID}} {
		if err := db.QueryRow(ctx, `INSERT INTO menu_categories (venue_id, name) VALUES ($1, 'Food') RETURNING id::text`, row.venue).Scan(row.category); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(ctx, `INSERT INTO menu_items (venue_id, category_id, name, price_minor, currency) VALUES ($1, $2, 'Soup', 750, 'EUR') RETURNING id::text`, row.venue, *row.category).Scan(row.item); err != nil {
			t.Fatal(err)
		}
	}
	service := ordering.NewService(ordering.NewPostgresRepository(db), access.NewVenueAuthorizer(db))
	input := ordering.CreateInput{CustomerUserID: customerID, GuestName: "Other Guest", GuestEmail: "other@example.test",
		Items: []ordering.CreateItem{{MenuItemID: itemID, Quantity: 2, ExpectedPriceMinor: 750}}, IdempotencyKey: "public-order-integration-1"}
	quote, err := service.Quote(ctx, venueID, input)
	if err != nil || quote.TotalMinor != 1500 || quote.Currency != "EUR" || len(quote.Items) != 1 {
		t.Fatalf("quote: %+v, %v", quote, err)
	}
	foreign := input
	foreign.Items = []ordering.CreateItem{{MenuItemID: foreignItemID, Quantity: 1, ExpectedPriceMinor: 750}}
	if _, err := service.Submit(ctx, venueID, foreign); !errors.Is(err, ordering.ErrItemUnavailable) {
		t.Fatalf("foreign item: %v", err)
	}
	changed := input
	changed.Items = []ordering.CreateItem{{MenuItemID: itemID, Quantity: 2, ExpectedPriceMinor: 1}}
	if _, err := service.Submit(ctx, venueID, changed); !errors.Is(err, ordering.ErrPriceChanged) {
		t.Fatalf("tampered price: %v", err)
	}
	order, err := service.Submit(ctx, venueID, input)
	if err != nil || order.Number < 1 || order.TotalMinor != 1500 || len(order.Items) != 1 || order.Items[0].TotalMinor != 1500 {
		t.Fatalf("order: %+v, %v", order, err)
	}
	if _, err := db.Exec(ctx, `UPDATE menu_items SET price_minor = 900 WHERE id = $1`, itemID); err != nil {
		t.Fatal(err)
	}
	repeated, err := service.Submit(ctx, venueID, input)
	if err != nil || repeated.ID != order.ID || repeated.TotalMinor != 1500 {
		t.Fatalf("repeat: %+v, %v", repeated, err)
	}
	var auditCount, eventCount int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE entity_id = $1 AND actor_user_id = $2`, order.ID, customerID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND topic = 'order.created'`, order.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 || eventCount != 1 {
		t.Fatalf("duplicate audit/event: %d, %d", auditCount, eventCount)
	}
	if _, err := service.Submit(ctx, venueID, changed); !errors.Is(err, appfault.ErrConflict) {
		t.Fatalf("reused key: %v", err)
	}
	vendor, err := service.List(ctx, ownerID, venueID)
	if err != nil || len(vendor) != 1 || vendor[0].ID != order.ID {
		t.Fatalf("vendor: %+v, %v", vendor, err)
	}
	history, err := service.ListCustomer(ctx, customerID)
	if err != nil || len(history) != 1 || history[0].ID != order.ID {
		t.Fatalf("history: %+v, %v", history, err)
	}
	if outsider, err := service.ListCustomer(ctx, ownerID); err != nil || len(outsider) != 0 {
		t.Fatalf("foreign history: %+v, %v", outsider, err)
	}
	if _, err := service.GetPublic(ctx, venueID, order.ID, "some-other-secret-key"); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("public read leaked: %v", err)
	}
	if got, err := service.GetPublic(ctx, venueID, order.ID, input.IdempotencyKey); err != nil || got.ID != order.ID {
		t.Fatalf("public read: %+v, %v", got, err)
	}
	if _, err := service.Quote(ctx, venueID, input); !errors.Is(err, ordering.ErrPriceChanged) {
		t.Fatalf("stale quote: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE menu_items SET is_available = false WHERE id = $1`, itemID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Quote(ctx, venueID, input); !errors.Is(err, ordering.ErrItemUnavailable) {
		t.Fatalf("unavailable item: %v", err)
	}
}
