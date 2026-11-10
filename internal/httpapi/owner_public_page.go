package httpapi

import (
	"net/http"
	"strings"
)

// The owner receives a link even for a draft. The public endpoint continues to
// enforce publication, so copying a link never publishes the venue implicitly.
func (api *API) ownerPublicPage(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	items, err := api.venues.List(r.Context(), user.ID)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	id := pathParam(r, "venueID")
	for _, venue := range items {
		if venue.ID != id {
			continue
		}
		locale := r.URL.Query().Get("locale")
		switch locale {
		case "ru", "en", "ua", "sk":
		default:
			locale = "ru"
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]any{"url": strings.TrimRight(api.dishBaseURL, "/") + "/" + locale + "/r/" + venue.ID, "published": venue.Status == "active"})
		return
	}
	writeAPIError(w, http.StatusNotFound, "not_found", "Venue not found.")
}
