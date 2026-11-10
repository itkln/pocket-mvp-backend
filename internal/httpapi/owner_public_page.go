package httpapi

import (
	"net/http"
	"strings"
)

// Whoever edits the venue (owner, manager) receives a link even for a draft. The public endpoint
// continues to enforce publication, so copying a link never publishes the venue implicitly.
func (api *API) ownerPublicPage(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venue, err := api.venues.Get(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	locale := r.URL.Query().Get("locale")
	switch locale {
	case "ru", "en", "ua", "sk":
	default:
		locale = "ru"
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"url": strings.TrimRight(api.dishBaseURL, "/") + "/" + locale + "/r/" + venue.ID, "published": venue.Status == "active"})
}
