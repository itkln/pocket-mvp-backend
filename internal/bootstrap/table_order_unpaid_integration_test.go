package bootstrap

import (
	"bytes"
	"context"
	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/modules/dinein"
	"pocket-mvp-backend/internal/modules/ordering"
	"pocket-mvp-backend/internal/money"
	"pocket-mvp-backend/internal/security"
	"strings"
	"testing"
)

func TestTableOrderWithoutPrepaymentIntegration(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	db := resetTestDatabase(t)
	ctx := context.Background()
	var ownerID, venueID, categoryID, itemID string
	must(db.QueryRow(ctx, `INSERT INTO users(email,password_hash,first_name,last_name,account_role) VALUES('qr-owner@example.test','test','QR','Owner','venue_owner') RETURNING id::text`).Scan(&ownerID))
	must(db.QueryRow(ctx, `INSERT INTO venues(owner_user_id,slug,name,address_line1,city,country_code,status,settings)
  VALUES($1,'qr-test','QR Test','Main 1','Bratislava','SK','active','{"floor_plan":[{"id":"floor-1","name":"Main","tables":[{"id":"07","seats":4,"x":12,"y":30}]}]}') RETURNING id::text`, ownerID).Scan(&venueID))
	must(db.QueryRow(ctx, `INSERT INTO menu_categories(venue_id,name,preorder_enabled) VALUES($1,'Food',false) RETURNING id::text`, venueID).Scan(&categoryID))
	must(db.QueryRow(ctx, `INSERT INTO menu_items(venue_id,category_id,name,price_minor) VALUES($1,$2,'Pasta',1200) RETURNING id::text`, venueID, categoryID).Scan(&itemID))
	protector, err := security.NewDataProtector(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	must(err)
	service := dinein.NewService(dinein.NewPostgresRepository(db), access.NewVenueAuthorizer(db), protector, "http://localhost:3001")
	qr, err := service.OwnerQRCode(ctx, ownerID, venueID, "07", false)
	must(err)
	token := qr.URL[strings.LastIndex(qr.URL, "/")+1:]
	bill, err := service.CurrentBill(ctx, token)
	must(err)
	if bill != nil {
		t.Fatal("read created a visit")
	}
	var count int
	must(db.QueryRow(ctx, `SELECT count(*) FROM bills WHERE venue_id=$1`, venueID).Scan(&count))
	if count != 0 {
		t.Fatal("empty visit created")
	}
	input := dinein.CreateInput{Token: token, Items: []dinein.ItemInput{{MenuItemID: itemID, Quantity: 1, ExpectedPriceMinor: 1200}}}

	queue := ordering.NewService(ordering.NewPostgresRepository(db), access.NewVenueAuthorizer(db))
	var first dinein.Order
	var visit string
	for i, key := range []string{"unpaid-order-first-key", "unpaid-order-second-key"} {
		input.IdempotencyKey = key
		input.Notes = "No onions"
		order, err := service.Create(ctx, input)
		must(err)
		if i == 0 {
			first = order
		}
		bill, err := service.CurrentBill(ctx, token)
		must(err)
		if i == 0 {
			visit = bill.ID
		}
		if bill.ID != visit || bill.Status != "open" || bill.TotalMinor != money.Minor((i+1)*1200) {
			t.Fatalf("wrong unpaid visit: %+v", bill)
		}
		detail, err := queue.Get(ctx, ownerID, venueID, order.ID)
		must(err)
		if detail.BillID != visit || detail.TableID != qr.ID || detail.TableLabel != "07" || detail.Status != "new" || detail.Notes != input.Notes {
			t.Fatalf("wrong service order: %+v", detail)
		}
		retry, err := service.Create(ctx, input)
		must(err)
		if retry.ID != order.ID {
			t.Fatal("retry duplicated order")
		}
	}
	active, err := queue.ListPage(ctx, ownerID, venueID, ordering.Query{Channel: "dine_in", TableID: qr.ID})
	must(err)
	if len(active.Orders) != 2 {
		t.Fatalf("service queue has %d orders", len(active.Orders))
	}
	previous := "new"
	for _, status := range []string{"preparing", "served", "completed"} {
		_, err := queue.UpdateStatus(ctx, ownerID, venueID, first.ID, status, previous)
		must(err)
		bill, err := service.CurrentBill(ctx, token)
		must(err)
		if bill.ID != visit || bill.Status != "open" || bill.TotalMinor != 2400 {
			t.Fatalf("service status %s changed bill: %+v", status, bill)
		}
		previous = status
	}
	// A retry even after service completion must return the same order and total.
	input.IdempotencyKey = "unpaid-order-first-key"
	replay, err := service.Create(ctx, input)
	must(err)
	if replay.ID != first.ID || replay.Status != "completed" {
		t.Fatalf("wrong replay: %+v", replay)
	}
	must(db.QueryRow(ctx, `SELECT count(*) FROM orders WHERE bill_id=$1`, visit).Scan(&count))
	if count != 2 {
		t.Fatal("duplicate order after completion")
	}
	must(db.QueryRow(ctx, `SELECT count(*) FROM payments WHERE bill_id=$1`, visit).Scan(&count))
	if count != 0 {
		t.Fatal("unpaid ordering created a payment")
	}
	var closed bool
	must(db.QueryRow(ctx, `SELECT closed_at IS NOT NULL FROM bills WHERE id=$1`, visit).Scan(&closed))
	if closed {
		t.Fatal("order completion closed visit")
	}
}
