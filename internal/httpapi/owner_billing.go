package httpapi

import (
	"net/http"

	"pocket-mvp-backend/internal/modules/billing"
)

func (api *API) listOwnerPayments(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	items, err := api.billing.ListPayments(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"payments": items})
}

func (api *API) getOwnerSubscription(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	item, err := api.billing.GetSubscription(r.Context(), user.ID)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscription": item})
}

func (api *API) updateOwnerSubscription(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input billing.SubscriptionInput
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	item, err := api.billing.UpsertSubscription(r.Context(), user.ID, input)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscription": item})
}
