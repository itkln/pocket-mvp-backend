package bootstrap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/httpapi"
	"pocket-mvp-backend/internal/modules/dinein"
	"pocket-mvp-backend/internal/modules/identity"
	"pocket-mvp-backend/internal/security"
)

type venueHistoryIdentity struct {
	httpapi.IdentityService
	userID string
}

func (auth venueHistoryIdentity) Authenticate(_ context.Context, token string) (identity.User, error) {
	if token != "valid-account-session" {
		return identity.User{}, errors.New("expired session")
	}
	return identity.User{ID: auth.userID}, nil
}

func TestVenueOrderHistoryIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var owner, customerA, customerB, venueID, otherVenue, category, dish string
	for index, target := range []*string{&owner, &customerA, &customerB} {
		must(db.QueryRow(ctx, `INSERT INTO users(email,password_hash,first_name,last_name) VALUES($1,'test','History','Guest') RETURNING id::text`, []string{"history-owner@example.test", "history-a@example.test", "history-b@example.test"}[index]).Scan(target))
	}
	for index, target := range []*string{&venueID, &otherVenue} {
		must(db.QueryRow(ctx, `INSERT INTO venues(owner_user_id,slug,name,address_line1,city,country_code,status,settings)
		 VALUES($1,$2,'History Bistro','Main 1','Bratislava','SK','active','{"floor_plan":[{"name":"Main","tables":[{"id":"1","seats":4},{"id":"5","seats":4}]}]}') RETURNING id::text`, owner, []string{"history-main", "history-other"}[index]).Scan(target))
	}
	must(db.QueryRow(ctx, `INSERT INTO menu_categories(venue_id,name) VALUES($1,'Food') RETURNING id::text`, venueID).Scan(&category))
	must(db.QueryRow(ctx, `INSERT INTO menu_items(venue_id,category_id,name,price_minor) VALUES($1,$2,'Pasta',1200) RETURNING id::text`, venueID, category).Scan(&dish))
	protector, err := security.NewDataProtector(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	must(err)
	service := dinein.NewService(dinein.NewPostgresRepository(db), access.NewVenueAuthorizer(db), protector, "http://localhost:3001")
	qr := func(venue, table string, rotate bool) string {
		result, err := service.OwnerQRCode(ctx, owner, venue, table, rotate)
		must(err)
		return result.URL[strings.LastIndex(result.URL, "/")+1:]
	}
	token1, token5 := qr(venueID, "1", false), qr(venueID, "5", false)
	deviceA := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32))
	deviceB := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{4}, 32))
	create := func(token, key, device, customer string) dinein.Order {
		order, err := service.Create(ctx, dinein.CreateInput{Token: token, IdempotencyKey: key, GuestToken: device, CustomerUserID: customer, Notes: "No onions", Items: []dinein.ItemInput{{MenuItemID: dish, Quantity: 2, ExpectedPriceMinor: 1200}}})
		must(err)
		return order
	}
	first := create(token1, "history-device-a-first", deviceA, "")
	create(token1, "history-device-b-first", deviceB, "")
	accountA := create(token1, "history-account-a-first", "", customerA)
	create(token1, "history-account-b-first", "", customerB)
	legacy := create(token1, "history-legacy-revoked", "", "")
	qr(venueID, "1", true)
	if _, err := service.Resolve(ctx, token1); !errors.Is(err, dinein.ErrQRInvalid) {
		t.Fatal("old QR must be revoked")
	}
	history := func(input dinein.HistoryInput) dinein.HistoryPage {
		page, err := service.History(ctx, input)
		must(err)
		return page
	}
	input := dinein.HistoryInput{VenueID: venueID, GuestToken: deviceA, LegacyKeys: []string{"history-legacy-revoked", "history-device-b-first", "history-account-b-first"}}
	page := history(input)
	if page.Scope != "device" || len(page.Orders) != 2 {
		t.Fatalf("device history leaked or migration failed: %+v", page)
	}
	if len(history(input).Orders) != 2 {
		t.Fatal("legacy import duplicated orders")
	}
	if len(history(dinein.HistoryInput{VenueID: otherVenue, GuestToken: deviceA}).Orders) != 0 {
		t.Fatal("mixed venues")
	}
	if len(history(dinein.HistoryInput{VenueID: venueID, GuestToken: deviceB, LegacyKeys: []string{"history-legacy-revoked"}}).Orders) != 1 {
		t.Fatal("claimed legacy order leaked to another device")
	}
	page = history(dinein.HistoryInput{VenueID: venueID, CustomerUserID: customerA, GuestToken: deviceB})
	if page.Scope != "account" || len(page.Orders) != 1 || page.Orders[0].ID != accountA.ID {
		t.Fatal("account history must ignore the device and other accounts")
	}
	if len(history(dinein.HistoryInput{VenueID: venueID, CustomerUserID: customerB, LegacyKeys: []string{"history-account-a-first", "history-legacy-revoked"}}).Orders) != 1 {
		t.Fatal("account could claim another owner's order")
	}
	if first.Items[0].MenuItemID != dish || first.QRVersion == nil || *first.QRVersion != 1 {
		t.Fatal("missing repeat identity or QR snapshot")
	}
	var oldTable, newTable, oldBill, newBill string
	must(db.QueryRow(ctx, `SELECT venue_table_id::text,bill_id::text FROM orders WHERE id=$1`, first.ID).Scan(&oldTable, &oldBill))
	must(db.QueryRow(ctx, `SELECT id::text FROM venue_tables WHERE venue_id=$1 AND identifier='5'`, venueID).Scan(&newTable))
	must(db.QueryRow(ctx, `INSERT INTO bills(venue_id,venue_table_id) VALUES($1,$2) RETURNING id::text`, venueID, newTable).Scan(&newBill))
	repeated := create(token5, "history-repeat-current-five", deviceA, "")
	var orderTable, orderBill string
	must(db.QueryRow(ctx, `SELECT venue_table_id::text,bill_id::text FROM orders WHERE id=$1`, repeated.ID).Scan(&orderTable, &orderBill))
	if orderTable != newTable || orderTable == oldTable || orderBill != newBill || orderBill == oldBill {
		t.Fatal("repeat used the old table or bill")
	}
	if first.TableIdentifier != "1" || repeated.TableIdentifier != "5" {
		t.Fatal("wrong receipt table")
	}
	if _, err := service.Lookup(ctx, token5, "history-repeat-current-five", dinein.HistoryInput{GuestToken: deviceB}); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatal("another guest could read a claimed receipt")
	}
	if _, err := service.Lookup(ctx, token5, "history-repeat-current-five", dinein.HistoryInput{GuestToken: deviceA}); err != nil {
		t.Fatal(err)
	}
	must(func() error {
		_, err := db.Exec(ctx, `UPDATE menu_items SET name='New pasta',price_minor=1500,is_available=false WHERE id=$1`, dish)
		return err
	}())
	page = history(input)
	for _, order := range page.Orders {
		if order.TotalMinor != 2400 || order.Items[0].Name != "Pasta" || order.Items[0].UnitPriceMinor != 1200 {
			t.Fatal("menu changes modified a past receipt")
		}
	}
	if legacy.ID == "" {
		t.Fatal("missing legacy fixture")
	}
	// A persistent cookie restores history across visits. A QR or client user ID cannot grant access.
	handler := httpapi.New(httpapi.Dependencies{DineIn: service, Identity: venueHistoryIdentity{userID: customerA}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SessionCookie: "session", SessionSecure: true})
	call := func(path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
		payload, _ := json.Marshal(body)
		request := httptest.NewRequest("POST", path, bytes.NewReader(payload))
		if cookie != nil {
			request.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	response := call("/api/v1/public/table-qr/resolve", map[string]any{"token": token5}, nil)
	cookies := response.Result().Cookies()
	if response.Code != 200 || len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].MaxAge < 86400 || cookies[0].Path != "/api/v1/public" {
		t.Fatal("guest identity cookie is not established before table interaction")
	}
	response = call("/api/v1/public/table-orders/history", map[string]any{"venue_id": venueID}, &http.Cookie{Name: "pocket_table_guest", Value: deviceA})
	if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Body.String(), "history-device-b-first") || strings.Contains(response.Body.String(), "guest_history_hash") {
		t.Fatalf("history contract: %d %s", response.Code, response.Body.String())
	}
	response = call("/api/v1/public/table-orders/history", map[string]any{"venue_id": venueID}, cookies[0])
	var empty dinein.HistoryPage
	must(json.Unmarshal(response.Body.Bytes(), &empty))
	if len(empty.Orders) != 0 {
		t.Fatal("QR holder can read another device's history")
	}
	response = call("/api/v1/public/table-orders/history", map[string]any{"venue_id": venueID, "customer_user_id": customerA}, cookies[0])
	if response.Code != http.StatusBadRequest {
		t.Fatal("client could select a history owner")
	}
	response = call("/api/v1/public/table-orders/history", map[string]any{"venue_id": venueID}, &http.Cookie{Name: "session", Value: "valid-account-session"})
	var accountPage dinein.HistoryPage
	must(json.Unmarshal(response.Body.Bytes(), &accountPage))
	if response.Code != 200 || accountPage.Scope != "account" || len(accountPage.Orders) != 1 || accountPage.Orders[0].ID != accountA.ID {
		t.Fatal("HTTP session did not select its own account history")
	}
	response = call("/api/v1/public/table-orders/history", map[string]any{"venue_id": venueID}, &http.Cookie{Name: "session", Value: "expired-session"})
	if response.Code != 401 {
		t.Fatal("invalid session did not fail closed")
	}
	// Pagination handles equal timestamps without duplicates or skipped orders.
	deviceC := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{5}, 32))
	sum := sha256.Sum256([]byte(deviceC))
	hash := hex.EncodeToString(sum[:])
	_, err = db.Exec(ctx, `INSERT INTO orders(venue_id,venue_table_id,channel,currency,guest_history_hash,created_at)
	 SELECT $1,$2,'dine_in','EUR',$3,'2026-01-01'::timestamptz FROM generate_series(1,35)`, venueID, newTable, hash)
	must(err)
	page = history(dinein.HistoryInput{VenueID: venueID, GuestToken: deviceC})
	if len(page.Orders) != 30 || page.NextCursor == "" {
		t.Fatal("missing history pagination")
	}
	next := history(dinein.HistoryInput{VenueID: venueID, GuestToken: deviceC, Cursor: page.NextCursor})
	if len(next.Orders) != 5 || next.NextCursor != "" {
		t.Fatal("wrong final history page")
	}
	seen := map[string]bool{}
	for _, order := range append(page.Orders, next.Orders...) {
		if seen[order.ID] {
			t.Fatal("duplicate across pages")
		}
		seen[order.ID] = true
	}
}
