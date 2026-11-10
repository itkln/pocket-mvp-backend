package bootstrap

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"pocket-mvp-backend/internal/money"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/gratuity"
	"pocket-mvp-backend/internal/modules/dinein"
	"pocket-mvp-backend/internal/modules/ordering"
	"pocket-mvp-backend/internal/modules/reservations"
)

type tipTestAvailability struct{ tableID string }

func (a tipTestAvailability) FindSlot(_ context.Context, _ string, start time.Time, _ int) (reservations.Slot, error) {
	return reservations.Slot{TableID: a.tableID, StartsAt: start, EndsAt: start.Add(time.Hour)}, nil
}

func TestGratuityIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	ownerID := seedUser(t, db, "tips@example.test", "venue_owner")
	venueID := seedVenue(t, db, ownerID, venueSeed{Slug: "tips-test", Name: "Tips Test", Settings: `{"preorder_enabled":true,"floor_plan":[{"id":"main","name":"Main","tables":[{"id":"07","seats":4,"x":12,"y":30}]}]}`})
	_, itemID := seedMenuItem(t, db, venueID, "Food", true, "Toast", 1090)
	protector := testProtector(t)
	service := dinein.NewService(dinein.NewPostgresRepository(db), access.NewVenueAuthorizer(db), protector, "http://localhost:3001")
	qr, err := service.OwnerQRCode(ctx, ownerID, venueID, "07", false)
	must(t, err)
	token := qr.URL[strings.LastIndex(qr.URL, "/")+1:]
	expected := money.Minor(1145)
	input := dinein.CreateInput{Token: token, IdempotencyKey: "e14-percent-order-test", Tip: &gratuity.Selection{Mode: "percent", Percent: 5}, ExpectedTotalMinor: &expected,
		Items: []dinein.ItemInput{{MenuItemID: itemID, Quantity: 1, ExpectedPriceMinor: 1090}}}
	created, err := service.Create(ctx, input)
	must(t, err)
	if created.SubtotalMinor != 1090 || created.TipMinor != 55 || created.TotalMinor != 1145 {
		t.Fatalf("bad receipt: %+v", created)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := service.Create(ctx, input)
			if err == nil && got.ID != created.ID {
				err = errors.New("duplicate order")
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		must(t, err)
	}
	var tipCount, tipValue, subtotal, total int
	var tipStatus, billID string
	must(t, db.QueryRow(ctx, `SELECT count(*),COALESCE(sum(amount_minor),0) FROM tips WHERE order_id=$1`, created.ID).Scan(&tipCount, &tipValue))
	must(t, db.QueryRow(ctx, `SELECT status FROM tips WHERE order_id=$1`, created.ID).Scan(&tipStatus))
	must(t, db.QueryRow(ctx, `SELECT bill_id::text FROM orders WHERE id=$1`, created.ID).Scan(&billID))
	checkBill := func(wantBase, wantTip, wantTotal int) {
		t.Helper()
		must(t, db.QueryRow(ctx, `SELECT subtotal_minor,tip_minor,total_minor FROM bills WHERE id=$1`, billID).Scan(&subtotal, &tipValue, &total))
		if subtotal != wantBase || tipValue != wantTip || total != wantTotal {
			t.Fatalf("bill=%d+%d=%d want %d+%d=%d", subtotal, tipValue, total, wantBase, wantTip, wantTotal)
		}
	}
	if tipCount != 1 || tipValue != 55 || tipStatus != "pending" {
		t.Fatalf("duplicate or prematurely paid tips: %d %d %s", tipCount, tipValue, tipStatus)
	}
	checkBill(1090, 55, 1145)
	lookup, err := service.Lookup(ctx, token, input.IdempotencyKey)
	must(t, err)
	if lookup.TipMinor != 55 || lookup.TotalMinor != 1145 {
		t.Fatal("lookup lost tips")
	}
	changed := input
	changed.Tip = &gratuity.Selection{Mode: "custom", AmountMinor: 100}
	if _, err := service.Create(ctx, changed); !errors.Is(err, appfault.ErrConflict) {
		t.Fatalf("changed request key accepted: %v", err)
	}
	tampered := input
	tampered.IdempotencyKey = "e14-tampered-total-test"
	wrong := money.Minor(1090)
	tampered.ExpectedTotalMinor = &wrong
	if _, err := service.Create(ctx, tampered); !errors.Is(err, dinein.ErrPriceChanged) {
		t.Fatalf("tampered total accepted: %v", err)
	}
	custom := input
	custom.IdempotencyKey = "e14-custom-order-test"
	custom.Tip = &gratuity.Selection{Mode: "custom", AmountMinor: 123}
	custom.ExpectedTotalMinor = nil
	second, err := service.Create(ctx, custom)
	must(t, err)
	if second.TipMinor != 123 || second.TotalMinor != 1213 {
		t.Fatalf("custom: %+v", second)
	}
	zero := input
	zero.IdempotencyKey = "e14-zero-order-test"
	zero.Tip = &gratuity.Selection{Mode: "none"}
	zero.ExpectedTotalMinor = nil
	third, err := service.Create(ctx, zero)
	must(t, err)
	must(t, db.QueryRow(ctx, `SELECT count(*) FROM tips WHERE order_id=$1`, third.ID).Scan(&tipCount))
	if tipCount != 0 {
		t.Fatal("zero tip created a tip record")
	}
	checkBill(3270, 178, 3448)
	// The authoritative menu price is rechecked before accepting the confirmed grand total.
	_, err = db.Exec(ctx, `UPDATE menu_items SET price_minor=1200 WHERE id=$1`, itemID)
	must(t, err)
	stale := input
	stale.IdempotencyKey = "e14-stale-price-order-test"
	if _, err := service.Create(ctx, stale); !errors.Is(err, dinein.ErrPriceChanged) {
		t.Fatalf("stale menu accepted: %v", err)
	}
	// Retrying a committed order after a menu change still returns the original snapshot.
	got, err := service.Create(ctx, input)
	must(t, err)
	if got.TotalMinor != 1145 {
		t.Fatal("retry recalculated committed order")
	}
	vendor := ordering.NewPostgresRepository(db)
	orders, err := vendor.List(ctx, venueID)
	must(t, err)
	found := false
	for _, o := range orders {
		if o.ID == created.ID {
			found = o.TipMinor == 55 && o.TotalMinor == 1145
		}
	}
	if !found {
		t.Fatal("Vendor lost tip breakdown")
	}
	_, err = vendor.UpdateStatus(ctx, venueID, created.ID, "cancelled")
	must(t, err)
	_, err = vendor.UpdateStatus(ctx, venueID, created.ID, "cancelled")
	must(t, err)
	checkBill(2180, 123, 2303)
	must(t, db.QueryRow(ctx, `SELECT status FROM tips WHERE order_id=$1`, created.ID).Scan(&tipStatus))
	if tipStatus != "cancelled" {
		t.Fatal("cancelled order retained pending tips")
	}
	// A bill with an in-progress payment cannot be changed beneath its payment amount.
	_, err = db.Exec(ctx, `UPDATE bills SET status='payment_pending' WHERE id=$1`, billID)
	must(t, err)
	if _, err := vendor.UpdateStatus(ctx, venueID, second.ID, "cancelled"); !errors.Is(err, appfault.ErrConflict) {
		t.Fatalf("cancelled locked bill: %v", err)
	}
	_, err = db.Exec(ctx, `UPDATE bills SET status='open' WHERE id=$1`, billID)
	must(t, err)

	booking := reservations.NewService(reservations.NewPostgresRepository(db), tipTestAvailability{tableID: qr.ID})
	start := time.Now().Add(48 * time.Hour).Truncate(time.Hour)
	reservation := reservations.CreateInput{StartsAt: start, Guests: 2, GuestName: "Tips Guest", GuestPhone: "+421900000000", CustomerUserID: ownerID,
		IdempotencyKey: "e14-preorder-hold-test", Items: []reservations.PreorderItem{{MenuItemID: itemID, Quantity: 1, ExpectedPriceMinor: 1200}}}
	grand := money.Minor(1200)
	reservation.ExpectedTotalMinor = &grand
	hold, err := booking.Hold(ctx, venueID, reservation)
	must(t, err)
	confirmed, err := booking.Confirm(ctx, venueID, hold.ID, reservation)
	must(t, err)
	if confirmed.PreorderSubtotalMinor != 1200 || confirmed.PreorderTipMinor != 0 || confirmed.PreorderTotalMinor != 1200 {
		t.Fatalf("preorder receipt: %+v", confirmed)
	}
	replay, err := booking.Confirm(ctx, venueID, hold.ID, reservation)
	must(t, err)
	if replay != confirmed {
		t.Fatalf("preorder retry changed: %+v", replay)
	}
	must(t, db.QueryRow(ctx, `SELECT count(*),COALESCE(sum(t.amount_minor),0) FROM tips t JOIN orders o ON o.id=t.order_id WHERE o.reservation_id=$1`, confirmed.ID).Scan(&tipCount, &tipValue))
	if tipCount != 0 || tipValue != 0 {
		t.Fatal("reservation created tips")
	}
	customer, err := reservations.NewPostgresRepository(db).ListCustomer(ctx, ownerID)
	must(t, err)
	if len(customer) != 1 || customer[0].PreorderTipMinor != 0 || customer[0].PreorderTotalMinor != 1200 {
		t.Fatalf("customer receipt: %+v", customer)
	}
	var preorderCount int
	var preorderTip int64
	must(t, db.QueryRow(ctx, `SELECT count(*),COALESCE(sum(tip_minor),0) FROM orders WHERE reservation_id=$1 AND channel='preorder'`, confirmed.ID).Scan(&preorderCount, &preorderTip))
	if preorderCount != 1 || preorderTip != 0 {
		t.Fatal("reservation unexpectedly includes tips")
	}
	_, err = reservations.NewPostgresRepository(db).CancelCustomer(ctx, ownerID, confirmed.ID)
	must(t, err)
	must(t, db.QueryRow(ctx, `SELECT count(*) FROM tips t JOIN orders o ON o.id=t.order_id WHERE o.reservation_id=$1`, confirmed.ID).Scan(&tipCount))
	if tipCount != 0 {
		t.Fatal("cancelled reservation has tips")
	}
	// Direct reservation creation also keeps tips at zero and remains idempotent.
	reservation.StartsAt = start.Add(3 * time.Hour)
	reservation.IdempotencyKey = "e14-preorder-direct-test"
	reservation.Tip = &gratuity.Selection{Mode: "none"}
	grand = 1200
	direct, err := booking.Create(ctx, venueID, reservation)
	must(t, err)
	if direct.PreorderTipMinor != 0 || direct.PreorderTotalMinor != 1200 {
		t.Fatalf("direct preorder: %+v", direct)
	}
	_, err = booking.Create(ctx, venueID, reservation)
	must(t, err)
	must(t, db.QueryRow(ctx, `SELECT count(*) FROM tips t JOIN orders o ON o.id=t.order_id WHERE o.reservation_id=$1`, direct.ID).Scan(&tipCount))
	if tipCount != 0 {
		t.Fatal("direct reservation created tips")
	}
}
