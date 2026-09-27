package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/venues"
)

func (api *API) searchPublicVenues(w http.ResponseWriter, r *http.Request) {
	page, ok := positiveQueryInt(r, "page", 1)
	if !ok {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Check the catalog parameters.")
		return
	}
	pageSize, ok := positiveQueryInt(r, "page_size", 24)
	if !ok {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Check the catalog parameters.")
		return
	}
	var ids []string
	if raw, present := r.URL.Query()["ids"]; present {
		if len(raw) != 1 || strings.TrimSpace(raw[0]) == "" {
			writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Check the catalog parameters.")
			return
		}
		ids = strings.Split(raw[0], ",")
	}
	result, err := api.venues.SearchPublished(r.Context(), venues.PublicVenueQuery{
		City: r.URL.Query().Get("city"), Search: r.URL.Query().Get("q"),
		Category: r.URL.Query().Get("category"), Filter: r.URL.Query().Get("filter"),
		IDs: ids, Page: page, PageSize: pageSize,
	})
	if err != nil {
		if errors.Is(err, appfault.ErrInvalidInput) {
			writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Check the catalog parameters.")
			return
		}
		api.logger.Error("public venue search failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "The catalog could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *API) getPublicVenue(w http.ResponseWriter, r *http.Request) {
	item, err := api.venues.GetPublished(r.Context(), pathParam(r, "venueID"))
	if err != nil {
		switch {
		case errors.Is(err, appfault.ErrInvalidInput):
			writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Invalid venue ID.")
		case errors.Is(err, appfault.ErrNotFound):
			writeAPIError(w, http.StatusNotFound, "not_found", "Venue not found.")
		default:
			api.logger.Error("public venue details failed", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "The venue could not be loaded.")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"venue": item})
}

func (api *API) getPublicVenueMenu(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	categories, err := api.venues.ListPublishedMenu(r.Context(), pathParam(r, "venueID"))
	if err != nil {
		switch {
		case errors.Is(err, appfault.ErrInvalidInput):
			writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Invalid venue ID.")
		case errors.Is(err, appfault.ErrNotFound):
			writeAPIError(w, http.StatusNotFound, "not_found", "Venue not found.")
		default:
			api.logger.Error("public venue menu failed", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "The menu could not be loaded.")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": categories})
}

func (api *API) getPublicVenueReviews(w http.ResponseWriter, r *http.Request) {
	reviews, err := api.venues.ListPublishedReviews(r.Context(), pathParam(r, "venueID"))
	if err != nil {
		switch {
		case errors.Is(err, appfault.ErrInvalidInput):
			writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Invalid venue ID.")
		case errors.Is(err, appfault.ErrNotFound):
			writeAPIError(w, http.StatusNotFound, "not_found", "Venue not found.")
		default:
			api.logger.Error("public venue reviews failed", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "The reviews could not be loaded.")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": reviews})
}

func (api *API) getPublicVenueAvailability(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	guests, ok := positiveQueryInt(r, "guests", 2)
	if !ok || guests > 20 || len(r.URL.Query()["date"]) > 1 || len(r.URL.Query()["guests"]) > 1 {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Check the booking parameters.")
		return
	}
	result, err := api.availability.Get(r.Context(), pathParam(r, "venueID"), r.URL.Query().Get("date"), guests)
	if err != nil {
		switch {
		case errors.Is(err, appfault.ErrInvalidInput):
			writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Check the booking parameters.")
		case errors.Is(err, appfault.ErrNotFound):
			writeAPIError(w, http.StatusNotFound, "not_found", "Venue not found.")
		default:
			api.logger.Error("public venue availability failed", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "Availability could not be loaded.")
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func positiveQueryInt(r *http.Request, name string, fallback int) (int, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed > 0
}
