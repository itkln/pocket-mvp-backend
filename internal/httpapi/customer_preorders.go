package httpapi

import "net/http"

func (api *API) listCustomerPreorders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	orders, err := api.ordering.ListCustomerPreorders(r.Context(), user.ID)
	if err != nil {
		api.logger.Error("customer preorders failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "Preorders could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preorders": orders})
}
