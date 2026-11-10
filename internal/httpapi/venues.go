package httpapi

// One venue's own settings: profile, removal, dashboard and the public page link.

import (
	"net/http"
	"strings"

	"pocket-mvp-backend/internal/modules/venues"
)

func (api *API) updateVenue(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input venues.Input
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.venues.Update(r.Context(), user.ID, pathParam(r, "venueID"), input)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"venue": item})
}

func (api *API) deleteVenue(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if err := api.venues.Delete(r.Context(), user.ID, pathParam(r, "venueID")); err != nil {
		api.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) getDashboard(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	result, err := api.reporting.Dashboard(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dashboard": result})
}

// Whoever edits the venue (owner, manager) receives a link even for a draft. The public endpoint
// continues to enforce publication, so copying a link never publishes the venue implicitly.
func (api *API) venuePublicPage(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	venue, err := api.venues.Get(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	locale := r.URL.Query().Get("locale")
	switch locale {
	case "ru", "en", "ua", "sk":
	default:
		locale = "ru"
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": strings.TrimRight(api.dishBaseURL, "/") + "/" + locale + "/r/" + venue.ID, "published": venue.Status == "active"})
}
