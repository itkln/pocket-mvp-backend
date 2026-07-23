package httpapi

import (
	"encoding/json"
	"net/http"
)

func (api *API) ownerFloorPlan(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID := pathParam(r, "venueID")
	switch r.Method {
	case http.MethodGet:
		plan, err := api.floorPlan.Get(r.Context(), user.ID, venueID)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"floor_plan": plan})
	case http.MethodPut:
		var input struct {
			FloorPlan json.RawMessage `json:"floor_plan"`
		}
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		plan, err := api.floorPlan.Save(r.Context(), user.ID, venueID, input.FloorPlan)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"floor_plan": plan})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
