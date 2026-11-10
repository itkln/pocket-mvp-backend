package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/httpapi"
	"pocket-mvp-backend/internal/modules/ordering"
	"pocket-mvp-backend/internal/security"
	"slices"
	"strings"
	"testing"
)

func TestOrderQueueIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var owner, venue, otherVenue, table, otherTable string
	must(db.QueryRow(ctx, `INSERT INTO users(email,password_hash,first_name,last_name,account_role) VALUES('queue@test.local','x','Queue','Owner','venue_owner') RETURNING id::text`).Scan(&owner))
	createVenue := func(slug string) string {
		var id string
		must(db.QueryRow(ctx, `INSERT INTO venues(owner_user_id,slug,name,address_line1,city,country_code,status) VALUES($1,$2,'Queue','Main','Bratislava','SK','active') RETURNING id::text`, owner, slug).Scan(&id))
		return id
	}
	venue = createVenue("queue")
	otherVenue = createVenue("other-queue")
	var layout string
	must(db.QueryRow(ctx, `INSERT INTO venue_layouts(venue_id,name) VALUES($1,'Main') RETURNING id::text`, venue).Scan(&layout))
	for i, target := range []*string{&table, &otherTable} {
		must(db.QueryRow(ctx, `INSERT INTO venue_tables(venue_id,layout_id,identifier,seats,qr_token_hash) VALUES($1,$2,$3,4,$3) RETURNING id::text`, venue, layout, fmt.Sprint(i+1)).Scan(target))
	}
	// More than the previous 200-record cap, including identical timestamps for UUID tie-breaking.
	_, err := db.Exec(ctx, `INSERT INTO orders(venue_id,venue_table_id,channel,status,created_at,guest_name,notes,total_minor)
 SELECT $1,$2,CASE WHEN n%2=0 THEN 'preorder' ELSE 'dine_in' END,
 CASE WHEN n%3=0 THEN 'accepted' WHEN n%3=1 THEN 'new' ELSE 'ready' END,
 '2026-01-01T12:00:00Z','PRIVATE GUEST','No salt',1200 FROM generate_series(1,231) n`, venue, table)
	must(err)
	_, err = db.Exec(ctx, `INSERT INTO orders(venue_id,channel,status,created_at)
 SELECT $1,'dine_in',CASE WHEN n%2=0 THEN 'completed' ELSE 'cancelled' END,'2026-02-01T12:00:00Z' FROM generate_series(1,251) n`, venue)
	must(err)
	_, err = db.Exec(ctx, `INSERT INTO orders(venue_id,channel,status) VALUES($1,'dine_in','draft'),($2,'dine_in','new')`, venue, otherVenue)
	must(err)
	_, err = db.Exec(ctx, `INSERT INTO order_items(order_id,item_name,quantity,unit_price_minor,total_minor,notes) SELECT id,'Soup snapshot',2,600,1200,'No pepper' FROM orders WHERE venue_id=$1`, venue)
	must(err)
	repo := ordering.NewPostgresRepository(db)
	svc := ordering.NewService(repo, access.NewVenueAuthorizer(db))
	var activeID string
	for _, scope := range []string{"active", "history"} {
		seen := map[string]bool{}
		cursor := ""
		previous := ""
		for {
			page, err := svc.ListPage(ctx, owner, venue, ordering.Query{Scope: scope, Limit: 37, Cursor: cursor})
			must(err)
			for _, o := range page.Orders {
				if seen[o.ID] || (scope == "active" && previous != "" && o.ID <= previous) || (scope == "history" && previous != "" && o.ID >= previous) {
					t.Fatal("unstable pagination", scope, o.ID)
				}
				if len(o.Items) != 1 || o.Items[0].Quantity != 2 || o.Items[0].TotalMinor != 1200 || o.Items[0].Notes != "No pepper" {
					t.Fatalf("missing snapshots: %+v", o)
				}
				seen[o.ID] = true
				previous = o.ID
				if scope == "active" {
					activeID = o.ID
					if o.TableID != table || o.TableLabel != "1" {
						t.Fatalf("missing table: %+v", o)
					}
				}
			}
			if !page.HasMore {
				if page.NextCursor != "" {
					t.Fatal("terminal cursor")
				}
				break
			}
			if page.NextCursor == "" {
				t.Fatal("missing cursor")
			}
			cursor = page.NextCursor
		}
		want := 231
		if scope == "history" {
			want = 251
		}
		if len(seen) != want {
			t.Fatalf("%s lost orders: %d", scope, len(seen))
		}
	}
	t.Run("history search and local dates", func(t *testing.T) {
		_, err := db.Exec(ctx, `UPDATE venues SET timezone='Europe/Bratislava' WHERE id=$1`, venue)
		must(err)
		var id string
		var number int64
		must(db.QueryRow(ctx, `INSERT INTO orders(venue_id,venue_table_id,channel,status,created_at,total_minor,tip_minor,currency) VALUES($1,$2,'dine_in','completed','2025-12-31T23:30:00Z',1300,100,'EUR') RETURNING id::text,order_number`, venue, table).Scan(&id, &number))
		_, err = db.Exec(ctx, `INSERT INTO order_items(order_id,item_name,quantity,unit_price_minor,total_minor) VALUES($1,'Historical soup',2,600,1200)`, id)
		must(err)
		filter := ordering.Query{Scope: "history", Search: fmt.Sprint(number), Table: "1", DateFrom: "2026-01-01", DateTo: "2026-01-01", Statuses: []string{"completed"}, Channel: "dine_in"}
		found, err := svc.ListPage(ctx, owner, venue, filter)
		must(err)
		if len(found.Orders) != 1 || found.Orders[0].ID != id || found.Orders[0].Items[0].Name != "Historical soup" || found.Orders[0].TipMinor != 100 {
			t.Fatalf("missing historical result: %+v", found)
		}
		protector, err := security.NewDataProtector(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
		must(err)
		first, err := protector.Encrypt("Demo", "users.first_name")
		must(err)
		last, err := protector.Encrypt("Owner", "users.last_name")
		must(err)
		_, err = db.Exec(ctx, `INSERT INTO order_status_events(order_id,actor_id,actor_name,previous_status,new_status) VALUES($1,$2,$3,'served','completed')`, id, owner, first+" "+last)
		must(err)
		detail, err := ordering.NewPostgresRepository(db, protector).Get(ctx, venue, id)
		must(err)
		if len(detail.StatusEvents) != 1 || detail.StatusEvents[0].ActorName != "Demo Owner" {
			t.Fatal("encrypted actor not decoded")
		}
		filter.DateFrom = "2025-12-31"
		filter.DateTo = "2025-12-31"
		found, err = svc.ListPage(ctx, owner, venue, filter)
		must(err)
		if len(found.Orders) != 0 {
			t.Fatal("ignored local midnight")
		}
		for _, q := range []ordering.Query{{Scope: "history", DateFrom: "2026-02-30"}, {Scope: "history", DateFrom: "2026-02-02", DateTo: "2026-02-01"}} {
			if _, err := svc.ListPage(ctx, owner, venue, q); !errors.Is(err, appfault.ErrInvalidInput) {
				t.Fatal("invalid dates accepted", err)
			}
		}
	})
	page, err := svc.ListPage(ctx, owner, venue, ordering.Query{Statuses: []string{"accepted", "new"}, Channel: "dine_in", TableID: table, Limit: 100})
	must(err)
	var expected int
	must(db.QueryRow(ctx, `SELECT count(*) FROM orders WHERE venue_id=$1 AND venue_table_id=$2 AND channel='dine_in' AND status IN ('accepted','new')`, venue, table).Scan(&expected))
	if len(page.Orders) != expected || page.HasMore {
		t.Fatal("combined filters", len(page.Orders), expected)
	}
	empty, err := svc.ListPage(ctx, owner, venue, ordering.Query{TableID: otherTable})
	must(err)
	if empty.Orders == nil || len(empty.Orders) != 0 || empty.HasMore {
		t.Fatal("empty page", empty)
	}
	first, err := svc.ListPage(ctx, owner, venue, ordering.Query{Limit: 1})
	must(err)
	for _, q := range []ordering.Query{{Cursor: "broken"}, {Scope: "all"}, {Limit: 101}, {Limit: -1}, {Channel: "delivery"}, {TableID: "invalid"}, {Statuses: []string{"draft"}}, {Statuses: []string{"completed"}}, {Scope: "history", Cursor: first.NextCursor}, {Channel: "preorder", Cursor: first.NextCursor}} {
		if _, err := svc.ListPage(ctx, owner, venue, q); !errors.Is(err, appfault.ErrInvalidInput) {
			t.Fatalf("accepted invalid query %+v: %v", q, err)
		}
	}
	if _, err := svc.ListPage(ctx, owner, otherVenue, ordering.Query{Cursor: first.NextCursor}); !errors.Is(err, appfault.ErrInvalidInput) {
		t.Fatal("cursor reused for another venue", err)
	}
	// A changed page size is valid; neither filters nor cursor boundaries are offsets.
	next, err := svc.ListPage(ctx, owner, venue, ordering.Query{Limit: 2, Cursor: first.NextCursor})
	must(err)
	if len(next.Orders) != 2 || next.Orders[0].ID == first.Orders[0].ID {
		t.Fatal("bad next page")
	}
	before := db.Stat().AcquireCount()
	_, err = repo.ListPage(ctx, venue, ordering.Query{Limit: 100})
	must(err)
	if count := db.Stat().AcquireCount() - before; count != 2 {
		t.Fatalf("page used %d queries; expected 2, no N+1", count)
	}
	if _, err := svc.Get(ctx, owner, otherVenue, activeID); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatal("cross venue detail", err)
	}
	var draft string
	must(db.QueryRow(ctx, `SELECT id::text FROM orders WHERE venue_id=$1 AND status='draft'`, venue).Scan(&draft))
	if _, err := svc.Get(ctx, owner, venue, draft); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatal("draft exposed", err)
	}
	_, err = db.Exec(ctx, `UPDATE orders SET status='accepted' WHERE id=$1`, activeID)
	must(err)
	actors := map[string]string{"owner": owner}
	for _, role := range []string{"manager", "waiter", "kitchen", "viewer", "outsider"} {
		var id string
		must(db.QueryRow(ctx, `INSERT INTO users(email,password_hash,first_name,last_name) VALUES($1,'x','Staff','Test') RETURNING id::text`, role+"@queue.test").Scan(&id))
		actors[role] = id
		if role != "outsider" {
			_, err = db.Exec(ctx, `INSERT INTO venue_staff(venue_id,user_id,invited_email,role,status,invited_by_user_id) VALUES($1,$2,$3,$4,'active',$5)`, venue, id, role+"@queue.test", role, owner)
			must(err)
		}
	}
	call := func(actor, prefix, path string) *httptest.ResponseRecorder {
		handler := httpapi.New(httpapi.Dependencies{Ordering: svc, Identity: venueHistoryIdentity{userID: actors[actor]}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SessionCookie: "session"})
		req := httptest.NewRequest("GET", "/api/v1/"+prefix+"/venues/"+venue+"/orders"+path, nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: "valid-account-session"})
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		return out
	}
	for _, prefix := range []string{"owner", "staff"} {
		for role := range actors {
			for _, suffix := range []string{"?limit=1", "/" + activeID} {
				out := call(role, prefix, suffix)
				if role == "outsider" {
					if out.Code != 404 {
						t.Fatal(role, out.Code, out.Body)
					}
					continue
				}
				if out.Code != 200 {
					t.Fatal(role, out.Code, out.Body)
				}
				if role == "kitchen" {
					for _, private := range []string{"PRIVATE GUEST", "total_minor", "unit_price_minor", "tip_minor", "guest_phone", "currency", "bill_id"} {
						if strings.Contains(out.Body.String(), private) {
							t.Fatal("kitchen leak", out.Body)
						}
					}
				}
			}
		}
	}
	for _, suffix := range []string{"?limit=0", "?limit=oops", "?limit=101", "?status=wat", "/not-uuid", "?cursor=bad"} {
		if out := call("owner", "owner", suffix); out.Code != 422 {
			t.Fatal(suffix, out.Code, out.Body)
		}
	}
	for role, want := range map[string][]string{"owner": {"preparing", "cancelled"}, "manager": {"preparing", "cancelled"}, "kitchen": {"preparing"}, "waiter": {}, "viewer": {}} {
		order, err := svc.Get(ctx, actors[role], venue, activeID)
		must(err)
		if !slices.Equal(order.AllowedActions, want) || order.UpdatedAt.IsZero() {
			t.Fatalf("%s actions/timestamps %+v", role, order)
		}
	}
	_, err = db.Exec(ctx, `INSERT INTO payments(venue_id,order_id,provider,provider_payment_id,amount_minor,currency) VALUES($1,$2,'manual','queue-payment',1200,'EUR')`, venue, activeID)
	must(err)
	order, err := svc.Get(ctx, owner, venue, activeID)
	must(err)
	if slices.Contains(order.AllowedActions, "cancelled") {
		t.Fatal("paid/pending order advertised cancellation")
	}
	data, err := json.Marshal(order)
	must(err)
	if !strings.Contains(string(data), `"allowed_actions":["preparing"]`) {
		t.Fatal(string(data))
	}
	t.Run("venue order alert settings", func(t *testing.T) {
		defaults, err := svc.GetAlertSettings(ctx, owner, venue)
		must(err)
		if defaults.WarningMinutes != 10 || defaults.CriticalMinutes != 20 || !defaults.CanEdit {
			t.Fatal("bad defaults", defaults)
		}
		input := ordering.AlertSettings{WarningMinutes: 5, CriticalMinutes: 15, WarningColor: "#ffaa00", CriticalColor: "#ff0000", PreorderLeadMinutes: 45}
		_, err = svc.SaveAlertSettings(ctx, owner, venue, input)
		must(err)
		for _, role := range []string{"manager", "kitchen", "waiter", "viewer"} {
			got, err := svc.GetAlertSettings(ctx, actors[role], venue)
			must(err)
			if got.WarningMinutes != 5 || got.PreorderLeadMinutes != 45 || got.CanEdit {
				t.Fatal("settings not shared", role, got)
			}
			if _, err = svc.SaveAlertSettings(ctx, actors[role], venue, input); !errors.Is(err, appfault.ErrForbidden) {
				t.Fatal("staff changed settings", role, err)
			}
		}
		if _, err = svc.GetAlertSettings(ctx, actors["outsider"], venue); !errors.Is(err, appfault.ErrNotFound) {
			t.Fatal("settings leaked", err)
		}
		untouched, err := svc.GetAlertSettings(ctx, owner, otherVenue)
		must(err)
		if untouched.WarningMinutes != 10 {
			t.Fatal("cross venue write")
		}
		for _, bad := range []ordering.AlertSettings{{WarningMinutes: 0, CriticalMinutes: 20, WarningColor: "#ffffff", CriticalColor: "#000000"}, {WarningMinutes: 20, CriticalMinutes: 10, WarningColor: "#ffffff", CriticalColor: "#000000"}, {WarningMinutes: 1, CriticalMinutes: 2, WarningColor: "red", CriticalColor: "#000000"}} {
			if _, err = svc.SaveAlertSettings(ctx, owner, venue, bad); !errors.Is(err, appfault.ErrInvalidInput) {
				t.Fatal("invalid settings accepted", err)
			}
		}
	})
	for _, state := range []string{"inactive", "removed"} {
		if state == "inactive" {
			_, err = db.Exec(ctx, `UPDATE venue_staff SET status='inactive' WHERE user_id=$1`, actors["kitchen"])
		} else {
			_, err = db.Exec(ctx, `UPDATE venue_staff SET status='active',removed_at=now() WHERE user_id=$1`, actors["kitchen"])
		}
		must(err)
		if out := call("kitchen", "staff", "/"+activeID); out.Code != 404 {
			t.Fatal("inactive/removed staff detail", out.Code)
		}
		if out := call("kitchen", "staff", ""); out.Code != 404 {
			t.Fatal("inactive/removed staff list", out.Code)
		}
	}
}
