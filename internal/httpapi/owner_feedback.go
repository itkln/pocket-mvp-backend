package httpapi

import "net/http"

func (api *API) ownerReviews(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	items, err := api.feedback.List(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": items})
}

func (api *API) ownerReview(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		OwnerReply string `json:"owner_reply"`
	}
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	item, err := api.feedback.Reply(
		r.Context(), user.ID, pathParam(r, "venueID"), pathParam(r, "resourceID"), input.OwnerReply,
	)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"review": item})
}
