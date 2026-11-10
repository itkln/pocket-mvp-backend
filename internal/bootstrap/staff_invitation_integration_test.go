package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"pocket-mvp-backend/internal/config"
	"pocket-mvp-backend/internal/security"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestStaffInvitationIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	smtpAddress, messages := resetTestSMTP(t)
	cfg := config.Config{DataEncryptionKey: []byte("0123456789abcdef0123456789abcdef"), DataLookupKey: []byte("fedcba9876543210fedcba9876543210"),
		SessionTTL: time.Hour, SessionCookieName: "pocket_session", AppBaseURL: "https://vendor.example", SpotsAppBaseURL: "https://dish.example",
		PasswordResetTTL: time.Hour, SMTPAddress: smtpAddress, SMTPFrom: "no-reply@pocket.test"}
	handler, err := NewHTTPHandler(db, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Host = "evil.example"
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	want := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("want %d got %d %s", status, w.Code, w.Body.String())
		}
	}
	register := func(email string) (string, *http.Cookie) {
		w := call("POST", "/auth/register", fmt.Sprintf(`{"first_name":"Invite","last_name":"Test","email":%q,"password":"test invitation password"}`, email), nil)
		want(w, 201)
		var p struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p.User.ID, w.Result().Cookies()[0]
	}
	ownerID, owner := register("invite-owner@example.com")
	staffID, staff := register("invite-kitchen@example.com")
	_, other := register("invite-other@example.com")
	var venue string
	err = db.QueryRow(ctx, `INSERT INTO venues(owner_user_id,name,slug,address_line1,city,country_code,currency,timezone,status) VALUES($1,'Invite Venue','invite-test','Test 1','Bratislava','SK','EUR','Europe/Bratislava','active') RETURNING id::text`, ownerID).Scan(&venue)
	if err != nil {
		t.Fatal(err)
	}
	base := "/owner/venues/" + venue + "/staff"
	create := func() *httptest.ResponseRecorder {
		return call("POST", base, `{"display_name":"Kitchen","email":"invite-kitchen@example.com","role":"kitchen"}`, owner)
	}
	w := create()
	want(w, 201)
	var created struct {
		Member struct {
			ID       string `json:"id"`
			Delivery string `json:"invite_delivery"`
		} `json:"staff_member"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Member.Delivery != "sent" {
		t.Fatal("invitation not sent")
	}
	member := created.Member.ID
	receive := func() string {
		t.Helper()
		select {
		case message := <-messages:
			raw := regexp.MustCompile(`https://vendor\.example/ru/invite\?token=[A-Za-z0-9_-]+`).FindString(message)
			link, e := url.Parse(raw)
			if e != nil || raw == "" {
				t.Fatal("missing invitation URL")
			}
			return link.Query().Get("token")
		case <-time.After(3 * time.Second):
			t.Fatal("no invitation email")
			return ""
		}
	}
	token := receive()
	var hash string
	var hours float64
	err = db.QueryRow(ctx, `SELECT invite_token_hash,extract(epoch FROM invite_expires_at-invite_sent_at)/3600 FROM venue_staff WHERE id=$1`, member).Scan(&hash, &hours)
	if err != nil || hash != security.HashSessionToken(token) || hours != 72 {
		t.Fatalf("token storage/expiry: %v", err)
	}
	payload := func(token string) string { return fmt.Sprintf(`{"token":%q}`, token) }
	inspect := "/staff/invitations/inspect"
	accept := "/staff/invitations/accept"
	want(call("POST", inspect, payload(token), nil), 401)
	want(call("POST", inspect, payload(token), other), 403)
	want(call("POST", accept, payload(token), other), 403)
	want(call("POST", base+"/"+member+"/invite", "", other), 404)
	want(call("POST", base+"/"+member+"/invite", "", owner), 409)
	want(call("PATCH", base+"/"+member, `{"role":"kitchen","status":"active"}`, owner), 404)
	w = call("GET", "/staff/venues", "", staff)
	want(w, 200)
	if strings.Contains(w.Body.String(), venue) {
		t.Fatal("unaccepted invitation grants access")
	}
	// Resending invalidates the old token.
	_, err = db.Exec(ctx, `UPDATE venue_staff SET invite_sent_at=now()-interval '2 minutes' WHERE id=$1`, member)
	if err != nil {
		t.Fatal(err)
	}
	want(call("POST", base+"/"+member+"/invite", "", owner), 204)
	replacement := receive()
	want(call("POST", accept, payload(token), staff), 404)
	want(call("POST", inspect, payload(replacement), staff), 200)
	// Exactly one acceptance succeeds even when two requests race.
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- call("POST", accept, payload(replacement), staff).Code }()
	}
	counts := map[int]int{}
	for i := 0; i < 2; i++ {
		counts[<-results]++
	}
	if counts[200] != 1 || counts[404] != 1 {
		t.Fatalf("accept race: %v", counts)
	}
	var linked, role, status string
	err = db.QueryRow(ctx, `SELECT user_id::text,role,status FROM venue_staff WHERE id=$1`, member).Scan(&linked, &role, &status)
	if err != nil || linked != staffID || role != "kitchen" || status != "active" {
		t.Fatal("membership was not activated correctly", err)
	}
	w = call("GET", "/auth/me", "", staff)
	want(w, 200)
	if !strings.Contains(w.Body.String(), `"staff"`) || strings.Contains(w.Body.String(), `"owner"`) {
		t.Fatal("incorrect capabilities", w.Body.String())
	}
	w = call("GET", "/staff/venues", "", staff)
	want(w, 200)
	if !strings.Contains(w.Body.String(), venue) {
		t.Fatal("assigned venue missing")
	}
	want(call("GET", "/owner/venues/"+venue+"/staff", "", staff), 404)
	want(call("GET", "/staff/venues/"+venue+"/orders", "", staff), 200)
	want(call("POST", accept, payload(replacement), staff), 404)
	// Removing membership immediately revokes permissions in an existing session.
	want(call("DELETE", base+"/"+member, "", owner), 204)
	want(call("GET", "/staff/venues/"+venue+"/orders", "", staff), 404)
	w = call("GET", "/staff/venues", "", staff)
	want(w, 200)
	if strings.Contains(w.Body.String(), venue) {
		t.Fatal("removed venue still exposed")
	}
	// Expiration and revocation of a pending invitation.
	w = create()
	want(w, 201)
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	member = created.Member.ID
	token = receive()
	_, err = db.Exec(ctx, `UPDATE venue_staff SET invite_expires_at=now()-interval '1 second' WHERE id=$1`, member)
	if err != nil {
		t.Fatal(err)
	}
	want(call("POST", accept, payload(token), staff), 404)
	want(call("DELETE", base+"/"+member, "", owner), 204)
	want(call("POST", accept, payload(token), staff), 404)
	// SMTP failure preserves the invited row for explicit retry, never grants access.
	cfg.SMTPAddress = "invalid-address"
	handler, err = NewHTTPHandler(db, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	w = create()
	want(w, 201)
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Member.Delivery != "failed" {
		t.Fatal("delivery failure hidden")
	}
	var tokens int
	err = db.QueryRow(ctx, `SELECT count(*) FROM venue_staff WHERE id=$1 AND invite_token_hash IS NOT NULL`, created.Member.ID).Scan(&tokens)
	if err != nil || tokens != 0 {
		t.Fatal("failed delivery leaves valid token")
	}
}
