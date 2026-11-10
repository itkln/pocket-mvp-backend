package httpapi

import (
	"net/http"
	"strconv"

	"pocket-mvp-backend/internal/appfault"
)

func (api *API) getPublicVenue(w http.ResponseWriter, r *http.Request) {
	item, err := api.venues.GetPublished(r.Context(), pathParam(r, "venueID"))
	if err != nil {
		api.writeError(w, r, err, publicVenueErrors, failure("The venue could not be loaded."))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"venue": item})
}

func (api *API) getPublicVenueMenu(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	categories, err := api.venues.ListPublishedMenu(r.Context(), pathParam(r, "venueID"))
	if err != nil {
		api.writeError(w, r, err, publicVenueErrors, failure("The menu could not be loaded."))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": categories})
}

func (api *API) getPublicVenueReviews(w http.ResponseWriter, r *http.Request) {
	reviews, err := api.venues.ListPublishedReviews(r.Context(), pathParam(r, "venueID"))
	if err != nil {
		api.writeError(w, r, err, publicVenueErrors, failure("The reviews could not be loaded."))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": reviews})
}

func (api *API) getPublicVenueAvailability(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	guests, ok := positiveQueryInt(r, "guests", 2)
	if !ok || guests > 20 || len(r.URL.Query()["date"]) > 1 || len(r.URL.Query()["guests"]) > 1 {
		writeReply(w, availabilityErrors[0].reply)
		return
	}
	result, err := api.availability.Get(r.Context(), pathParam(r, "venueID"), r.URL.Query().Get("date"), guests)
	if err != nil {
		api.writeError(w, r, err, availabilityErrors, publicVenueErrors, failure("Availability could not be loaded."))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

var availabilityErrors = []errorRule{
	{appfault.ErrInvalidInput, reply{http.StatusUnprocessableEntity, "invalid_input", "Check the booking parameters."}},
}

func positiveQueryInt(r *http.Request, name string, fallback int) (int, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed > 0
}
