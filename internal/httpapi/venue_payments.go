package httpapi

import "net/http"

func (api *API) listPayments(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	items, err := api.billing.ListPayments(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"payments": items})
}
