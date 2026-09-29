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
	input, ok := api.publicReservationInput(w, r)
	if !ok {
		return
	}
	result, err := api.reservations.Create(r.Context(), pathParam(r, "venueID"), input)
	if err != nil {
		api.writeReservationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"reservation": result})
}

func (api *API) publicReservationInput(w http.ResponseWriter, r *http.Request) (reservations.CreateInput, bool) {
	var input reservations.CreateInput
	if !decodeOwnerJSON(w, r, &input) {
		return input, false
	}
	if cookie, err := r.Cookie(api.sessionCookie); err == nil {
		user, err := api.identity.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "Sign in again to complete the booking.")
			return input, false
		}
		input.CustomerUserID = user.ID
		if strings.TrimSpace(input.GuestName) == "" && strings.TrimSpace(input.GuestEmail) == "" && strings.TrimSpace(input.GuestPhone) == "" {
			input.GuestName = strings.TrimSpace(user.FirstName + " " + user.LastName)
			input.GuestEmail = user.Email
			input.GuestPhone = user.Phone
		}
	}
	return input, true
}

func (api *API) holdPublicReservation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	input, ok := api.publicReservationInput(w, r)
	if !ok {
		return
	}
	result, err := api.reservations.Hold(r.Context(), pathParam(r, "venueID"), input)
	if err != nil {
		api.writeReservationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"hold": result})
}

func (api *API) confirmPublicReservation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	input, ok := api.publicReservationInput(w, r)
	if !ok {
		return
	}
	result, err := api.reservations.Confirm(r.Context(), pathParam(r, "venueID"), pathParam(r, "reservationID"), input)
	if err != nil {
		api.writeReservationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"reservation": result})
}

func (api *API) getPublicReservation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	result, err := api.reservations.GetConfirmed(r.Context(), pathParam(r, "venueID"),
		pathParam(r, "reservationID"), r.Header.Get("X-Reservation-Key"))
	if err != nil {
		api.writeReservationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reservation": result})
}

func (api *API) writeReservationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, reservations.ErrPriceChanged):
		writeAPIError(w, http.StatusConflict, "price_changed", "The price changed. Review the current menu and confirm again.")
	case errors.Is(err, reservations.ErrItemUnavailable):
		writeAPIError(w, http.StatusConflict, "item_unavailable", "A selected item is no longer available.")
	case errors.Is(err, appfault.ErrConflict):
		writeAPIError(w, http.StatusConflict, "slot_unavailable", "This time is no longer available. Choose another slot.")
	case errors.Is(err, reservations.ErrHoldExpired):
		writeAPIError(w, http.StatusConflict, "hold_expired", "The reservation hold expired. Choose a time again.")
	case errors.Is(err, appfault.ErrInvalidInput):
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Check the booking details.")
	case errors.Is(err, appfault.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "Venue not found.")
	default:
		api.logger.Error("reservation request failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "The reservation could not be saved.")
	}
}
