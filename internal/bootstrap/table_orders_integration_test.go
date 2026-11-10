package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/httpapi"
	"pocket-mvp-backend/internal/modules/dinein"
	"pocket-mvp-backend/internal/modules/ordering"
	"pocket-mvp-backend/internal/security"
)

func TestTableQRIntegration(t *testing.T) {
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
	if !qr.OrderingEnabled {
		t.Fatal("owner QR does not report ordering enabled")
	}
	token := qr.URL[strings.LastIndex(qr.URL, "/")+1:]
	table, err := service.Resolve(ctx, token)
	must(t, err)
	if table.VenueID != venueID || table.Identifier != "07" || !table.OrderingEnabled {
		t.Fatalf("wrong table: %+v", table)
	}
	again, err := service.OwnerQRCode(ctx, ownerID, venueID, "07", false)
	must(t, err)
	if again.URL != qr.URL {
		t.Fatal("download invalidated the printed QR")
	}
	if _, err := service.OwnerQRCode(ctx, ownerID, venueID, "missing", false); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("missing table: %v", err)
	}
	if _, err := service.OwnerQRCode(ctx, "10000000-0000-4000-8000-000000000009", venueID, "07", true); err == nil {
		t.Fatal("foreign owner could rotate QR")
	}
	var billID string
	must(t, db.QueryRow(ctx, `INSERT INTO bills(venue_id,venue_table_id,subtotal_minor,total_minor) VALUES($1,$2,700,700) RETURNING id::text`, venueID, table.ID).Scan(&billID))
	var otherOrder string
	must(t, db.QueryRow(ctx, `INSERT INTO orders(venue_id,bill_id,venue_table_id,channel,total_minor,subtotal_minor) VALUES($1,$2,$3,'dine_in',700,700) RETURNING id::text`, venueID, billID, table.ID).Scan(&otherOrder))
	_, err = db.Exec(ctx, `INSERT INTO order_items(order_id,item_name,unit_price_minor,quantity,total_minor) VALUES($1,'Private other guest dish',700,1,700)`, otherOrder)
	must(t, err)
	input := dinein.CreateInput{Token: token, IdempotencyKey: "qr-integration-first-order", Notes: "No onions", Items: []dinein.ItemInput{{MenuItemID: itemID, Quantity: 2, ExpectedPriceMinor: 1200}}}
	created, err := service.Create(ctx, input)
	must(t, err)
	if created.TotalMinor != 2400 || len(created.Items) != 1 || created.Items[0].Name != "Pasta" {
		t.Fatalf("order snapshots or privacy: %+v", created)
	}
	detail, err := ordering.NewService(ordering.NewPostgresRepository(db), access.NewVenueAuthorizer(db)).Get(ctx, ownerID, venueID, created.ID)
	must(t, err)
	if detail.TableID != table.ID || detail.BillID != billID || detail.Notes != input.Notes || detail.Currency != "EUR" || len(detail.Items) != 1 || detail.Items[0].Quantity != 2 || detail.Items[0].TotalMinor != 2400 || detail.PlacedAt == nil {
		t.Fatalf("queue QR details: %+v", detail)
	}
	var orderBill, orderTable string
	var total int
	must(t, db.QueryRow(ctx, `SELECT bill_id::text,venue_table_id::text FROM orders WHERE id=$1`, created.ID).Scan(&orderBill, &orderTable))
	must(t, db.QueryRow(ctx, `SELECT total_minor FROM bills WHERE id=$1`, billID).Scan(&total))
	if orderBill != billID || orderTable != table.ID || total != 3100 {
		t.Fatalf("wrong bill/table/total: %s %s %d", orderBill, orderTable, total)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := service.Create(ctx, input)
			if err == nil && result.ID != created.ID {
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
	var count int
	must(t, db.QueryRow(ctx, `SELECT count(*) FROM orders WHERE idempotency_key=$1`, input.IdempotencyKey).Scan(&count))
	if count != 1 {
		t.Fatal("duplicate orders")
	}
	different := input
	different.Notes = "Changed"
	if _, err := service.Create(ctx, different); !errors.Is(err, appfault.ErrConflict) {
		t.Fatalf("reused key changed payload: %v", err)
	}
	if _, err := service.Lookup(ctx, token, "unknown-secret-order-key"); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("foreign order disclosure: %v", err)
	}
	vendor := ordering.NewPostgresRepository(db)
	orders, err := vendor.List(ctx, venueID)
	must(t, err)
	if len(orders) != 2 || orders[0].ID != created.ID || orders[0].Source != "Стол 07" || len(orders[0].Items) != 1 {
		t.Fatalf("vendor order: %+v", orders)
	}
	_, err = vendor.UpdateStatus(ctx, venueID, created.ID, "preparing")
	must(t, err)
	saved, err := service.Lookup(ctx, token, input.IdempotencyKey)
	must(t, err)
	if saved.Status != "preparing" {
		t.Fatal("status not persisted")
	}
	handler := httpapi.New(httpapi.Dependencies{DineIn: service, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SessionCookie: "session"})
	call := func(path string, body any) *httptest.ResponseRecorder {
		payload, _ := json.Marshal(body)
		response := httptest.NewRecorder()
		request := httptest.NewRequest("POST", path, bytes.NewReader(payload))
		handler.ServeHTTP(response, request)
		return response
	}
	response := call("/api/v1/public/table-qr/resolve", map[string]any{"token": token})
	if response.Code != 200 || strings.Contains(response.Body.String(), billID) {
		t.Fatalf("resolve contract: %d %s", response.Code, response.Body.String())
	}
	response = call("/api/v1/public/table-orders/lookup", map[string]any{"token": token, "idempotency_key": input.IdempotencyKey})
	if response.Code != 200 || strings.Contains(response.Body.String(), "Private other guest") || strings.Contains(response.Body.String(), billID) {
		t.Fatalf("order privacy: %d %s", response.Code, response.Body.String())
	}
	guest := input
	guest.IdempotencyKey = "qr-integration-http-guest"
	response = call("/api/v1/public/table-orders", guest)
	if response.Code != 201 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("guest order HTTP: %d %s", response.Code, response.Body.String())
	}
	for _, test := range []struct {
		name   string
		mutate func(*dinein.CreateInput)
		want   error
	}{
		{"price", func(v *dinein.CreateInput) { v.Items[0].ExpectedPriceMinor = 100 }, dinein.ErrPriceChanged},
		{"foreign item", func(v *dinein.CreateInput) { v.Items[0].MenuItemID = "50000000-0000-4000-8000-000000000099" }, dinein.ErrItemUnavailable},
		{"malformed id", func(v *dinein.CreateInput) { v.Items[0].MenuItemID = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" }, appfault.ErrInvalidInput},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := input
			candidate.IdempotencyKey = "qr-invalid-case-" + test.name
			candidate.Items = append([]dinein.ItemInput(nil), input.Items...)
			test.mutate(&candidate)
			if _, err := service.Create(ctx, candidate); !errors.Is(err, test.want) {
				t.Fatalf("want %v, got %v", test.want, err)
			}
		})
	}
	_, err = db.Exec(ctx, `UPDATE bills SET status='payment_pending' WHERE id=$1`, billID)
	must(t, err)
	next := input
	next.IdempotencyKey = "qr-integration-next-order"
	if _, err := service.Create(ctx, next); !errors.Is(err, dinein.ErrBillLocked) {
		t.Fatalf("pending payment bill: %v", err)
	}
	_, err = db.Exec(ctx, `UPDATE bills SET status='paid',closed_at=now() WHERE id=$1`, billID)
	must(t, err)
	nextOrder, err := service.Create(ctx, next)
	must(t, err)
	must(t, db.QueryRow(ctx, `SELECT bill_id::text FROM orders WHERE id=$1`, nextOrder.ID).Scan(&orderBill))
	if orderBill == billID {
		t.Fatal("used paid bill")
	}
	rotated, err := service.OwnerQRCode(ctx, ownerID, venueID, "07", true)
	must(t, err)
	if rotated.Version != qr.Version+1 || rotated.URL == qr.URL {
		t.Fatal("QR did not rotate")
	}
	if _, err := service.Resolve(ctx, token); !errors.Is(err, dinein.ErrQRInvalid) {
		t.Fatalf("revoked QR: %v", err)
	}
	if _, err := service.Create(ctx, input); !errors.Is(err, dinein.ErrQRInvalid) {
		t.Fatalf("revoked QR created order: %v", err)
	}
	response = call("/api/v1/public/table-qr/resolve", map[string]any{"token": token})
	if response.Code != 404 {
		t.Fatal("revoked QR HTTP status")
	}
	token = rotated.URL[strings.LastIndex(rotated.URL, "/")+1:]
	_, err = service.Resolve(ctx, token)
	must(t, err)
	_, err = db.Exec(ctx, `UPDATE venues SET settings=jsonb_set(settings,'{dine_in}','false') WHERE id=$1`, venueID)
	must(t, err)
	disabled, err := service.Resolve(ctx, token)
	must(t, err)
	if disabled.OrderingEnabled {
		t.Fatal("disabled venue permits ordering")
	}
	guest.Token, guest.IdempotencyKey = token, "qr-disabled-ordering-guest"
	if _, err := service.Create(ctx, guest); !errors.Is(err, dinein.ErrBillLocked) {
		t.Fatalf("disabled ordering: %v", err)
	}
	_, err = db.Exec(ctx, `UPDATE venues SET settings=jsonb_set(settings,'{floor_plan}','[{"name":"Main","tables":[]}]') WHERE id=$1`, venueID)
	must(t, err)
	if _, err := service.Resolve(ctx, token); !errors.Is(err, dinein.ErrQRInvalid) {
		t.Fatalf("removed table QR: %v", err)
	}
	_, err = db.Exec(ctx, `UPDATE venues SET settings=jsonb_set(settings,'{floor_plan}','[]') WHERE id=$1`, venueID)
	must(t, err)
	if _, err := service.Resolve(ctx, token); !errors.Is(err, dinein.ErrQRInvalid) {
		t.Fatalf("empty saved plan QR: %v", err)
	}
	if _, err := service.OwnerQRCode(ctx, ownerID, venueID, "07", false); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("empty saved plan issued QR: %v", err)
	}
}
