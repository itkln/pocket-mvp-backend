package httpapi

import (
	"errors"
	"net/http"

	"pocket-mvp-backend/internal/appfault"
)

func (api *API) listSavedVenues(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	ids, err := api.favorites.List(r.Context(), user.ID)
	if err != nil {
		api.writeFavoritesError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, map[string]any{"venue_ids": ids})
}

func (api *API) addSavedVenue(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	if err := api.favorites.Add(r.Context(), user.ID, pathParam(r, "venueID")); err != nil {
		api.writeFavoritesError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) removeSavedVenue(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	if err := api.favorites.Remove(r.Context(), user.ID, pathParam(r, "venueID")); err != nil {
		api.writeFavoritesError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) writeFavoritesError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, appfault.ErrInvalidInput):
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Invalid venue ID.")
	case errors.Is(err, appfault.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "Venue not found.")
	default:
		api.logger.Error("saved venues request failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "Saved places could not be updated.")
	}
}
