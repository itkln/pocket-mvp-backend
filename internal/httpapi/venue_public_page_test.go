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
	"pocket-mvp-backend/internal/modules/venues"
)

type publicPageVenues struct {
	VenueService
	user string
}

// Get stands in for the venue capability check: only own-venue may be edited by this user.
func (s *publicPageVenues) Get(_ context.Context, user, venueID string) (venues.Venue, error) {
	s.user = user
	if venueID != "own-venue" {
		return venues.Venue{}, appfault.ErrNotFound
	}
	return venues.Venue{ID: "own-venue", Status: "draft"}, nil
}
func TestOwnerPublicPageScopesVenueAndKeepsDraftHidden(t *testing.T) {
	repository := &publicPageVenues{}
	handler := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Identity: &fakeAuth{}, Venues: repository, SessionCookie: "session", DishBaseURL: "https://dish.example/"})
	for _, tc := range []struct {
		path    string
		session bool
		status  int
	}{{"own-venue", false, 401}, {"someone-else", true, 404}, {"own-venue", true, 200}} {
		request := httptest.NewRequest("GET", "/api/v1/venues/"+tc.path+"/public-page?locale=sk", nil)
		if tc.session {
			request.AddCookie(&http.Cookie{Name: "session", Value: "secret"})
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("%s status %d: %s", tc.path, response.Code, response.Body.String())
		}
		if tc.status == 200 && (!strings.Contains(response.Body.String(), "https://dish.example/sk/r/own-venue") || !strings.Contains(response.Body.String(), `"published":false`)) {
			t.Fatal(response.Body.String())
		}
	}
	if repository.user != "user-1" {
		t.Fatal("user identity not enforced")
	}
}
