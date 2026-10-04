package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/ordering"
)

func (api *API) orderInput(w http.ResponseWriter, r *http.Request) (ordering.CreateInput, bool) {
	var input ordering.CreateInput
	if !decodeOwnerJSON(w, r, &input) {
		return input, false
	}
	if cookie, err := r.Cookie(api.sessionCookie); err == nil {
		user, err := api.identity.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "Sign in again to place the order.")
			return input, false
		}
		input.CustomerUserID = user.ID
		if strings.TrimSpace(input.GuestName) == "" {
			input.GuestName = strings.TrimSpace(user.FirstName + " " + user.LastName)
		}
		if strings.TrimSpace(input.GuestEmail) == "" {
			input.GuestEmail = user.Email
		}
	}
	return input, true
}

func (api *API) quotePublicOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	input, ok := api.orderInput(w, r)
	if !ok {
		return
	}
	quote, err := api.ordering.Quote(r.Context(), pathParam(r, "venueID"), input)
	if err != nil {
		api.writeOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"quote": quote})
}

func (api *API) submitPublicOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	input, ok := api.orderInput(w, r)
	if !ok {
		return
	}
	order, err := api.ordering.Submit(r.Context(), pathParam(r, "venueID"), input)
	if err != nil {
		api.writeOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"order": order})
}

func (api *API) getPublicOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	order, err := api.ordering.GetPublic(r.Context(), pathParam(r, "venueID"), pathParam(r, "orderID"), r.Header.Get("X-Order-Key"))
	if err != nil {
		api.writeOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": order})
}

func (api *API) listCustomerOrders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	orders, err := api.ordering.ListCustomer(r.Context(), user.ID)
	if err != nil {
		api.writeOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

func (api *API) writeOrderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ordering.ErrPriceChanged):
		writeAPIError(w, http.StatusConflict, "price_changed", "The price changed. Review the current menu.")
	case errors.Is(err, ordering.ErrItemUnavailable):
		writeAPIError(w, http.StatusConflict, "item_unavailable", "A selected item is no longer available.")
	case errors.Is(err, appfault.ErrConflict):
		writeAPIError(w, http.StatusConflict, "idempotency_conflict", "This order key was already used for a different request.")
	case errors.Is(err, appfault.ErrInvalidInput):
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Check the order details.")
	case errors.Is(err, appfault.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "Order or venue not found.")
	default:
		api.logger.Error("public order request failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "The order could not be processed.")
	}
}
