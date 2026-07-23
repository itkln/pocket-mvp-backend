package httpapi

import (
	"net/http"

	"pocket-mvp-backend/internal/modules/workforce"
)

func (api *API) listOwnerStaff(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID := pathParam(r, "venueID")
	items, err := api.workforce.List(r.Context(), user.ID, venueID)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"staff": items})
}

func (api *API) createOwnerStaffMember(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input workforce.StaffInput
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	item, err := api.workforce.Create(r.Context(), user.ID, pathParam(r, "venueID"), input)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"staff_member": item})
}

func (api *API) updateOwnerStaffMember(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input workforce.StaffInput
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	item, err := api.workforce.Update(
		r.Context(),
		user.ID,
		pathParam(r, "venueID"),
		pathParam(r, "resourceID"),
		input,
	)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"staff_member": item})
}

func (api *API) deleteOwnerStaffMember(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	if err := api.workforce.Delete(
		r.Context(),
		user.ID,
		pathParam(r, "venueID"),
		pathParam(r, "resourceID"),
	); err != nil {
		api.writeOwnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
