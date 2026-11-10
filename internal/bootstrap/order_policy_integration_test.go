package bootstrap

import (
	"context"
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
	"pocket-mvp-backend/internal/modules/venues"
	"strings"
	"testing"
	"time"
)

func TestOrderPolicyIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	user := func(name string) string {
		var id string
		must(db.QueryRow(ctx, `INSERT INTO users(email,password_hash,first_name,last_name,account_role) VALUES($1,'test','Test','User','venue_owner') RETURNING id::text`, name+"@policy.test").Scan(&id))
		return id
	}
	owner := user("owner")
	var venue string
	must(db.QueryRow(ctx, `INSERT INTO venues(owner_user_id,slug,name,address_line1,city,country_code,timezone,currency,status) VALUES($1,'policy','Policy','Main','Bratislava','SK','Europe/Bratislava','EUR','active') RETURNING id::text`, owner).Scan(&venue))
	actors := map[string]string{"owner": owner, "outsider": user("outsider")}
	for _, role := range []string{"manager", "waiter"} {
		id := user(role)
		actors[role] = id
		_, err := db.Exec(ctx, `INSERT INTO venue_staff(venue_id,user_id,invited_email,role,status,invited_by_user_id) VALUES($1,$2,$3,$4,'active',$5)`, venue, id, role+"@policy.test", role, owner)
		must(err)
	}
	// The kitchen and viewer roles no longer exist (V-SRV-01).
	for _, role := range []string{"kitchen", "viewer"} {
		if _, err := db.Exec(ctx, `INSERT INTO venue_staff(venue_id,invited_email,role,status,invited_by_user_id) VALUES($1,$2,$3,'invited',$4)`, venue, role+"-removed@policy.test", role, owner); err == nil {
			t.Fatalf("%s role still accepted", role)
		}
	}
	workplaces := venues.NewPostgresRepository(db)
	for role, actor := range actors {
		listed, err := workplaces.ListWorkplaces(ctx, actor)
		must(err)
		if role == "outsider" {
			if len(listed) != 0 {
				t.Fatal("outsider sees workplace")
			}
			continue
		}
		if len(listed) != 1 || listed[0].ID != venue || listed[0].AccessRole != role || len(listed[0].Settings) != 0 || listed[0].Email != "" {
			t.Fatalf("workplace %s: %+v", role, listed)
		}
	}
	repo := ordering.NewPostgresRepository(db)
	svc := ordering.NewService(repo, access.NewVenueAuthorizer(db))
	create := func() string {
		var id string
		must(db.QueryRow(ctx, `INSERT INTO orders(venue_id,channel,guest_name,total_minor) VALUES($1,'dine_in','PRIVATE',1200) RETURNING id::text`, venue).Scan(&id))
		_, err := db.Exec(ctx, `INSERT INTO order_items(order_id,item_name,quantity,unit_price_minor,total_minor) VALUES($1,'Soup',1,1200,1200)`, id)
		must(err)
		return id
	}
	id := create()
	var bill string
	must(db.QueryRow(ctx, `INSERT INTO bills(venue_id,subtotal_minor,total_minor) VALUES($1,1200,1200) RETURNING id::text`, venue).Scan(&bill))
	_, err := db.Exec(ctx, `UPDATE orders SET bill_id=$1 WHERE id=$2`, bill, id)
	must(err)
	if _, err := svc.List(ctx, actors["outsider"], venue); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := svc.UpdateStatus(ctx, owner, venue, id, "served", "new"); !errors.Is(err, appfault.ErrConflict) {
		t.Fatal(err)
	}
	if _, err := svc.UpdateStatus(ctx, owner, venue, id, "accepted", "new"); !errors.Is(err, appfault.ErrInvalidInput) {
		t.Fatal("removed accepted status still settable", err)
	}
	_, err = svc.UpdateStatus(ctx, actors["waiter"], venue, id, "preparing", "new")
	must(err)
	var conflict *ordering.StatusConflict
	_, err = svc.UpdateStatus(ctx, owner, venue, id, "cancelled", "new", ordering.StatusChange{Reason: "guest_request"})
	if !errors.As(err, &conflict) || conflict.CurrentStatus != "preparing" {
		t.Fatal(err)
	}
	for _, step := range []struct{ role, from, to string }{{"waiter", "preparing", "served"}, {"manager", "served", "completed"}} {
		_, err = svc.UpdateStatus(ctx, actors[step.role], venue, id, step.to, step.from)
		must(err)
		var billStatus string
		var subtotal, total, payments int
		var closed *time.Time
		must(db.QueryRow(ctx, `SELECT status,subtotal_minor,total_minor,closed_at,(SELECT count(*) FROM payments WHERE bill_id=$1) FROM bills WHERE id=$1`, bill).Scan(&billStatus, &subtotal, &total, &closed, &payments))
		if billStatus != "open" || subtotal != 1200 || total != 1200 || closed != nil || payments != 0 {
			t.Fatalf("service action changed bill: %s %d %d %v %d", billStatus, subtotal, total, closed, payments)
		}
	}
	var before, after time.Time
	must(db.QueryRow(ctx, `SELECT updated_at FROM orders WHERE id=$1`, id).Scan(&before))
	_, err = svc.UpdateStatus(ctx, owner, venue, id, "completed", "served")
	must(err)
	must(db.QueryRow(ctx, `SELECT updated_at FROM orders WHERE id=$1`, id).Scan(&after))
	if !before.Equal(after) {
		t.Fatal("replay wrote order")
	}
	if _, err = svc.UpdateStatus(ctx, owner, venue, id, "cancelled", "completed", ordering.StatusChange{Reason: "guest_request"}); !errors.Is(err, appfault.ErrConflict) {
		t.Fatal(err)
	}
	t.Run("HTTP contract", func(t *testing.T) {
		orderID := create()
		call := func(role, method, body string) *httptest.ResponseRecorder {
			handler := httpapi.New(httpapi.Dependencies{Ordering: svc, Identity: venueHistoryIdentity{userID: actors[role]}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SessionCookie: "session"})
			path := "/api/v1/venues/" + venue + "/orders"
			if method == "PATCH" {
				path += "/" + orderID
			}
			req := httptest.NewRequest(method, path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: "session", Value: "valid-account-session"})
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			return out
		}
		for _, tc := range []struct {
			role, body string
			code       int
		}{
			{"owner", `{"status":"preparing"}`, 422},
			{"owner", `{"status":"ready","expected_status":"new"}`, 422},
			{"outsider", `{"status":"preparing","expected_status":"new"}`, 404},
			{"owner", `{"status":"served","expected_status":"new"}`, 409},
		} {
			out := call(tc.role, "PATCH", tc.body)
			if out.Code != tc.code {
				t.Fatalf("HTTP %s: %d %s", tc.role, out.Code, out.Body)
			}
			if tc.code == 409 && (!strings.Contains(out.Body.String(), `"current_status":"new"`) || !strings.Contains(out.Body.String(), `"code":"order_status_conflict"`)) {
				t.Fatal(out.Body)
			}
		}
		out := call("waiter", "GET", "")
		if out.Code != 200 || !strings.Contains(out.Body.String(), "total_minor") {
			t.Fatal(out.Code, out.Body)
		}
	})

	t.Run("cancellation reasons and atomic audit", func(t *testing.T) {
		oid := create()
		for _, c := range []ordering.StatusChange{{}, {Reason: "invalid"}, {Reason: "other", Comment: strings.Repeat("я", 501)}} {
			if _, err := svc.UpdateStatus(ctx, owner, venue, oid, "cancelled", "new", c); !errors.Is(err, appfault.ErrInvalidInput) {
				t.Fatalf("invalid reason accepted: %v", err)
			}
		}
		change := ordering.StatusChange{Reason: "guest_request", Comment: "Гость передумал"}
		if _, err := svc.UpdateStatus(ctx, actors["outsider"], venue, oid, "cancelled", "new", change); !errors.Is(err, appfault.ErrNotFound) {
			t.Fatal(err)
		}
		// Waiters cancel orders themselves (V-SRV-01).
		if _, err := svc.UpdateStatus(ctx, actors["waiter"], venue, create(), "cancelled", "new", change); err != nil {
			t.Fatalf("waiter cancellation: %v", err)
		}
		var bid string
		must(db.QueryRow(ctx, `INSERT INTO bills(venue_id,subtotal_minor,tip_minor,total_minor) VALUES($1,1200,100,1300) RETURNING id::text`, venue).Scan(&bid))
		_, err := db.Exec(ctx, `UPDATE orders SET bill_id=$1,subtotal_minor=1200,tip_minor=100,total_minor=1300 WHERE id=$2`, bid, oid)
		must(err)
		start := make(chan struct{})
		results := make(chan error, 2)
		for n := 0; n < 2; n++ {
			go func() {
				<-start
				_, err := svc.UpdateStatus(ctx, owner, venue, oid, "cancelled", "new", change)
				results <- err
			}()
		}
		close(start)
		must(<-results)
		must(<-results)
		var total int
		must(db.QueryRow(ctx, `SELECT total_minor FROM bills WHERE id=$1`, bid).Scan(&total))
		if total != 0 {
			t.Fatal(total)
		}
		detail, err := svc.Get(ctx, owner, venue, oid)
		must(err)
		if len(detail.StatusEvents) != 1 || detail.StatusEvents[0].ActorID != owner || detail.StatusEvents[0].Reason != "guest_request" || detail.StatusEvents[0].Comment != "Гость передумал" || detail.StatusEvents[0].PreviousStatus != "new" {
			t.Fatalf("audit: %+v", detail.StatusEvents)
		}
		if _, err := svc.Get(ctx, actors["outsider"], venue, oid); !errors.Is(err, appfault.ErrNotFound) {
			t.Fatal(err)
		}
		protected := create()
		_, err = db.Exec(ctx, `UPDATE orders SET bill_id=$1 WHERE id=$2`, bill, protected)
		must(err)
		_, err = db.Exec(ctx, `UPDATE bills SET status='paid' WHERE id=$1`, bill)
		must(err)
		if _, err := svc.UpdateStatus(ctx, owner, venue, protected, "cancelled", "new", change); !errors.Is(err, appfault.ErrConflict) {
			t.Fatalf("closed bill: %v", err)
		}
		protectedDetail, err := svc.Get(ctx, owner, venue, protected)
		must(err)
		if len(protectedDetail.StatusEvents) != 0 || protectedDetail.CancellationBlocked == "" {
			t.Fatalf("blocked audit %+v", protectedDetail)
		}
	})
	for i := 0; i < 5; i++ {
		id = create()
		start := make(chan struct{})
		results := make(chan error, 2)
		for _, status := range []string{"preparing", "cancelled"} {
			go func(status string) {
				<-start
				_, err := svc.UpdateStatus(ctx, owner, venue, id, status, "new", ordering.StatusChange{Reason: map[string]string{"cancelled": "guest_request"}[status]})
				results <- err
			}(status)
		}
		close(start)
		a, b := <-results, <-results
		if (a == nil) == (b == nil) {
			t.Fatalf("race results %v / %v", a, b)
		}
		if a != nil && !errors.Is(a, appfault.ErrConflict) || b != nil && !errors.Is(b, appfault.ErrConflict) {
			t.Fatal(fmt.Sprint(a, b))
		}
	}
	_, err = db.Exec(ctx, `UPDATE venue_staff SET status='inactive' WHERE user_id=$1`, actors["waiter"])
	must(err)
	listed, err := workplaces.ListWorkplaces(ctx, actors["waiter"])
	must(err)
	if len(listed) != 0 {
		t.Fatal("inactive membership visible")
	}
	_, err = db.Exec(ctx, `UPDATE venue_staff SET status='active',removed_at=now() WHERE user_id=$1`, actors["waiter"])
	must(err)
	listed, err = workplaces.ListWorkplaces(ctx, actors["waiter"])
	must(err)
	if len(listed) != 0 {
		t.Fatal("removed membership visible")
	}
	caps, err := access.NewCapabilityReader(db).ListCapabilities(ctx, actors["waiter"])
	must(err)
	for _, capability := range caps {
		if capability == "staff" {
			t.Fatal("removed membership has staff capability")
		}
	}
	if _, err = svc.List(ctx, actors["waiter"], venue); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatal("inactive staff allowed", err)
	}
}
