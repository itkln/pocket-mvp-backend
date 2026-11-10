package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/config"
	"pocket-mvp-backend/internal/security"
)

// Opt-in: uses a fresh schema in a disposable database whose name ends in _test.
// Exercises the real HTTP router, services, migrations, PostgreSQL and SMTP sender.
func TestPasswordRecoveryIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	smtpAddress, messages := resetTestSMTP(t)
	cfg := config.Config{
		DataEncryptionKey: []byte("0123456789abcdef0123456789abcdef"),
		DataLookupKey:     []byte("fedcba9876543210fedcba9876543210"),
		SessionTTL:        time.Hour, SessionCookieName: "pocket_session",
		AppBaseURL: "https://vendor.example", DishAppBaseURL: "https://dish.example",
		PasswordResetTTL: 30 * time.Minute, SMTPAddress: smtpAddress, SMTPFrom: "no-reply@pocket.test",
		AllowedOrigins: []string{"https://vendor.example", "https://dish.example"},
	}
	handler, err := NewHTTPHandler(db, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1/auth"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		// Neither spoofed Host nor forwarded host may affect the e-mail destination.
		r.Host = "evil.example"
		r.Header.Set("X-Forwarded-Host", "evil.example")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	wantStatus := func(r *httptest.ResponseRecorder, status int) {
		t.Helper()
		if r.Code != status {
			t.Fatalf("HTTP status=%d, want=%d: %s", r.Code, status, r.Body.String())
		}
	}
	registration := call("POST", "/register", `{"first_name":"Reset","last_name":"Guest","email":"guest@example.com","password":"initial secure password"}`, nil)
	wantStatus(registration, http.StatusCreated)
	firstSession := registration.Result().Cookies()[0]
	login := call("POST", "/login", `{"email":"guest@example.com","password":"initial secure password"}`, nil)
	wantStatus(login, http.StatusOK)
	secondSession := login.Result().Cookies()[0]

	request := call("POST", "/password-reset/request", `{"email":"guest@example.com","locale":"sk","product":"dish"}`, nil)
	wantStatus(request, http.StatusAccepted)
	link := resetMessageURL(t, messages)
	if link.Scheme != "https" || link.Host != "dish.example" || link.Path != "/sk/reset-password" {
		t.Fatalf("wrong Spots destination: %s%s", link.Host, link.Path)
	}
	token := link.Query().Get("token")
	if len(token) < 32 {
		t.Fatal("missing reset token")
	}
	var storedHash string
	var ttlSeconds float64
	if err := db.QueryRow(context.Background(), `SELECT token_hash, extract(epoch FROM expires_at-created_at) FROM password_reset_tokens WHERE token_hash=$1`, security.HashSessionToken(token)).Scan(&storedHash, &ttlSeconds); err != nil {
		t.Fatal(err)
	}
	if storedHash == token || ttlSeconds < 1790 || ttlSeconds > 1810 {
		t.Fatal("token storage/TTL mismatch")
	}

	confirmation, _ := json.Marshal(map[string]string{"token": token, "password": "replacement secure password"})
	results := make(chan *httptest.ResponseRecorder, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- call("POST", "/password-reset/confirm", string(confirmation), nil) }()
	}
	statuses := map[int]int{}
	for i := 0; i < 2; i++ {
		statuses[(<-results).Code]++
	}
	if statuses[http.StatusNoContent] != 1 || statuses[http.StatusUnprocessableEntity] != 1 {
		t.Fatalf("concurrent token consumption: %v", statuses)
	}
	wantStatus(call("POST", "/password-reset/confirm", string(confirmation), nil), http.StatusUnprocessableEntity)
	wantStatus(call("GET", "/me", "", firstSession), http.StatusUnauthorized)
	wantStatus(call("GET", "/me", "", secondSession), http.StatusUnauthorized)
	wantStatus(call("POST", "/login", `{"email":"guest@example.com","password":"initial secure password"}`, nil), http.StatusUnauthorized)
	wantStatus(call("POST", "/login", `{"email":"guest@example.com","password":"replacement secure password"}`, nil), http.StatusOK)

	// Backward-compatible Vendor destination and expiration checked against real SQL.
	wantStatus(call("POST", "/password-reset/request", `{"email":"guest@example.com","locale":"en"}`, nil), http.StatusAccepted)
	vendorLink := resetMessageURL(t, messages)
	if vendorLink.Host != "vendor.example" || vendorLink.Path != "/en/reset-password" {
		t.Fatal("legacy request did not return to Vendor")
	}
	expiredToken := vendorLink.Query().Get("token")
	if _, err := db.Exec(context.Background(), `UPDATE password_reset_tokens SET created_at=now()-interval '1 hour', expires_at=now()-interval '1 minute' WHERE token_hash=$1`, security.HashSessionToken(expiredToken)); err != nil {
		t.Fatal(err)
	}
	expiredBody, _ := json.Marshal(map[string]string{"token": expiredToken, "password": "expired link must not work"})
	wantStatus(call("POST", "/password-reset/confirm", string(expiredBody), nil), http.StatusUnprocessableEntity)
	wantStatus(call("POST", "/login", `{"email":"guest@example.com","password":"replacement secure password"}`, nil), http.StatusOK)

	unknown := call("POST", "/password-reset/request", `{"email":"missing@example.com","product":"dish"}`, nil)
	wantStatus(unknown, http.StatusAccepted)
	if unknown.Body.String() != request.Body.String() {
		t.Fatal("response reveals account existence")
	}
	wantStatus(call("POST", "/password-reset/request", `{"email":"guest@example.com","product":"https://evil.example"}`, nil), http.StatusUnprocessableEntity)
	select {
	case <-messages:
		t.Fatal("unexpected e-mail for unknown account/invalid product")
	default:
	}
}

func resetTestDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to an isolated PostgreSQL database ending in _test")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid TEST_DATABASE_URL")
	}
	if !strings.HasSuffix(config.ConnConfig.Database, "_test") {
		t.Fatal("refusing integration test against a database without the _test suffix")
	}
	ctx := context.Background()
	admin, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	// Extensions remain in public; all application objects live in our own schema.
	if _, err := admin.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public; CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("e03_reset_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Errorf("clean test schema: %v", err)
		}
	})
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	paths, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil || len(paths) == 0 {
		t.Fatal("migrations not found")
	}
	for _, path := range paths {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(path), err)
		}
	}
	return pool
}

// Minimal local SMTP capture server; the production net/smtp sender talks over TCP.
func resetTestSMTP(t *testing.T) (string, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	messages := make(chan string, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				client := textproto.NewConn(conn)
				_ = client.PrintfLine("220 localhost ESMTP")
				for {
					line, err := client.ReadLine()
					if err != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"):
						_ = client.PrintfLine("250-localhost\r\n250 8BITMIME")
					case line == "DATA":
						_ = client.PrintfLine("354 send message")
						data, err := io.ReadAll(client.DotReader())
						if err != nil {
							return
						}
						messages <- string(data)
						_ = client.PrintfLine("250 accepted")
					case line == "QUIT":
						_ = client.PrintfLine("221 bye")
						return
					default:
						_ = client.PrintfLine("250 OK")
					}
				}
			}()
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); <-done })
	return listener.Addr().String(), messages
}

func resetMessageURL(t *testing.T, messages <-chan string) *url.URL {
	t.Helper()
	select {
	case raw := <-messages:
		message, err := mail.ReadMessage(strings.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if message.Header.Get("To") != "guest@example.com" || message.Header.Get("From") != "Pocket <no-reply@pocket.test>" {
			t.Fatal("unexpected SMTP envelope headers")
		}
		body, err := io.ReadAll(message.Body)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range strings.Fields(string(body)) {
			if strings.HasPrefix(field, "https://") {
				link, err := url.Parse(field)
				if err != nil {
					t.Fatal(err)
				}
				return link
			}
		}
		t.Fatal("SMTP message did not include a reset link")
	case <-time.After(5 * time.Second):
		t.Fatal("SMTP message not received")
	}
	return nil
}
