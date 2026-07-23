package httpapi

import (
	"net/http"

	"pocket-mvp-backend/internal/modules/venues"
)

func (api *API) listOwnerVenues(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	items, err := api.venues.List(r.Context(), user.ID)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"venues": items})
}

func (api *API) createOwnerVenue(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
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
}

func (api *API) updateOwnerVenue(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input venues.Input
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	item, err := api.venues.Update(r.Context(), user.ID, pathParam(r, "venueID"), input)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"venue": item})
}

func (api *API) deleteOwnerVenue(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	if err := api.venues.Delete(r.Context(), user.ID, pathParam(r, "venueID")); err != nil {
		api.writeOwnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) getOwnerDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	result, err := api.reporting.Dashboard(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dashboard": result})
}
