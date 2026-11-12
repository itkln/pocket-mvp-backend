package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/dinein"
	"pocket-mvp-backend/internal/money"
)

// V-TBL-05: a guest calls the staff to settle; a waiter confirms the in-person payment and the
// visit closes atomically. The call alone is not a payment, a changed bill must be checked again,
// a repeated confirmation is idempotent and the next order opens a new visit.
func TestTableBillSettlementIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	ownerID := seedUser(t, db, "settle-owner@example.test", "venue_owner")
	waiterID := seedUser(t, db, "settle-waiter@example.test", "")
	strangerID := seedUser(t, db, "settle-stranger@example.test", "")
	venueID := seedVenue(t, db, ownerID, venueSeed{Slug: "settle", Settings: `{"floor_plan":[{"id":"f1","name":"Main","tables":[{"id":"07","seats":4,"x":10,"y":10}]}]}`})
	seedStaff(t, db, venueID, waiterID, "settle-waiter@example.test", "waiter", ownerID)
	_, itemID := seedMenuItem(t, db, venueID, "Food", false, "Pasta", 1200)
	service := dinein.NewService(dinein.NewPostgresRepository(db), access.NewVenueAuthorizer(db), testProtector(t), "http://localhost:3001")
	qr, err := service.OwnerQRCode(ctx, ownerID, venueID, "07", false)
	must(t, err)
	token := qr.URL[strings.LastIndex(qr.URL, "/")+1:]
	orders := 0
	order := func() {
		t.Helper()
		orders++
		_, err := service.Create(ctx, dinein.CreateInput{Token: token, IdempotencyKey: fmt.Sprintf("settle-order-%04d", orders),
			Items: []dinein.ItemInput{{MenuItemID: itemID, Quantity: 1, ExpectedPriceMinor: 1200}}})
		must(t, err)
	}
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		must(t, db.QueryRow(ctx, query, args...).Scan(&n))
		return n
	}

	// Nothing to settle before the first order.
	if _, err = service.CallStaff(ctx, token); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("call without a visit: %v", err)
	}
	order()
	order()

	// Calling twice keeps the first call; the visit stays open and orderable.
	first, err := service.CallStaff(ctx, token)
	must(t, err)
	again, err := service.CallStaff(ctx, token)
	must(t, err)
	if first.PaymentRequestedAt == nil || !again.PaymentRequestedAt.Equal(*first.PaymentRequestedAt) || again.Status != "open" || again.TotalMinor != 2400 {
		t.Fatalf("staff call: %+v then %+v", first, again)
	}
	if n := count(`SELECT count(*) FROM audit_logs WHERE entity_id=$1 AND action='bill.payment_requested'`, first.ID); n != 1 {
		t.Fatalf("duplicate call records: %d", n)
	}
	if n := count(`SELECT count(*) FROM payments WHERE bill_id=$1`, first.ID); n != 0 {
		t.Fatal("a call was recorded as a payment")
	}

	// The waiter sees the called table with its dishes and amount; to outsiders the venue does not exist.
	bills, err := service.OpenBills(ctx, waiterID, venueID)
	must(t, err)
	if len(bills) != 1 || bills[0].ID != first.ID || bills[0].TableIdentifier != "07" || bills[0].PaymentRequestedAt == nil ||
		bills[0].TotalMinor != 2400 || len(bills[0].Items) != 2 {
		t.Fatalf("open bills: %+v", bills)
	}
	if _, err = service.OpenBills(ctx, strangerID, venueID); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("stranger lists bills: %v", err)
	}
	if _, err = service.Settle(ctx, strangerID, venueID, first.ID, dinein.SettleInput{ExpectedTotalMinor: 2400, Method: "cash"}); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("stranger settles: %v", err)
	}
	if _, err = service.Settle(ctx, waiterID, venueID, first.ID, dinein.SettleInput{ExpectedTotalMinor: 2400, Method: "crypto"}); !errors.Is(err, appfault.ErrInvalidInput) {
		t.Fatalf("unknown method: %v", err)
	}

	// A dish ordered after the waiter opened the bill: the old amount is refused, nothing closes.
	order()
	if _, err = service.Settle(ctx, waiterID, venueID, first.ID, dinein.SettleInput{ExpectedTotalMinor: 2400, Method: "card"}); !errors.Is(err, dinein.ErrBillChanged) {
		t.Fatalf("stale amount: %v", err)
	}
	if bill, _ := service.CurrentBill(ctx, token); bill == nil || bill.Status != "open" || bill.TotalMinor != 3600 {
		t.Fatalf("bill after a refused settlement: %+v", bill)
	}

	// An order racing the settlement lands either before it (amount changes, refused) or after it
	// (new visit). It is never added to the closing bill.
	var wg sync.WaitGroup
	var settleErr, orderErr error
	var settled dinein.Settlement
	wg.Add(2)
	go func() {
		defer wg.Done()
		settled, settleErr = service.Settle(ctx, waiterID, venueID, first.ID, dinein.SettleInput{ExpectedTotalMinor: 3600, Method: "card"})
	}()
	go func() {
		defer wg.Done()
		_, orderErr = service.Create(ctx, dinein.CreateInput{Token: token, IdempotencyKey: "settle-order-race",
			Items: []dinein.ItemInput{{MenuItemID: itemID, Quantity: 1, ExpectedPriceMinor: 1200}}})
	}()
	wg.Wait()
	must(t, orderErr)
	if errors.Is(settleErr, dinein.ErrBillChanged) {
		settled, settleErr = service.Settle(ctx, waiterID, venueID, first.ID, dinein.SettleInput{ExpectedTotalMinor: 4800, Method: "card"})
	}
	must(t, settleErr)
	var itemsOnBill money.Minor
	must(t, db.QueryRow(ctx, `SELECT COALESCE(sum(oi.total_minor),0) FROM order_items oi JOIN orders o ON o.id=oi.order_id WHERE o.bill_id=$1`, first.ID).Scan(&itemsOnBill))
	if settled.Status != "paid" || settled.AmountMinor != itemsOnBill || settled.Method != "card" || settled.SettledByUserID != waiterID || settled.TableIdentifier != "07" {
		t.Fatalf("settlement %+v, dishes on the bill %d", settled, itemsOnBill)
	}

	// The payment and the closed visit are recorded together; orders keep their kitchen status.
	var status, provider, method, recordedBy string
	var amount money.Minor
	must(t, db.QueryRow(ctx, `
		SELECT b.status, p.provider, p.method, p.recorded_by_user_id::text, p.amount_minor
		FROM bills b JOIN payments p ON p.bill_id=b.id WHERE b.id=$1 AND b.closed_at IS NOT NULL AND p.status='paid'`, first.ID).Scan(
		&status, &provider, &method, &recordedBy, &amount))
	if status != "paid" || provider != "staff" || method != "card" || recordedBy != waiterID || amount != settled.AmountMinor {
		t.Fatalf("recorded payment: %s %s %s %s %d", status, provider, method, recordedBy, amount)
	}
	if n := count(`SELECT count(*) FROM orders WHERE bill_id=$1 AND status<>'new'`, first.ID); n != 0 {
		t.Fatal("settlement changed kitchen statuses")
	}

	// Confirming again returns the same settlement and records nothing new.
	repeat, err := service.Settle(ctx, ownerID, venueID, first.ID, dinein.SettleInput{ExpectedTotalMinor: 1, Method: "cash"})
	must(t, err)
	if repeat.AmountMinor != settled.AmountMinor || repeat.Method != "card" || !repeat.SettledAt.Equal(settled.SettledAt) {
		t.Fatalf("repeated confirmation: %+v vs %+v", repeat, settled)
	}
	if n := count(`SELECT count(*) FROM payments WHERE bill_id=$1`, first.ID); n != 1 {
		t.Fatalf("payments after a repeat: %d", n)
	}

	// The guest's bill is now the next visit (if the racing order came after) or empty; the paid
	// visit keeps its history and is never shown as current.
	current, err := service.CurrentBill(ctx, token)
	must(t, err)
	if current != nil && (current.ID == first.ID || current.PaymentRequestedAt != nil) {
		t.Fatalf("old visit shown as current: %+v", current)
	}
	if current == nil {
		order()
		if current, err = service.CurrentBill(ctx, token); err != nil || current == nil || current.ID == first.ID {
			t.Fatalf("next order did not open a new visit: %+v %v", current, err)
		}
	}
	if n := count(`SELECT count(*) FROM orders WHERE bill_id=$1`, first.ID); n < 3 {
		t.Fatalf("paid visit lost its orders: %d", n)
	}

	// A visit whose every dish was cancelled closes without a payment, also idempotently.
	_, err = db.Exec(ctx, `UPDATE order_items SET status='cancelled' WHERE order_id IN (SELECT id FROM orders WHERE bill_id=$1)`, current.ID)
	must(t, err)
	empty, err := service.Settle(ctx, waiterID, venueID, current.ID, dinein.SettleInput{ExpectedTotalMinor: 0, Method: "cash"})
	must(t, err)
	if empty.Status != "void" || empty.AmountMinor != 0 {
		t.Fatalf("empty visit: %+v", empty)
	}
	if _, err = service.Settle(ctx, waiterID, venueID, current.ID, dinein.SettleInput{ExpectedTotalMinor: 0, Method: "cash"}); err != nil {
		t.Fatalf("repeat on an empty visit: %v", err)
	}
	if n := count(`SELECT count(*) FROM payments WHERE bill_id=$1`, current.ID); n != 0 {
		t.Fatal("a payment for an empty visit")
	}
	if bills, err = service.OpenBills(ctx, waiterID, venueID); err != nil || len(bills) != 0 {
		t.Fatalf("open bills after settling: %+v %v", bills, err)
	}
}
