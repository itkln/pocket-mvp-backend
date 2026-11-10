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
		if invalid.key == "foreign" {
			candidate.Items = append(input.Items, invalid.item)
		}
		if _, err := booking.Create(ctx, venueID, candidate); !errors.Is(err, invalid.want) {
			t.Fatalf("%s: expected %v, got %v", invalid.key, invalid.want, err)
		}
	}
	if _, err := db.Exec(ctx, `UPDATE menu_categories SET preorder_enabled = false WHERE id = $1`, categoryID); err != nil {
		t.Fatal(err)
	}
	if _, err := booking.Create(ctx, venueID, input); !errors.Is(err, reservations.ErrItemUnavailable) {
		t.Fatalf("disabled category accepted on direct booking: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE menu_categories SET preorder_enabled = true WHERE id = $1`, categoryID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM reservations WHERE venue_id = $1`, venueID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid preorder created reservation: %d, %v", count, err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM orders WHERE venue_id = $1`, venueID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed booking left an order: %d, %v", count, err)
	}
	created, err := booking.Create(ctx, venueID, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.PreorderTotalMinor != 2400 || created.PreorderOrderNumber == 0 {
		t.Fatalf("unexpected reservation: %+v", created)
	}
	replayed, err := booking.Create(ctx, venueID, input)
	if err != nil || replayed.ID != created.ID || replayed.PreorderOrderNumber != created.PreorderOrderNumber {
		t.Fatalf("idempotent retry: %+v, %v", replayed, err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM reservation_preorder_items WHERE reservation_id = $1 AND item_name = 'Pasta' AND unit_price_minor = 1200 AND quantity = 2`, created.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("price snapshot: %d, %v", count, err)
	}
	ownerOrders, err := ordering.NewPostgresRepository(db).List(ctx, venueID)
	if err != nil || len(ownerOrders) != 1 || ownerOrders[0].ReservationCode != created.Code || len(ownerOrders[0].Items) != 1 || ownerOrders[0].TotalMinor != 2400 ||
		ownerOrders[0].ReservationStartsAt == nil || !ownerOrders[0].ReservationStartsAt.Equal(input.StartsAt) || ownerOrders[0].GuestName != input.GuestName || ownerOrders[0].GuestPhone != input.GuestPhone || ownerOrders[0].GuestCount != input.Guests {
		t.Fatalf("Vendor preorder: %+v, %v", ownerOrders, err)
	}
	detail, err := ordering.NewPostgresRepository(db).Get(ctx, venueID, ownerOrders[0].ID)
	if err != nil || detail.ReservationID != created.ID || detail.ReservationCode != created.Code || detail.ReservationStartsAt == nil || !detail.ReservationStartsAt.Equal(input.StartsAt) || detail.Currency != "EUR" || len(detail.Items) != 1 || detail.Items[0].Quantity != 2 || detail.Items[0].TotalMinor != 2400 {
		t.Fatalf("queue preorder details: %+v, %v", detail, err)
	}
	updatedOrder, err := ordering.NewPostgresRepository(db).UpdateStatus(ctx, venueID, ownerOrders[0].ID, "accepted")
	if err != nil || updatedOrder.Status != "accepted" || updatedOrder.ReservationCode != created.Code ||
		updatedOrder.ReservationStartsAt == nil || !updatedOrder.ReservationStartsAt.Equal(input.StartsAt) ||
		updatedOrder.GuestPhone != input.GuestPhone || updatedOrder.GuestCount != input.Guests ||
		len(updatedOrder.Items) != 1 || updatedOrder.Items[0].Name != "Pasta" || updatedOrder.Items[0].UnitPriceMinor != 1200 {
		t.Fatalf("Vendor status update lost booking/preorder details: %+v, %v", updatedOrder, err)
	}
	otherOrders, err := ordering.NewPostgresRepository(db).List(ctx, otherVenueID)
	if err != nil || len(otherOrders) != 0 {
		t.Fatalf("preorder leaked into another venue: %+v, %v", otherOrders, err)
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
		Items: []reservations.PreorderItem{{MenuItemID: itemID, Quantity: 1, ExpectedPriceMinor: 1200}},
	}
	hold, err := booking.Hold(ctx, venueID, accountBooking)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE menu_categories SET preorder_enabled = false WHERE id = $1`, categoryID); err != nil {
		t.Fatal(err)
	}
	if _, err := booking.Confirm(ctx, venueID, hold.ID, accountBooking); !errors.Is(err, reservations.ErrItemUnavailable) {
		t.Fatalf("disabled category accepted after hold: %v", err)
	}
	var holdStatus string
	if err := db.QueryRow(ctx, `SELECT status FROM reservations WHERE id = $1`, hold.ID).Scan(&holdStatus); err != nil || holdStatus != "pending" {
		t.Fatalf("failed confirmation changed hold: %s, %v", holdStatus, err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM reservation_preorder_items WHERE reservation_id = $1`, hold.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed confirmation left preorder items: %d, %v", count, err)
	}
	if _, err := db.Exec(ctx, `UPDATE menu_categories SET preorder_enabled = true WHERE id = $1`, categoryID); err != nil {
		t.Fatal(err)
	}
	stalePrice := accountBooking
	stalePrice.Items = []reservations.PreorderItem{{MenuItemID: itemID, Quantity: 1, ExpectedPriceMinor: 1000}}
	if _, err := booking.Confirm(ctx, venueID, hold.ID, stalePrice); !errors.Is(err, reservations.ErrPriceChanged) {
		t.Fatalf("stale price accepted after hold: %v", err)
	}
	accountResult, err := booking.Confirm(ctx, venueID, hold.ID, accountBooking)
	if err != nil {
		t.Fatal(err)
	}
	if accountResult.PreorderOrderNumber == 0 || accountResult.PreorderTotalMinor != 1200 {
		t.Fatalf("account preorder confirmation: %+v", accountResult)
	}
	reloaded, err := booking.GetConfirmed(ctx, venueID, accountResult.ID, accountBooking.IdempotencyKey)
	if err != nil || reloaded.PreorderOrderNumber != accountResult.PreorderOrderNumber {
		t.Fatalf("reloaded preorder number: %+v, %v", reloaded, err)
	}
	replayedAccount, err := booking.Confirm(ctx, venueID, hold.ID, accountBooking)
	if err != nil || replayedAccount.PreorderOrderNumber != accountResult.PreorderOrderNumber {
		t.Fatalf("idempotent preorder confirmation: %+v, %v", replayedAccount, err)
	}
	// The account's preorder is stored with its reservation and menu snapshot.
	var preorderID, preorderItem string
	var preorderNumber int64
	if err := db.QueryRow(ctx, `SELECT o.id::text, o.order_number, oi.item_name FROM orders o JOIN order_items oi ON oi.order_id = o.id
		WHERE o.customer_user_id = $1 AND o.channel = 'preorder' AND o.reservation_id = $2`, ownerID, accountResult.ID).
		Scan(&preorderID, &preorderNumber, &preorderItem); err != nil || preorderNumber != accountResult.PreorderOrderNumber || preorderItem != "Pasta" {
		t.Fatalf("account preorder history: %d, %s, %v", preorderNumber, preorderItem, err)
	}
	var linkedUserID, linkedEmail string
	if err := db.QueryRow(ctx, `SELECT customer_user_id::text, guest_email FROM reservations WHERE id = $1`, accountResult.ID).
		Scan(&linkedUserID, &linkedEmail); err != nil || linkedUserID != ownerID || linkedEmail != accountBooking.GuestEmail {
		t.Fatalf("account booking link: %s, %s, %v", linkedUserID, linkedEmail, err)
	}
	// The account list includes ordinary bookings and saved preorder snapshots,
	// while guest bookings, unfinished holds and other accounts remain invisible.
	plainInput := reservations.CreateInput{StartsAt: start.Add(6 * time.Hour), Guests: 2, CustomerUserID: ownerID,
		GuestName: "Another Guest", GuestPhone: "+421900111222", GuestEmail: "another@example.test", Notes: "Window seat", IdempotencyKey: "plain-account-booking"}
	plain, err := booking.Create(ctx, venueID, plainInput)
	if err != nil {
		t.Fatal(err)
	}
	pendingInput := plainInput
	pendingInput.StartsAt = start.AddDate(0, 0, 1)
	pendingInput.IdempotencyKey = "unfinished-account-hold"
	pending, err := booking.Hold(ctx, venueID, pendingInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE menu_items SET name = 'Changed pasta', price_minor = 9999 WHERE id = $1`, itemID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE menu_categories SET preorder_enabled = false WHERE id = $1`, categoryID); err != nil {
		t.Fatal(err)
	}
	replayedSaved, err := booking.Create(ctx, venueID, input)
	if err != nil || replayedSaved.ID != created.ID || replayedSaved.PreorderTotalMinor != 2400 {
		t.Fatalf("saved booking changed with menu settings: %+v, %v", replayedSaved, err)
	}
	checkList := func() {
		t.Helper()
		list, err := booking.ListCustomer(ctx, ownerID)
		if err != nil || len(list) != 2 {
			t.Fatalf("own bookings: %+v, %v", list, err)
		}
		if list[0].ID != plain.ID || list[0].GuestName != plainInput.GuestName || list[0].GuestPhone != plainInput.GuestPhone ||
			list[0].GuestEmail != plainInput.GuestEmail || list[0].Notes != plainInput.Notes || len(list[0].PreorderItems) != 0 || list[0].PreorderTotalMinor != 0 {
			t.Fatalf("plain booking details: %+v", list[0])
		}
		entry := list[1]
		if entry.ID != accountResult.ID || entry.Code != accountResult.Code || entry.Timezone != "Europe/Bratislava" ||
			!entry.StartsAt.Equal(accountBooking.StartsAt) || entry.VenueName != "Preorder Test" || entry.GuestCount != 2 || entry.Status != "confirmed" ||
			entry.PreorderOrderNumber != accountResult.PreorderOrderNumber || entry.PreorderTotalMinor != 1200 || entry.Currency != "EUR" ||
			len(entry.PreorderItems) != 1 || entry.PreorderItems[0].Name != "Pasta" || entry.PreorderItems[0].UnitPriceMinor != 1200 || entry.PreorderItems[0].Quantity != 1 {
			t.Fatalf("preorder details: %+v", entry)
		}
	}
	checkList()
	if _, err := db.Exec(ctx, `UPDATE reservations SET status = 'cancelled' WHERE id = $1`, pending.ID); err != nil {
		t.Fatal(err)
	}
	checkList()
	otherList, err := booking.ListCustomer(ctx, "10000000-0000-4000-8000-000000000099")
	if err != nil || len(otherList) != 0 {
		t.Fatalf("other account: %+v, %v", otherList, err)
	}

	orders := ordering.NewPostgresRepository(db)
	prepared, err := orders.UpdateStatus(ctx, venueID, updatedOrder.ID, "preparing")
	if err != nil || prepared.Status != "preparing" || prepared.ReservationID != created.ID ||
		prepared.ReservationStatus != "confirmed" || prepared.ReservationStartsAt == nil || !prepared.ReservationStartsAt.Equal(input.StartsAt) {
		t.Fatalf("manual preparation changed the visit: %+v, %v", prepared, err)
	}
	// A stale client cannot advance an accepted order after its booking was cancelled.
	staleOrderID := preorderID
	if _, err := orders.UpdateStatus(ctx, venueID, staleOrderID, "accepted"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE reservations SET status='cancelled' WHERE id=$1`, accountResult.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := orders.UpdateStatus(ctx, venueID, staleOrderID, "preparing"); !errors.Is(err, appfault.ErrConflict) {
		t.Fatalf("preparation after reservation cancellation: %v", err)
	}
}
