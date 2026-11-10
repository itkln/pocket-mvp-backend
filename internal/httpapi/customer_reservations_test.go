package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/reservations"
)

func TestCancelCustomerReservationRoute(t *testing.T) {
	for _, test := range []struct {
		name     string
		signedIn bool
		err      error
		status   int
		code     string
	}{
		{"anonymous", false, nil, 401, "unauthorized"},
		{"own reservation", true, nil, 200, "cancelled"},
		{"foreign or missing", true, appfault.ErrNotFound, 404, "not_found"},
		{"invalid ID", true, appfault.ErrInvalidInput, 422, "invalid_input"},
		{"deadline", true, &reservations.CancellationDenied{Reason: "cancellation_deadline_passed"}, 409, "cancellation_deadline_passed"},
		{"preorder", true, &reservations.CancellationDenied{Reason: "cancellation_preorder_locked"}, 409, "cancellation_preorder_locked"},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &reservationRouteSpy{cancelErr: test.err}
			handler := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Identity: &reservationIdentity{}, Reservations: service, SessionCookie: "pocket_session"})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/customer/reservations/20000000-0000-4000-8000-000000000001/cancel?customer_user_id=other-account", strings.NewReader(`{"customer_user_id":"other-account"}`))
			if test.signedIn {
				request.AddCookie(&http.Cookie{Name: "pocket_session", Value: "active-session"})
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("%d %s", response.Code, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("private cancellation response must not be cached")
			}
			if test.signedIn && (service.customerID != "10000000-0000-4000-8000-000000000001" || service.reservationID != "20000000-0000-4000-8000-000000000001") {
				t.Fatal("cancellation did not use authenticated account and path ID")
			}
			if !test.signedIn && service.customerID != "" {
				t.Fatal("anonymous cancellation reached service")
			}
		})
	}
}
