package httpapi

import (
	"errors"
	"net/http"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/ordering"
	"strconv"
	"strings"
)

func (api *API) listOrders(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	values := r.URL.Query()
	query := ordering.Query{Search: values.Get("number"), Table: values.Get("table"), DateFrom: values.Get("date_from"), DateTo: values.Get("date_to"), Scope: values.Get("scope"), Channel: values.Get("channel"), TableID: values.Get("table_id"), Cursor: values.Get("cursor")}
	if value, exists := values["limit"]; exists {
		var err error
		if len(value) != 1 {
			api.writeError(w, r, appfault.ErrInvalidInput)
			return
		}
		query.Limit, err = strconv.Atoi(value[0])
		if err != nil || query.Limit < 1 {
			api.writeError(w, r, appfault.ErrInvalidInput)
			return
		}
	}
	if value, exists := values["status"]; exists {
		query.Statuses = strings.Split(strings.Join(value, ","), ",")
	}
	items, err := api.ordering.ListPage(r.Context(), user.ID, pathParam(r, "venueID"), query)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (api *API) updateOrderStatus(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input struct {
		Status         string `json:"status"`
		ExpectedStatus string `json:"expected_status"`
		Reason         string `json:"reason"`
		Comment        string `json:"comment"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.ordering.UpdateStatus(
		r.Context(), user.ID, pathParam(r, "venueID"), pathParam(r, "orderID"), input.Status, input.ExpectedStatus, ordering.StatusChange{Reason: input.Reason, Comment: input.Comment},
	)
	if err != nil {
		var conflict *ordering.StatusConflict
		if errors.As(err, &conflict) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "order_status_conflict", "message": "Order status changed or this transition is not allowed."}, "current_status": conflict.CurrentStatus})
			return
		}
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": item})
}

func (api *API) getOrder(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	order, err := api.ordering.Get(r.Context(), user.ID, pathParam(r, "venueID"), pathParam(r, "orderID"))
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": order})
}

func (api *API) getOrderAlertSettings(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	settings, err := api.ordering.GetAlertSettings(r.Context(), user.ID, pathParam(r, "venueID"))
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}
func (api *API) saveOrderAlertSettings(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input ordering.AlertSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	settings, err := api.ordering.SaveAlertSettings(r.Context(), user.ID, pathParam(r, "venueID"), input)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}
