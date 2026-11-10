package httpapi

import (
	"net/http"

	"pocket-mvp-backend/internal/modules/billing"
	"pocket-mvp-backend/internal/modules/venues"
)

// The owner's workspace as a whole: their venues and the subscription.

func (api *API) listOwnerVenues(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	items, err := api.venues.List(r.Context(), user.ID)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"venues": items})
}

func (api *API) createOwnerVenue(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input venues.Input
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.venues.Create(r.Context(), user.ID, input)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"venue": item})
}

func (api *API) getOwnerSubscription(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	item, err := api.billing.GetSubscription(r.Context(), user.ID)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscription": item})
}

func (api *API) updateOwnerSubscription(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input billing.SubscriptionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.billing.UpsertSubscription(r.Context(), user.ID, input)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscription": item})
}
