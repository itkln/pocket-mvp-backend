package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/identity"
)

func TestDomainErrorsKeepCodesAndUseEnglishMessages(t *testing.T) {
	api := &API{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/venues/venue-1/menu-items", nil)
	testCases := []struct {
		name    string
		write   func(http.ResponseWriter)
		status  int
		code    string
		message string
	}{
		{"invalid auth input", func(w http.ResponseWriter) { api.writeError(w, request, identity.ErrInvalidInput, authErrors) }, 422, "invalid_input", "Check the form fields. The password must be 12 to 128 characters long."},
		{"existing account", func(w http.ResponseWriter) { api.writeError(w, request, identity.ErrEmailAlreadyExists, authErrors) }, 409, "account_exists", "An account with this email already exists."},
		{"invalid credentials", func(w http.ResponseWriter) { api.writeError(w, request, identity.ErrInvalidCredentials, authErrors) }, 401, "invalid_credentials", "Incorrect email or password."},
		{"invalid current password", func(w http.ResponseWriter) {
			api.writeError(w, request, identity.ErrInvalidCurrentPassword, authErrors)
		}, 422, "invalid_current_password", "The current password is incorrect."},
		{"rate limit", func(w http.ResponseWriter) { api.writeError(w, request, identity.ErrTooManyAttempts, authErrors) }, 429, "too_many_attempts", "Too many attempts. Please try again in 15 minutes."},
		{"unauthorized", func(w http.ResponseWriter) { api.writeError(w, request, identity.ErrUnauthorized, authErrors) }, 401, "unauthorized", "Sign in to continue."},
		{"auth internal error", func(w http.ResponseWriter) { api.writeError(w, request, errors.New("storage unavailable"), authErrors) }, 500, "internal_server_error", "The request could not be completed."},
		{"venue invalid input", func(w http.ResponseWriter) { api.writeError(w, request, appfault.ErrInvalidInput) }, 422, "invalid_input", "Check the submitted data."},
		{"venue not found", func(w http.ResponseWriter) { api.writeError(w, request, appfault.ErrNotFound) }, 404, "not_found", "The resource was not found or is unavailable."},
		{"venue conflict", func(w http.ResponseWriter) { api.writeError(w, request, appfault.ErrConflict) }, 409, "conflict", "The resource already exists or is in use."},
		{"venue internal error", func(w http.ResponseWriter) { api.writeError(w, request, errors.New("storage unavailable")) }, 500, "internal_server_error", "The request could not be completed."},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			testCase.write(response)
			if response.Code != testCase.status {
				t.Fatalf("status = %d, want %d", response.Code, testCase.status)
			}
			var payload errorEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if payload.Error.Code != testCase.code || payload.Error.Message != testCase.message {
				t.Fatalf("error = %#v, want code %q and message %q", payload.Error, testCase.code, testCase.message)
			}
		})
	}
}
