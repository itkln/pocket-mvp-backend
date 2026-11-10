package httpapi

import "net/http"

func (api *API) listReviews(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	items, err := api.feedback.List(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": items})
}

func (api *API) replyToReview(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input struct {
		OwnerReply string `json:"owner_reply"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.feedback.Reply(
		r.Context(), user.ID, pathParam(r, "venueID"), pathParam(r, "reviewID"), input.OwnerReply,
	)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"review": item})
}
