package httpapi

import (
	"net/http"
	"strings"

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
		api.writeError(w, r, err, reservationErrors)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"reservation": result})
}

func (api *API) publicReservationInput(w http.ResponseWriter, r *http.Request) (reservations.CreateInput, bool) {
	var input reservations.CreateInput
	if !decodeJSON(w, r, &input) {
		return input, false
	}
	user, signedIn, err := api.optionalUser(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "Sign in again to complete the booking.")
		return input, false
	}
	if signedIn {
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
		api.writeError(w, r, err, reservationErrors)
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
		api.writeError(w, r, err, reservationErrors)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"reservation": result})
}

func (api *API) getPublicReservation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	result, err := api.reservations.GetConfirmed(r.Context(), pathParam(r, "venueID"),
		pathParam(r, "reservationID"), r.Header.Get("X-Reservation-Key"))
	if err != nil {
		api.writeError(w, r, err, reservationErrors)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reservation": result})
}
