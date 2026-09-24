package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"pocket-mvp-backend/internal/modules/venues"
)

type venueRouteSpy struct {
	ownerID string
	venueID string
	query   venues.PublicVenueQuery
	menu    []venues.PublicMenuCategory
}

func (s *venueRouteSpy) List(context.Context, string) ([]venues.Venue, error) {
	return nil, nil
}

func (s *venueRouteSpy) SearchPublished(_ context.Context, query venues.PublicVenueQuery) (venues.PublicVenuePage, error) {
	s.query = query
	return venues.PublicVenuePage{Venues: []venues.PublicVenue{}, Page: query.Page, PageSize: query.PageSize}, nil
}
func (s *venueRouteSpy) GetPublished(context.Context, string) (venues.PublicVenue, error) {
	return venues.PublicVenue{}, nil
}
func (s *venueRouteSpy) ListPublishedMenu(_ context.Context, venueID string) ([]venues.PublicMenuCategory, error) {
	s.venueID = venueID
	return s.menu, nil
}

func (s *venueRouteSpy) Create(context.Context, string, venues.Input) (venues.Venue, error) {
	return venues.Venue{}, nil
}

func TestPublicVenueRoutePassesCatalogQueryWithoutAuthentication(t *testing.T) {
	venueService := &venueRouteSpy{}
	handler := New(Dependencies{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Venues: venueService,
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/venues?city=Bratislava&q=coffee&category=restaurants&filter=Open&page=2&page_size=12", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	want := venues.PublicVenueQuery{City: "Bratislava", Search: "coffee", Category: "restaurants", Filter: "Open", Page: 2, PageSize: 12}
	if !reflect.DeepEqual(venueService.query, want) {
		t.Fatalf("unexpected public query: %#v", venueService.query)
	}
}

func TestPublicVenueMenuIsAvailableWithoutAuthentication(t *testing.T) {
	venueService := &venueRouteSpy{menu: []venues.PublicMenuCategory{{ID: "category-1", Name: "Lunch", Items: []venues.PublicMenuItem{{ID: "item-1", Name: "Soup"}}}}}
	handler := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Venues: venueService})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/venues/venue-42/menu", nil))
	if response.Code != http.StatusOK || venueService.venueID != "venue-42" || !strings.Contains(response.Body.String(), "Soup") {
		t.Fatalf("public menu: status=%d venue=%q body=%s", response.Code, venueService.venueID, response.Body.String())
	}
}

func TestPublicVenueRouteRejectsInvalidPagination(t *testing.T) {
	handler := New(Dependencies{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Venues: &venueRouteSpy{},
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/venues?page=zero", nil))
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", response.Code, response.Body.String())
	}
}

func TestPublicVenueRoutePassesStableIDsAndRejectsMalformedIDLists(t *testing.T) {
	venueService := &venueRouteSpy{}
	handler := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Venues: venueService})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/venues?ids=11111111-1111-4111-8111-111111111111,22222222-2222-4222-8222-222222222222", nil))
	if response.Code != http.StatusOK || len(venueService.query.IDs) != 2 {
		t.Fatalf("ID query: %d, %#v", response.Code, venueService.query.IDs)
	}
	for _, path := range []string{"/api/v1/public/venues?ids=", "/api/v1/public/venues?ids=x&ids=y"} {
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("malformed IDs %q: %d", path, response.Code)
		}
	}
}

func (s *venueRouteSpy) Update(context.Context, string, string, venues.Input) (venues.Venue, error) {
	return venues.Venue{}, nil
}

func (s *venueRouteSpy) Delete(_ context.Context, ownerID, venueID string) error {
	s.ownerID = ownerID
	s.venueID = venueID
	return nil
}

func TestChiRoutePassesVenuePathParameter(t *testing.T) {
	venueService := &venueRouteSpy{}
	handler := New(Dependencies{
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		AllowedOrigins: []string{"http://localhost:3000"},
		Identity:       &fakeAuth{},
		Venues:         venueService,
		SessionCookie:  "pocket_session",
	})
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/owner/venues/venue-42", nil)
	request.AddCookie(&http.Cookie{Name: "pocket_session", Value: "active-token"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", response.Code, response.Body.String())
	}
	if venueService.ownerID != "user-1" || venueService.venueID != "venue-42" {
		t.Fatalf("unexpected route parameters: owner=%q venue=%q", venueService.ownerID, venueService.venueID)
	}
}
