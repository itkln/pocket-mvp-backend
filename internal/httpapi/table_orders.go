package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"pocket-mvp-backend/internal/modules/dinein"
)

func (api *API) resolveTableQR(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	table, err := api.dineIn.Resolve(r.Context(), input.Token)
	if err != nil {
		api.writeError(w, r, err, tableOrderErrors)
		return
	}
	if _, ok := api.tableGuestToken(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"table": table})
}
func (api *API) createTableOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input dinein.CreateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	var ok bool
	input.CustomerUserID, input.GuestToken, ok = api.tableOrderIdentity(w, r)
	if !ok {
		return
	}
	order, err := api.dineIn.Create(r.Context(), input)
	if err != nil {
		api.writeError(w, r, err, tableOrderErrors)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"order": order})
}
func (api *API) lookupTableOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		Token string `json:"token"`
		Key   string `json:"idempotency_key"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	customerID, guestToken, ok := api.tableOrderIdentity(w, r)
	if !ok {
		return
	}
	order, err := api.dineIn.Lookup(r.Context(), input.Token, input.Key, dinein.HistoryInput{CustomerUserID: customerID, GuestToken: guestToken})
	if err != nil {
		api.writeError(w, r, err, tableOrderErrors)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": order})
}

// A persistent HttpOnly capability identifies guest orders on this device.
// Account requests use only their authenticated user, never a supplied user ID.
func (api *API) tableOrderIdentity(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	user, signedIn, err := api.optionalUser(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "Sign in again or continue as a guest.")
		return "", "", false
	}
	if signedIn {
		return user.ID, "", true
	}
	token, ok := api.tableGuestToken(w, r)
	return "", token, ok
}

func (api *API) tableGuestToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	const name = "pocket_table_guest"
	token := ""
	if cookie, err := r.Cookie(name); err == nil {
		if raw, err := base64.RawURLEncoding.DecodeString(cookie.Value); err == nil && len(raw) == 32 {
			token = cookie.Value
		}
	}
	if token == "" {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			api.writeError(w, r, err, tableOrderErrors)
			return "", false
		}
		token = base64.RawURLEncoding.EncodeToString(raw)
	}
	http.SetCookie(w, &http.Cookie{Name: name, Value: token, Path: "/api/v1/public", HttpOnly: true, Secure: api.sessionSecure, SameSite: http.SameSiteLaxMode, MaxAge: 365 * 24 * 60 * 60, Expires: time.Now().Add(365 * 24 * time.Hour)})
	return token, true
}

func (api *API) tableOrderHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input dinein.HistoryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	var ok bool
	input.CustomerUserID, input.GuestToken, ok = api.tableOrderIdentity(w, r)
	if !ok {
		return
	}
	page, err := api.dineIn.History(r.Context(), input)
	if err != nil {
		api.writeError(w, r, err, tableOrderErrors)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (api *API) currentTableBill(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	bill, err := api.dineIn.CurrentBill(r.Context(), input.Token)
	if err != nil {
		api.writeError(w, r, err, tableOrderErrors)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bill": bill})
}

// callStaffToTable: a guest asks the staff to come and settle the table's current bill.
func (api *API) callStaffToTable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	bill, err := api.dineIn.CallStaff(r.Context(), input.Token)
	if err != nil {
		api.writeError(w, r, err, tableOrderErrors)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bill": bill})
}
