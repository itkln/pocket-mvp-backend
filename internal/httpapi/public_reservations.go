package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/reservations"
)

func (api *API) createPublicReservation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input reservations.CreateInput
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	if cookie, err := r.Cookie(api.sessionCookie); err == nil {
		user, err := api.identity.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "Sign in again to complete the booking.")
			return
		}
		input.CustomerUserID = user.ID
		input.GuestName = strings.TrimSpace(user.FirstName + " " + user.LastName)
		input.GuestEmail = user.Email
		input.GuestPhone = user.Phone
	}
	result, err := api.reservations.Create(r.Context(), pathParam(r, "venueID"), input)
	if err != nil {
		switch {
		case errors.Is(err, reservations.ErrPriceChanged):
			writeAPIError(w, http.StatusConflict, "price_changed", "The price changed. Review the current menu and confirm again.")
		case errors.Is(err, reservations.ErrItemUnavailable):
			writeAPIError(w, http.StatusConflict, "item_unavailable", "A selected item is no longer available.")
		case errors.Is(err, appfault.ErrConflict):
			writeAPIError(w, http.StatusConflict, "slot_unavailable", "This time is no longer available. Choose another slot.")
		case errors.Is(err, appfault.ErrInvalidInput):
			writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Check the booking details.")
		case errors.Is(err, appfault.ErrNotFound):
			writeAPIError(w, http.StatusNotFound, "not_found", "Venue not found.")
		default:
			api.logger.Error("reservation creation failed", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "The reservation could not be saved.")
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"reservation": result})
}
