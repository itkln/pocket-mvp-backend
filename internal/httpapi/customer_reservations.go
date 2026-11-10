package httpapi

import (
	"errors"
	"net/http"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/reservations"
)

func (api *API) listCustomerReservations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	reservations, err := api.reservations.ListCustomer(r.Context(), user.ID)
	if err != nil {
		api.logger.Error("customer reservations failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "Reservations could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reservations": reservations})
}

func (api *API) cancelCustomerReservation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	reservation, err := api.reservations.CancelCustomer(r.Context(), user.ID, pathParam(r, "reservationID"))
	if err != nil {
		var denied *reservations.CancellationDenied
		switch {
		case errors.As(err, &denied):
			writeAPIError(w, http.StatusConflict, denied.Reason, "This reservation cannot be cancelled.")
		case errors.Is(err, appfault.ErrNotFound):
			writeAPIError(w, http.StatusNotFound, "not_found", "Reservation not found.")
		case errors.Is(err, appfault.ErrInvalidInput):
			writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Invalid reservation ID.")
		default:
			api.logger.Error("customer reservation cancellation failed", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "Reservation could not be cancelled.")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reservation": reservation})
}
