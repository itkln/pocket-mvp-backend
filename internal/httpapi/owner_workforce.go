package httpapi

import (
	"net/http"

	"pocket-mvp-backend/internal/modules/workforce"
)

func (api *API) ownerStaff(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID := r.PathValue("venueID")
	switch r.Method {
	case http.MethodGet:
		items, err := api.workforce.List(r.Context(), user.ID, venueID)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"staff": items})
	case http.MethodPost:
		var input workforce.StaffInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.workforce.Create(r.Context(), user.ID, venueID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"staff_member": item})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (api *API) ownerStaffMember(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID, staffID := r.PathValue("venueID"), r.PathValue("resourceID")
	switch r.Method {
	case http.MethodPatch:
		var input workforce.StaffInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.workforce.Update(r.Context(), user.ID, venueID, staffID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"staff_member": item})
	case http.MethodDelete:
		if err := api.workforce.Delete(r.Context(), user.ID, venueID, staffID); err != nil {
			api.writeOwnerError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
