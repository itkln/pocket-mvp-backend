package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pocket-mvp-backend/internal/appfault"
)

type favoritesSpy struct{ userID, venueID, action string }

func (s *favoritesSpy) List(_ context.Context, userID string) ([]string, error) {
	s.userID, s.action = userID, "list"
	return []string{"venue-1"}, nil
}
func (s *favoritesSpy) Add(_ context.Context, userID, venueID string) error {
	s.userID, s.venueID, s.action = userID, venueID, "add"
	if venueID == "unpublished" {
		return appfault.ErrNotFound
	}
	return nil
}
func (s *favoritesSpy) Remove(_ context.Context, userID, venueID string) error {
	s.userID, s.venueID, s.action = userID, venueID, "remove"
	return nil
}

func TestSavedVenuesRoutesRequireSessionAndUseAuthenticatedUser(t *testing.T) {
	spy := &favoritesSpy{}
	handler := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Identity: &fakeAuth{}, Favorites: spy, SessionCookie: "pocket_session"})
	call := func(method, path string, authenticated bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		if authenticated {
			r.AddCookie(&http.Cookie{Name: "pocket_session", Value: "token"})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	base := "/api/v1/spots/me/saved-venues"
	if got := call(http.MethodGet, base, false).Code; got != http.StatusUnauthorized {
		t.Fatalf("guest GET: %d", got)
	}
	if got := call(http.MethodPut, base+"/venue-1", false).Code; got != http.StatusUnauthorized {
		t.Fatalf("guest PUT: %d", got)
	}
	if got := call(http.MethodDelete, base+"/venue-1", false).Code; got != http.StatusUnauthorized {
		t.Fatalf("guest DELETE: %d", got)
	}
	listed := call(http.MethodGet, base, true)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"venue_ids":["venue-1"]`) || listed.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("list: %d %s", listed.Code, listed.Body.String())
	}
	if spy.userID != "user-1" {
		t.Fatalf("client controlled user: %q", spy.userID)
	}
	if got := call(http.MethodPut, base+"/venue-1", true).Code; got != http.StatusNoContent || spy.action != "add" || spy.venueID != "venue-1" {
		t.Fatalf("PUT: %d, %#v", got, spy)
	}
	if got := call(http.MethodDelete, base+"/venue-1", true).Code; got != http.StatusNoContent || spy.action != "remove" {
		t.Fatalf("DELETE: %d, %#v", got, spy)
	}
	if got := call(http.MethodPut, base+"/unpublished", true).Code; got != http.StatusNotFound {
		t.Fatalf("unpublished: %d", got)
	}
}
