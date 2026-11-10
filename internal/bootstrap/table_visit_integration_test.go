package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/httpapi"
	"pocket-mvp-backend/internal/modules/dinein"
	"pocket-mvp-backend/internal/security"
	"strings"
	"sync"
	"testing"
)

func TestTableVisitIntegration(t *testing.T) {
	must := func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	db := resetTestDatabase(t)
	ctx := context.Background()
	var ownerID, venueID, categoryID, itemID string
	must(t, db.QueryRow(ctx, `INSERT INTO users(email,password_hash,first_name,last_name,account_role) VALUES('qr-owner@example.test','test','QR','Owner','venue_owner') RETURNING id::text`).Scan(&ownerID))
	must(t, db.QueryRow(ctx, `INSERT INTO venues(owner_user_id,slug,name,address_line1,city,country_code,status,settings)
  VALUES($1,'qr-test','QR Test','Main 1','Bratislava','SK','active','{"floor_plan":[{"id":"floor-1","name":"Main","tables":[{"id":"07","seats":4,"x":12,"y":30}]}]}') RETURNING id::text`, ownerID).Scan(&venueID))
	must(t, db.QueryRow(ctx, `INSERT INTO menu_categories(venue_id,name,preorder_enabled) VALUES($1,'Food',false) RETURNING id::text`, venueID).Scan(&categoryID))
	must(t, db.QueryRow(ctx, `INSERT INTO menu_items(venue_id,category_id,name,price_minor) VALUES($1,$2,'Pasta',1200) RETURNING id::text`, venueID, categoryID).Scan(&itemID))
	protector, err := security.NewDataProtector(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	must(t, err)
	service := dinein.NewService(dinein.NewPostgresRepository(db), access.NewVenueAuthorizer(db), protector, "http://localhost:3001")
	qr, err := service.OwnerQRCode(ctx, ownerID, venueID, "07", false)
	must(t, err)
	token := qr.URL[strings.LastIndex(qr.URL, "/")+1:]
	bill, err := service.CurrentBill(ctx, token)
	must(t, err)
	if bill != nil {
		t.Fatal("read created a visit")
	}
	var count int
	must(t, db.QueryRow(ctx, `SELECT count(*) FROM bills WHERE venue_id=$1`, venueID).Scan(&count))
	if count != 0 {
		t.Fatal("empty visit created")
	}
	input := dinein.CreateInput{Token: token, Items: []dinein.ItemInput{{MenuItemID: itemID, Quantity: 1, ExpectedPriceMinor: 1200}}}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			in := input
			in.IdempotencyKey = fmt.Sprintf("visit-concurrent-%02d", i)
			_, e := service.Create(ctx, in)
			errs <- e
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		must(t, e)
	}
	bill, err = service.CurrentBill(ctx, token)
	must(t, err)
	if bill == nil || bill.TotalMinor != 9600 || len(bill.Items) != 8 {
		t.Fatalf("shared bill: %+v", bill)
	}
	oldID := bill.ID
	must(t, db.QueryRow(ctx, `SELECT count(*) FROM bills WHERE venue_id=$1`, venueID).Scan(&count))
	if count != 1 {
		t.Fatal("duplicate visit")
	}
	// Read the same visit with independent HTTP clients, with no caller-supplied table IDs.
	api := httpapi.New(httpapi.Dependencies{DineIn: service, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/v1/public/table-bill", strings.NewReader(fmt.Sprintf(`{"token":%q}`, token)))
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("bill HTTP: %d %s", rec.Code, rec.Body.String())
		}
		var payload struct {
			Bill *dinein.Bill `json:"bill"`
		}
		must(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		if payload.Bill.ID != oldID {
			t.Fatal("phone received another visit")
		}
	}
	req := httptest.NewRequest("POST", "/api/v1/public/table-bill", strings.NewReader(fmt.Sprintf(`{"token":%q,"table_id":"10000000-0000-4000-8000-000000000099"}`, token)))
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatal("unexpected fields accepted")
	}

	// Menu prices are not used to reprice previously ordered dishes.
	_, err = db.Exec(ctx, `UPDATE menu_items SET price_minor=9999 WHERE id=$1`, itemID)
	must(t, err)
	_, err = db.Exec(ctx, `UPDATE orders SET status='cancelled' WHERE id=$1`, bill.Items[0].OrderID)
	must(t, err)
	_, err = db.Exec(ctx, `UPDATE order_items SET status='cancelled' WHERE order_id=$1`, bill.Items[1].OrderID)
	must(t, err)
	bill, err = service.CurrentBill(ctx, token)
	must(t, err)
	if bill.TotalMinor != 7200 || len(bill.Items) != 6 {
		t.Fatalf("cancellation/snapshot: %+v", bill)
	}
	// A different valid table QR has no access to this visit.
	_, err = db.Exec(ctx, `UPDATE venues SET settings='{}' WHERE id=$1`, venueID)
	must(t, err)
	var layoutID string
	must(t, db.QueryRow(ctx, `SELECT layout_id::text FROM venue_tables WHERE id=$1`, qr.ID).Scan(&layoutID))
	_, err = db.Exec(ctx, `INSERT INTO venue_tables(venue_id,layout_id,identifier,seats,qr_token_hash) VALUES($1,$2,'08',2,'unissued:test')`, venueID, layoutID)
	must(t, err)
	other, err := service.OwnerQRCode(ctx, ownerID, venueID, "08", false)
	must(t, err)
	otherToken := other.URL[strings.LastIndex(other.URL, "/")+1:]
	empty, err := service.CurrentBill(ctx, otherToken)
	must(t, err)
	if empty != nil {
		t.Fatal("cross-table bill leak")
	}
	// Settlement is a fixture here; its staff/payment workflow is V-TBL-05/06.
	_, err = db.Exec(ctx, `UPDATE bills SET status='paid',closed_at=now() WHERE id=$1`, oldID)
	must(t, err)
	bill, err = service.CurrentBill(ctx, token)
	must(t, err)
	if bill != nil {
		t.Fatal("paid visit visible")
	}
	input.IdempotencyKey = "visit-after-settlement"
	input.Items[0].ExpectedPriceMinor = 9999
	_, err = service.Create(ctx, input)
	must(t, err)
	bill, err = service.CurrentBill(ctx, token)
	must(t, err)
	if bill.ID == oldID || bill.TotalMinor != 9999 {
		t.Fatalf("new visit: %+v", bill)
	}
	if _, err = service.CurrentBill(ctx, "invalid"); err == nil {
		t.Fatal("invalid QR accepted")
	}
	_, err = service.OwnerQRCode(ctx, ownerID, venueID, "07", true)
	must(t, err)
	if _, err = service.CurrentBill(ctx, token); err == nil {
		t.Fatal("revoked QR accepted")
	}
}
