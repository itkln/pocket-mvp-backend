package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// Routes a guest may call without a session. Everything else must be behind requireUser.
var guestRoutes = map[string]bool{
	"GET /health": true, "GET /healthz": true, "GET /readyz": true, "GET /api/v1": true,
	"POST /api/v1/auth/register": true, "POST /api/v1/auth/login": true, "POST /api/v1/auth/logout": true,
	"POST /api/v1/auth/password-reset/request": true, "POST /api/v1/auth/password-reset/confirm": true,
}

func TestEveryNonPublicRouteNeedsASession(t *testing.T) {
	handler := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Identity: &fakeAuth{}, SessionCookie: "session"})
	routes, ok := handler.(chi.Routes)
	if !ok {
		t.Fatal("the API router does not expose its routes")
	}
	params := regexp.MustCompile(`\{[^}]+\}`)
	checked := 0
	err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.TrimSuffix(strings.ReplaceAll(route, "/*", ""), "/")
		if route == "" {
			route = "/"
		}
		if guestRoutes[method+" "+route] || strings.HasPrefix(route, "/api/v1/public/") {
			return nil
		}
		checked++
		path := params.ReplaceAllString(route, "id-1")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader("{}")))
		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without a session: %d", method, route, response.Code)
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s %s may be cached", method, route)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 40 {
		t.Fatalf("only %d private routes checked: the walk missed route groups", checked)
	}
}

// The venue workspace carries no role in its URL: owner and staff screens share the routes.
func TestVenueWorkspaceRoutesReplaceOwnerAndStaffCopies(t *testing.T) {
	handler := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Identity: &fakeAuth{}, SessionCookie: "session"})
	for _, path := range []string{
		"/api/v1/owner/venues/v-1/menu-items", "/api/v1/owner/venues/v-1/floor-plan",
		"/api/v1/owner/venues/v-1/orders", "/api/v1/staff/venues/v-1/orders",
		"/api/v1/staff/venues/v-1/order-alert-settings", "/api/v1/owner/venues/v-1/staff",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.AddCookie(&http.Cookie{Name: "session", Value: "secret"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound && response.Code != http.StatusMethodNotAllowed {
			t.Errorf("old route %s still answers %d", path, response.Code)
		}
	}
}
