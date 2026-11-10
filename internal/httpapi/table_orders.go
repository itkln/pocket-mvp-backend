package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/dinein"
)

func (api *API) issueTableQR(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Identifier string `json:"table_identifier"`
		Rotate     bool   `json:"rotate"`
	}
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	result, err := api.dineIn.OwnerQRCode(r.Context(), user.ID, pathParam(r, "venueID"), input.Identifier, input.Rotate)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"qr": result})
}
func (api *API) resolveTableQR(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		Token string `json:"token"`
	}
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	table, err := api.dineIn.Resolve(r.Context(), input.Token)
	if err != nil {
		api.writeTableOrderError(w, err)
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
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	var ok bool
	input.CustomerUserID, input.GuestToken, ok = api.tableOrderIdentity(w, r)
	if !ok {
		return
	}
	order, err := api.dineIn.Create(r.Context(), input)
	if err != nil {
		api.writeTableOrderError(w, err)
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
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	customerID, guestToken, ok := api.tableOrderIdentity(w, r)
	if !ok {
		return
	}
	order, err := api.dineIn.Lookup(r.Context(), input.Token, input.Key, dinein.HistoryInput{CustomerUserID: customerID, GuestToken: guestToken})
	if err != nil {
		api.writeTableOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": order})
}

// A persistent HttpOnly capability identifies guest orders on this device.
// Account requests use only their authenticated user, never a supplied user ID.
func (api *API) tableOrderIdentity(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	if cookie, err := r.Cookie(api.sessionCookie); err == nil {
		user, err := api.identity.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			writeAPIError(w, 401, "unauthorized", "Sign in again or continue as a guest.")
			return "", "", false
		}
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
			api.writeTableOrderError(w, err)
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
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	var ok bool
	input.CustomerUserID, input.GuestToken, ok = api.tableOrderIdentity(w, r)
	if !ok {
		return
	}
	page, err := api.dineIn.History(r.Context(), input)
	if err != nil {
		api.writeTableOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (api *API) writeTableOrderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, dinein.ErrQRInvalid):
		writeAPIError(w, 404, "qr_invalid", "This table QR is no longer valid. Ask the staff for a new code.")
	case errors.Is(err, dinein.ErrPriceChanged):
		writeAPIError(w, 409, "price_changed", "Review the updated menu prices.")
	case errors.Is(err, dinein.ErrItemUnavailable):
		writeAPIError(w, 409, "item_unavailable", "A selected dish is no longer available.")
	case errors.Is(err, dinein.ErrBillLocked):
		writeAPIError(w, 409, "bill_locked", "Ordering is currently unavailable. Contact the staff.")
	case errors.Is(err, appfault.ErrInvalidInput):
		writeAPIError(w, 422, "invalid_input", "Check the order details.")
	case errors.Is(err, appfault.ErrNotFound):
		writeAPIError(w, 404, "not_found", "Order not found.")
	case errors.Is(err, appfault.ErrConflict):
		writeAPIError(w, 409, "conflict", "This request was already used for a different order.")
	default:
		api.logger.Error("table order request failed", "error", err)
		writeAPIError(w, 500, "internal_server_error", "The order could not be saved.")
	}
}

func (api *API) currentTableBill(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		Token string `json:"token"`
	}
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	bill, err := api.dineIn.CurrentBill(r.Context(), input.Token)
	if err != nil {
		api.writeTableOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bill": bill})
}
