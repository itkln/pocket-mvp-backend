package httpapi

import (
	"errors"
	"net/http"

	"pocket-mvp-backend/internal/modules/reservations"
)

func (api *API) listCustomerReservations(w http.ResponseWriter, r *http.Request) {
	items, err := api.reservations.ListCustomer(r.Context(), currentUser(r).ID)
	if err != nil {
		api.writeError(w, r, err, failure("Reservations could not be loaded."))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reservations": items})
}

func (api *API) cancelCustomerReservation(w http.ResponseWriter, r *http.Request) {
	reservation, err := api.reservations.CancelCustomer(r.Context(), currentUser(r).ID, pathParam(r, "reservationID"))
	if err != nil {
		// The reason (too late, already cancelled...) is the code the client localizes.
		var denied *reservations.CancellationDenied
		if errors.As(err, &denied) {
			writeAPIError(w, http.StatusConflict, denied.Reason, "This reservation cannot be cancelled.")
			return
		}
		api.writeError(w, r, err, customerReservationErrors, failure("Reservation could not be cancelled."))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reservation": reservation})
}
