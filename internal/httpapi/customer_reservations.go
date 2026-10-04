package httpapi

import "net/http"

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
