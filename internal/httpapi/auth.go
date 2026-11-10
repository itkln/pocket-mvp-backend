package httpapi

import (
	"fmt"
	"net/http"

	"pocket-mvp-backend/internal/modules/identity"
)

// Sign-up, sign-in, sign-out and password reset: the routes that start or end a session.

const maxAuthBody = 64 << 10

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

type passwordResetRequest struct {
	Email   string `json:"email"`
	Locale  string `json:"locale"`
	Product string `json:"product"`
}

type passwordResetConfirmation struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type authResponse struct {
	User authUserResponse `json:"user"`
}

type authUserResponse struct {
	identity.User
	AvatarURL string `json:"avatar_url,omitempty"`
}

func newAuthResponse(user identity.User) authResponse {
	response := authUserResponse{User: user}
	if user.AvatarVersion > 0 {
		response.AvatarURL = fmt.Sprintf("/api/v1/auth/me/avatar?v=%d", user.AvatarVersion)
	}
	return authResponse{User: response}
}

func (api *API) register(w http.ResponseWriter, r *http.Request) {
	var request registerRequest
	if !decodeJSON(w, r, &request) {
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
		api.writeError(w, r, err, authErrors)
		return
	}
	api.setSessionCookie(w, session)
	writeJSON(w, http.StatusCreated, newAuthResponse(user))
}

func (api *API) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	user, session, err := api.identity.Login(r.Context(), identity.LoginInput{
		Email:     request.Email,
		Password:  request.Password,
		UserAgent: r.UserAgent(),
		IPAddress: clientIP(r),
	})
	if err != nil {
		api.writeError(w, r, err, authErrors)
		return
	}
	api.setSessionCookie(w, session)
	writeJSON(w, http.StatusOK, newAuthResponse(user))
}

// logout works without a valid session too: it always clears the cookie.
func (api *API) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(api.sessionCookie); err == nil {
		if err := api.identity.Logout(r.Context(), cookie.Value); err != nil {
			api.writeError(w, r, err, authErrors)
			return
		}
	}
	api.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var request passwordResetRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	err := api.identity.RequestPasswordReset(r.Context(), identity.PasswordResetRequest{
		Email: request.Email, Locale: request.Locale, Product: request.Product, IPAddress: clientIP(r),
	})
	if err != nil {
		api.writeError(w, r, err, passwordResetErrors)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (api *API) resetPassword(w http.ResponseWriter, r *http.Request) {
	var request passwordResetConfirmation
	if !decodeJSON(w, r, &request) {
		return
	}
	err := api.identity.ResetPassword(r.Context(), identity.PasswordResetConfirmation{
		Token: request.Token, Password: request.Password,
	})
	if err != nil {
		api.writeError(w, r, err, passwordResetErrors)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
