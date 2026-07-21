package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"pocket-mvp-backend/internal/modules/identity"
)

const maxAuthBody = 64 << 10

func (api *API) currentUser(w http.ResponseWriter, r *http.Request) (identity.User, bool) {
	cookie, err := r.Cookie(api.sessionCookie)
	if err != nil {
		api.writeAuthError(w, identity.ErrUnauthorized)
		return identity.User{}, false
	}
	user, err := api.identity.Authenticate(r.Context(), cookie.Value)
	if err != nil {
		api.writeAuthError(w, err)
		return identity.User{}, false
	}
	return user, true
}

type registerRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Password  string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	User identity.User `json:"user"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (api *API) register(w http.ResponseWriter, r *http.Request) {
	var request registerRequest
	if !decodeAuthJSON(w, r, &request) {
		return
	}
	user, session, err := api.identity.Register(r.Context(), identity.RegisterInput{
		FirstName: request.FirstName,
		LastName:  request.LastName,
		Email:     request.Email,
		Phone:     request.Phone,
		Password:  request.Password,
		UserAgent: r.UserAgent(),
		IPAddress: clientIP(r),
	})
	if err != nil {
		api.writeAuthError(w, err)
		return
	}
	api.setSessionCookie(w, session)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, authResponse{User: user})
}

func (api *API) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if !decodeAuthJSON(w, r, &request) {
		return
	}
	user, session, err := api.identity.Login(r.Context(), identity.LoginInput{
		Email:     request.Email,
		Password:  request.Password,
		UserAgent: r.UserAgent(),
		IPAddress: clientIP(r),
	})
	if err != nil {
		api.writeAuthError(w, err)
		return
	}
	api.setSessionCookie(w, session)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, authResponse{User: user})
}

func (api *API) me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(api.sessionCookie)
	if err != nil {
		api.writeAuthError(w, identity.ErrUnauthorized)
		return
	}
	user, err := api.identity.Authenticate(r.Context(), cookie.Value)
	if err != nil {
		api.writeAuthError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, authResponse{User: user})
}

func (api *API) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(api.sessionCookie); err == nil {
		if err := api.identity.Logout(r.Context(), cookie.Value); err != nil {
			api.logger.Error("revoke session", "error", err)
			api.writeAuthError(w, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: api.sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: api.sessionSecure, SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) setSessionCookie(w http.ResponseWriter, session identity.Session) {
	http.SetCookie(w, &http.Cookie{
		Name: api.sessionCookie, Value: session.Token, Path: "/",
		Expires: session.ExpiresAt, MaxAge: int(session.ExpiresAt.Sub(timeNow()).Seconds()),
		HttpOnly: true, Secure: api.sessionSecure, SameSite: http.SameSiteLaxMode,
	})
}

var timeNow = func() time.Time { return time.Now().UTC() }

func decodeAuthJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "Проверьте введенные данные")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "Проверьте введенные данные")
		return false
	}
	return true
}

func (api *API) writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Проверьте поля формы. Пароль должен содержать от 12 до 128 символов")
	case errors.Is(err, identity.ErrEmailAlreadyExists):
		writeAPIError(w, http.StatusConflict, "account_exists", "Аккаунт с таким e-mail уже существует")
	case errors.Is(err, identity.ErrInvalidCredentials):
		writeAPIError(w, http.StatusUnauthorized, "invalid_credentials", "Неверный e-mail или пароль")
	case errors.Is(err, identity.ErrTooManyAttempts):
		writeAPIError(w, http.StatusTooManyRequests, "too_many_attempts", "Слишком много попыток. Попробуйте через 15 минут")
	case errors.Is(err, identity.ErrUnauthorized):
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "Требуется вход в аккаунт")
	default:
		api.logger.Error("authentication request failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "Не удалось выполнить запрос")
	}
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message}})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
