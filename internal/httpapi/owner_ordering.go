package httpapi

import "net/http"

func (api *API) listOwnerOrders(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	items, err := api.ordering.List(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": items})
}

func (api *API) updateOwnerOrderStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	item, err := api.ordering.UpdateStatus(
		r.Context(), user.ID, pathParam(r, "venueID"), pathParam(r, "resourceID"), input.Status,
	)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": item})
}
