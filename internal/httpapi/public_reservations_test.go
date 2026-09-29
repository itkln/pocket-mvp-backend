package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pocket-mvp-backend/internal/modules/identity"
	"pocket-mvp-backend/internal/modules/reservations"
)

type reservationIdentity struct{ fakeAuth }

func (*reservationIdentity) Authenticate(context.Context, string) (identity.User, error) {
	return identity.User{
		ID: "10000000-0000-4000-8000-000000000001", FirstName: "Account", LastName: "Guest",
		Email: "account@example.test", Phone: "+421900123456", Role: "customer",
	}, nil
}

type reservationRouteSpy struct {
	input      reservations.CreateInput
	confirmErr error
}

func (s *reservationRouteSpy) Create(_ context.Context, _ string, input reservations.CreateInput) (reservations.Result, error) {
	s.input = input
	return reservations.Result{ID: "booking-1", Code: "BOOK123", StartsAt: time.Now(), GuestCount: input.Guests}, nil
}

func (s *reservationRouteSpy) Hold(_ context.Context, _ string, input reservations.CreateInput) (reservations.HoldResult, error) {
	s.input = input
	return reservations.HoldResult{ID: "10000000-0000-4000-8000-000000000001", ExpiresAt: time.Now().Add(10 * time.Minute)}, nil
}

func (s *reservationRouteSpy) Confirm(_ context.Context, _, _ string, input reservations.CreateInput) (reservations.Result, error) {
	if s.confirmErr != nil {
		return reservations.Result{}, s.confirmErr
	}
	return s.Create(context.Background(), "", input)
}

func (s *reservationRouteSpy) GetConfirmed(context.Context, string, string, string) (reservations.Result, error) {
	return reservations.Result{ID: "10000000-0000-4000-8000-000000000001", Code: "BOOK123"}, nil
}

func TestPublicReservationHoldConfirmAndReloadRoutes(t *testing.T) {
	service := &reservationRouteSpy{}
	handler := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Reservations: service, SessionCookie: "pocket_session"})
	body := `{"starts_at":"2026-10-01T16:00:00Z","guests":2,"guest_name":"Guest Name","guest_phone":"+421900123456","idempotency_key":"hold-route-test-123"}`
	for _, route := range []struct{ path, expected string }{
		{"/api/v1/public/venues/10000000-0000-4000-8000-000000000001/reservations/holds", "10000000-0000-4000-8000-000000000001"},
		{"/api/v1/public/venues/10000000-0000-4000-8000-000000000001/reservations/10000000-0000-4000-8000-000000000001/confirm", "BOOK123"},
	} {
		request := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), route.expected) {
			t.Fatalf("%s: %d %s", route.path, response.Code, response.Body.String())
		}
	}
	if service.input.IdempotencyKey != "hold-route-test-123" {
		t.Fatalf("operation key lost: %+v", service.input)
	}
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/public/venues/10000000-0000-4000-8000-000000000001/reservations/10000000-0000-4000-8000-000000000001", nil)
	request.Header.Set("X-Reservation-Key", "hold-route-test-123")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "BOOK123") {
		t.Fatalf("reload: %d %s", response.Code, response.Body.String())
	}
	service.confirmErr = reservations.ErrHoldExpired
	request = httptest.NewRequest(http.MethodPost,
		"/api/v1/public/venues/10000000-0000-4000-8000-000000000001/reservations/10000000-0000-4000-8000-000000000001/confirm", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "hold_expired") {
		t.Fatalf("expired hold: %d %s", response.Code, response.Body.String())
	}
}

func TestPublicReservationKeepsEnteredContactsForSignedInUser(t *testing.T) {
	service := &reservationRouteSpy{}
	handler := New(Dependencies{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Identity: &reservationIdentity{},
		Reservations: service, SessionCookie: "pocket_session",
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/public/venues/venue-1/reservations", strings.NewReader(`{
		"starts_at":"2026-10-01T16:00:00Z","guests":2,"guest_name":"Other Guest","guest_email":"other@example.test",
		"guest_phone":"+999999999","idempotency_key":"desktop-booking-test"
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "pocket_session", Value: "active-session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("booking response: %d %s", response.Code, response.Body.String())
	}
	if service.input.CustomerUserID != "10000000-0000-4000-8000-000000000001" || service.input.GuestName != "Other Guest" ||
		service.input.GuestEmail != "other@example.test" || service.input.GuestPhone != "+999999999" {
		t.Fatalf("entered booking contacts were not preserved: %+v", service.input)
	}
}

func TestPublicReservationFallsBackToAccountContactsWhenOmitted(t *testing.T) {
	service := &reservationRouteSpy{}
	handler := New(Dependencies{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Identity: &reservationIdentity{},
		Reservations: service, SessionCookie: "pocket_session",
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/public/venues/venue-1/reservations", strings.NewReader(`{
		"starts_at":"2026-10-01T16:00:00Z","guests":2,"idempotency_key":"desktop-booking-legacy"
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "pocket_session", Value: "active-session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.input.GuestName != "Account Guest" || service.input.GuestEmail != "account@example.test" || service.input.GuestPhone != "+421900123456" {
		t.Fatalf("account contact fallback: %d %+v", response.Code, service.input)
	}
}

func TestPublicReservationAllowsGuestEmailWithoutSession(t *testing.T) {
	service := &reservationRouteSpy{}
	handler := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Reservations: service, SessionCookie: "pocket_session"})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/public/venues/venue-1/reservations", strings.NewReader(`{
		"starts_at":"2026-10-01T16:00:00Z","guests":2,"guest_name":"Guest Name","guest_email":"guest@example.test",
		"idempotency_key":"desktop-booking-guest"
	}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.input.CustomerUserID != "" || service.input.GuestEmail != "guest@example.test" {
		t.Fatalf("guest booking: %d %+v", response.Code, service.input)
	}
}
