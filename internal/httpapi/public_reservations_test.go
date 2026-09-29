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

type reservationRouteSpy struct{ input reservations.CreateInput }

func (s *reservationRouteSpy) Create(_ context.Context, _ string, input reservations.CreateInput) (reservations.Result, error) {
	s.input = input
	return reservations.Result{ID: "booking-1", Code: "BOOK123", StartsAt: time.Now(), GuestCount: input.Guests}, nil
}

func TestPublicReservationUsesAccountContactsFromSession(t *testing.T) {
	service := &reservationRouteSpy{}
	handler := New(Dependencies{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Identity: &reservationIdentity{},
		Reservations: service, SessionCookie: "pocket_session",
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/public/venues/venue-1/reservations", strings.NewReader(`{
		"starts_at":"2026-10-01T16:00:00Z","guests":2,"guest_name":"Forged","guest_email":"forged@example.test",
		"guest_phone":"+999999999","idempotency_key":"desktop-booking-test"
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "pocket_session", Value: "active-session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("booking response: %d %s", response.Code, response.Body.String())
	}
	if service.input.CustomerUserID != "10000000-0000-4000-8000-000000000001" || service.input.GuestName != "Account Guest" ||
		service.input.GuestEmail != "account@example.test" || service.input.GuestPhone != "+421900123456" {
		t.Fatalf("session did not override guest contacts: %+v", service.input)
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
