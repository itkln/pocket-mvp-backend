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
)

type fakeAuth struct {
	registerInput identity.RegisterInput
	registerErr   error
	loginErr      error
	authErr       error
	logoutToken   string
}

func (f *fakeAuth) Register(_ context.Context, input identity.RegisterInput) (identity.User, identity.Session, error) {
	f.registerInput = input
	return identity.User{ID: "user-1", Email: input.Email, FirstName: input.FirstName, LastName: input.LastName, Role: "customer"}, identity.Session{Token: "secret-session-token", ExpiresAt: time.Now().Add(time.Hour)}, f.registerErr
}
func (f *fakeAuth) Login(_ context.Context, input identity.LoginInput) (identity.User, identity.Session, error) {
	return identity.User{ID: "user-1", Email: input.Email, Role: "customer"}, identity.Session{Token: "secret-session-token", ExpiresAt: time.Now().Add(time.Hour)}, f.loginErr
}
func (f *fakeAuth) Authenticate(_ context.Context, _ string) (identity.User, error) {
	return identity.User{ID: "user-1", Email: "user@example.com", Role: "customer"}, f.authErr
}
func (f *fakeAuth) Logout(_ context.Context, token string) error {
	f.logoutToken = token
	return nil
}

func authHandler(service IdentityService, secure bool) http.Handler {
	return New(Dependencies{
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		AllowedOrigins: []string{"http://localhost:3000"},
		Identity:       service, SessionCookie: "pocket_session", SessionSecure: secure,
	})
}

func TestRegisterSetsProtectedSessionCookie(t *testing.T) {
	service := &fakeAuth{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"first_name":"Denis","last_name":"Itkin","email":"denis@example.com","password":"a secure password"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://localhost:3000")
	response := httptest.NewRecorder()
	authHandler(service, true).ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected session cookie: %#v", cookies)
	}
	if service.registerInput.Password != "a secure password" || service.registerInput.IPAddress == "" {
		t.Fatal("request was not passed to auth service")
	}
}

func TestRegisterDoesNotAcceptAccountRole(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"first_name":"Denis","last_name":"Itkin","email":"denis@example.com","password":"a secure password","role":"owner"}`))
	response := httptest.NewRecorder()
	authHandler(&fakeAuth{}, false).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected role field to be rejected, got %d", response.Code)
	}
}

func TestOwnerAPIRequiresSession(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/owner/venues", nil)
	response := httptest.NewRecorder()
	authHandler(&fakeAuth{}, false).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestLoginDoesNotRevealCredentialFailure(t *testing.T) {
	service := &fakeAuth{loginErr: identity.ErrInvalidCredentials}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"missing@example.com","password":"wrong password"}`))
	response := httptest.NewRecorder()
	authHandler(service, false).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "invalid_credentials") {
		t.Fatalf("unexpected response %d: %s", response.Code, response.Body.String())
	}
}

func TestAuthRejectsUnknownJSONFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"a@example.com","password":"secret","admin":true}`))
	response := httptest.NewRecorder()
	authHandler(&fakeAuth{}, false).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

func TestUnsafeCrossOriginRequestIsRejected(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	authHandler(&fakeAuth{}, false).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
}

func TestMeRequiresValidSession(t *testing.T) {
	service := &fakeAuth{authErr: identity.ErrUnauthorized}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "pocket_session", Value: "expired"})
	response := httptest.NewRecorder()
	authHandler(service, false).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestLogoutRevokesAndClearsSession(t *testing.T) {
	service := &fakeAuth{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: "pocket_session", Value: "active-token"})
	response := httptest.NewRecorder()
	authHandler(service, false).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || service.logoutToken != "active-token" {
		t.Fatalf("logout failed: status=%d token=%q", response.Code, service.logoutToken)
	}
	if len(response.Result().Cookies()) != 1 || response.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout must clear browser cookie")
	}
}

func TestLoginRateLimitError(t *testing.T) {
	service := &fakeAuth{loginErr: identity.ErrTooManyAttempts}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"a@example.com","password":"wrong password"}`))
	response := httptest.NewRecorder()
	authHandler(service, false).ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
}
