package httpapi

import (
	"errors"
	"net/http"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/ordering"
	"strconv"
	"strings"
)

func (api *API) listOwnerOrders(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	values := r.URL.Query()
	query := ordering.Query{Search: values.Get("number"), Table: values.Get("table"), DateFrom: values.Get("date_from"), DateTo: values.Get("date_to"), Scope: values.Get("scope"), Channel: values.Get("channel"), TableID: values.Get("table_id"), Cursor: values.Get("cursor")}
	if value, exists := values["limit"]; exists {
		var err error
		if len(value) != 1 {
			api.writeOwnerError(w, appfault.ErrInvalidInput)
			return
		}
		query.Limit, err = strconv.Atoi(value[0])
		if err != nil || query.Limit < 1 {
			api.writeOwnerError(w, appfault.ErrInvalidInput)
			return
		}
	}
	if value, exists := values["status"]; exists {
		query.Statuses = strings.Split(strings.Join(value, ","), ",")
	}
	items, err := api.ordering.ListPage(r.Context(), user.ID, pathParam(r, "venueID"), query)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (api *API) updateOwnerOrderStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Status         string `json:"status"`
		ExpectedStatus string `json:"expected_status"`
		Reason         string `json:"reason"`
		Comment        string `json:"comment"`
	}
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	item, err := api.ordering.UpdateStatus(
		r.Context(), user.ID, pathParam(r, "venueID"), pathParam(r, "resourceID"), input.Status, input.ExpectedStatus, ordering.StatusChange{Reason: input.Reason, Comment: input.Comment},
	)
	if err != nil {
		var conflict *ordering.StatusConflict
		if errors.As(err, &conflict) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "order_status_conflict", "message": "Order status changed or this transition is not allowed."}, "current_status": conflict.CurrentStatus})
			return
		}
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": item})
}

func (api *API) getOwnerOrder(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	order, err := api.ordering.Get(r.Context(), user.ID, pathParam(r, "venueID"), pathParam(r, "resourceID"))
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": order})
}

func (api *API) getOrderAlertSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	settings, err := api.ordering.GetAlertSettings(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}
func (api *API) saveOrderAlertSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input ordering.AlertSettings
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	settings, err := api.ordering.SaveAlertSettings(r.Context(), user.ID, pathParam(r, "venueID"), input)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}
