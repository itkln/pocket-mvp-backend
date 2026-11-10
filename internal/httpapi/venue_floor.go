package httpapi

import (
	"encoding/json"
	"net/http"
)

func (api *API) getFloorPlan(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	plan, err := api.floorPlan.Get(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"floor_plan": plan})
}

func (api *API) updateFloorPlan(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input struct {
		FloorPlan json.RawMessage `json:"floor_plan"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	plan, err := api.floorPlan.Save(r.Context(), user.ID, pathParam(r, "venueID"), input.FloorPlan)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"floor_plan": plan})
}

// issueTableQR returns a table's QR code, rotating it on request (the old code stops working).
func (api *API) issueTableQR(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input struct {
		Identifier string `json:"table_identifier"`
		Rotate     bool   `json:"rotate"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := api.dineIn.OwnerQRCode(r.Context(), user.ID, pathParam(r, "venueID"), input.Identifier, input.Rotate)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"qr": result})
}
