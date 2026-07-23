package httpapi

import (
	"encoding/json"
	"net/http"
)

func (api *API) getOwnerFloorPlan(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	plan, err := api.floorPlan.Get(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"floor_plan": plan})
}

func (api *API) updateOwnerFloorPlan(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		FloorPlan json.RawMessage `json:"floor_plan"`
	}
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	plan, err := api.floorPlan.Save(r.Context(), user.ID, pathParam(r, "venueID"), input.FloorPlan)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"floor_plan": plan})
}
