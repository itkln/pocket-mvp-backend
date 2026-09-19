package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"pocket-mvp-backend/internal/modules/venues"
)

type venueRouteSpy struct {
	ownerID string
	venueID string
	query   venues.PublicVenueQuery
}

func (s *venueRouteSpy) List(context.Context, string) ([]venues.Venue, error) {
	return nil, nil
}

func (s *venueRouteSpy) SearchPublished(_ context.Context, query venues.PublicVenueQuery) (venues.PublicVenuePage, error) {
	s.query = query
	return venues.PublicVenuePage{Venues: []venues.PublicVenue{}, Page: query.Page, PageSize: query.PageSize}, nil
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
	if venueService.query != want {
		t.Fatalf("unexpected public query: %#v", venueService.query)
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
