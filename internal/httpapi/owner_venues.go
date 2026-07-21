package httpapi

import (
	"net/http"

	"pocket-mvp-backend/internal/modules/venues"
)

func (api *API) ownerVenues(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := api.venues.List(r.Context(), user.ID)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"venues": items})
	case http.MethodPost:
		var input venues.Input
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.venues.Create(r.Context(), user.ID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"venue": item})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (api *API) ownerVenue(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID := r.PathValue("venueID")
	switch r.Method {
	case http.MethodPatch:
		var input venues.Input
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.venues.Update(r.Context(), user.ID, venueID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"venue": item})
	case http.MethodDelete:
		if err := api.venues.Delete(r.Context(), user.ID, venueID); err != nil {
			api.writeOwnerError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (api *API) ownerDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	result, err := api.reporting.Dashboard(r.Context(), user.ID, r.PathValue("venueID"))
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dashboard": result})
}
