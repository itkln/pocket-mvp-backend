package httpapi

import (
	"context"
	"net/http"
	"time"

	"pocket-mvp-backend/internal/modules/identity"
)

type sessionKey struct{}

type session struct {
	user  identity.User
	token string
}

// requireUser lets only signed-in users through and hands their session to the handlers.
func (api *API) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(api.sessionCookie)
		if err != nil {
			api.writeError(w, r, identity.ErrUnauthorized)
			return
		}
		user, err := api.identity.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			api.writeError(w, r, err, authErrors)
			return
		}
		ctx := context.WithValue(r.Context(), sessionKey{}, session{user: user, token: cookie.Value})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// currentSession is available to handlers behind requireUser; anywhere else it is a wiring bug.
func currentSession(r *http.Request) session {
	current, ok := r.Context().Value(sessionKey{}).(session)
	if !ok {
		panic("httpapi: handler needs a signed-in user but its route is not behind requireUser")
	}
	return current
}

func currentUser(r *http.Request) identity.User {
	return currentSession(r).user
}

// optionalUser is for public routes that also serve signed-in users. A request without a session
// cookie is a guest; a cookie that no longer authenticates is an error the caller reports.
func (api *API) optionalUser(r *http.Request) (identity.User, bool, error) {
	cookie, err := r.Cookie(api.sessionCookie)
	if err != nil {
		return identity.User{}, false, nil
	}
	user, err := api.identity.Authenticate(r.Context(), cookie.Value)
	if err != nil {
		return identity.User{}, false, err
	}
	return user, true, nil
}

func (api *API) setSessionCookie(w http.ResponseWriter, session identity.Session) {
	http.SetCookie(w, &http.Cookie{
		Name: api.sessionCookie, Value: session.Token, Path: "/",
		Expires: session.ExpiresAt, MaxAge: int(session.ExpiresAt.Sub(timeNow()).Seconds()),
		HttpOnly: true, Secure: api.sessionSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (api *API) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: api.sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: api.sessionSecure, SameSite: http.SameSiteLaxMode,
	})
}

var timeNow = func() time.Time { return time.Now().UTC() }
