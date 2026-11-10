package httpapi

import (
	"net/http"

	"pocket-mvp-backend/internal/modules/workforce"
)

func (api *API) listTeam(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	venueID := pathParam(r, "venueID")
	items, err := api.workforce.List(r.Context(), user.ID, venueID)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"staff": items})
}

func (api *API) createTeamMember(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input workforce.StaffInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.workforce.Create(r.Context(), user.ID, pathParam(r, "venueID"), input)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"staff_member": item})
}

func (api *API) updateTeamMember(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input workforce.StaffInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.workforce.Update(
		r.Context(),
		user.ID,
		pathParam(r, "venueID"),
		pathParam(r, "memberID"),
		input,
	)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"staff_member": item})
}

func (api *API) deleteTeamMember(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if err := api.workforce.Delete(
		r.Context(),
		user.ID,
		pathParam(r, "venueID"),
		pathParam(r, "memberID"),
	); err != nil {
		api.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) resendTeamInvitation(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if err := api.workforce.SendInvitation(r.Context(), user.ID, pathParam(r, "venueID"), pathParam(r, "memberID")); err != nil {
		api.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
