package httpapi

import "net/http"

// Where a user works, and joining a venue team by invitation.

func (api *API) listWorkplaces(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	items, err := api.venues.ListWorkplaces(r.Context(), user.ID)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"venues": items})
}

func (api *API) inspectStaffInvitation(w http.ResponseWriter, r *http.Request) {
	api.handleStaffInvitation(w, r, false)
}

func (api *API) acceptStaffInvitation(w http.ResponseWriter, r *http.Request) {
	api.handleStaffInvitation(w, r, true)
}

func (api *API) handleStaffInvitation(w http.ResponseWriter, r *http.Request, accept bool) {
	user := currentUser(r)
	var input struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	invitation, err := api.workforce.Invitation(r.Context(), user.ID, user.Email, input.Token, accept)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invitation": invitation})
}
