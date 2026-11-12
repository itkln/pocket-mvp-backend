package httpapi

import (
	"net/http"

	"pocket-mvp-backend/internal/modules/dinein"
)

// The venue's open table bills, the ones whose guests called the staff first.
func (api *API) listOpenBills(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	bills, err := api.dineIn.OpenBills(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeError(w, r, err, billSettlementErrors)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bills": bills})
}

// settleBill: the guests paid the whole bill in person; the visit is closed.
func (api *API) settleBill(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input dinein.SettleInput
	if !decodeJSON(w, r, &input) {
		return
	}
	settlement, err := api.dineIn.Settle(r.Context(), user.ID, pathParam(r, "venueID"), pathParam(r, "billID"), input)
	if err != nil {
		api.writeError(w, r, err, billSettlementErrors)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settlement": settlement})
}
