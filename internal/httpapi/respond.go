package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/dinein"
	"pocket-mvp-backend/internal/modules/identity"
	"pocket-mvp-backend/internal/modules/reservations"
)

// reply is an error response: an HTTP status, a stable code the clients localize, and an English
// message for logs and API users.
type reply struct {
	status  int
	code    string
	message string
}

// errorRule maps a domain error to its reply. A rule without an error matches any error: it is
// the area's reply for failures nobody anticipated.
type errorRule struct {
	err   error
	reply reply
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var (
	replyInvalidRequest = reply{http.StatusBadRequest, "invalid_request", "Check the submitted data."}
	replyInternal       = reply{http.StatusInternalServerError, "internal_server_error", "The request could not be completed."}
)

// commonErrors apply after the area rules of every request.
var commonErrors = []errorRule{
	{identity.ErrUnauthorized, reply{http.StatusUnauthorized, "unauthorized", "Sign in to continue."}},
	{appfault.ErrForbidden, reply{http.StatusForbidden, "forbidden", "This role cannot perform this action."}},
	{appfault.ErrInvalidInput, reply{http.StatusUnprocessableEntity, "invalid_input", "Check the submitted data."}},
	{appfault.ErrNotFound, reply{http.StatusNotFound, "not_found", "The resource was not found or is unavailable."}},
	{appfault.ErrConflict, reply{http.StatusConflict, "conflict", "The resource already exists or is in use."}},
	{nil, replyInternal},
}

// authErrors: sign-up, sign-in and the account.
var authErrors = []errorRule{
	{identity.ErrInvalidInput, reply{http.StatusUnprocessableEntity, "invalid_input", "Check the form fields. The password must be 12 to 128 characters long."}},
	{identity.ErrEmailAlreadyExists, reply{http.StatusConflict, "account_exists", "An account with this email already exists."}},
	{identity.ErrInvalidCredentials, reply{http.StatusUnauthorized, "invalid_credentials", "Incorrect email or password."}},
	{identity.ErrInvalidCurrentPassword, reply{http.StatusUnprocessableEntity, "invalid_current_password", "The current password is incorrect."}},
	{identity.ErrTooManyAttempts, reply{http.StatusTooManyRequests, "too_many_attempts", "Too many attempts. Please try again in 15 minutes."}},
	{identity.ErrAvatarNotFound, reply{http.StatusNotFound, "avatar_not_found", "Photo not found."}},
}

// passwordResetErrors: the reset flow never tells whether an account exists.
var passwordResetErrors = []errorRule{
	{identity.ErrInvalidResetToken, reply{http.StatusUnprocessableEntity, "invalid_reset_token", "This link is invalid or has expired."}},
	{identity.ErrInvalidInput, reply{http.StatusUnprocessableEntity, "invalid_input", "Check the submitted data."}},
}

// reservationErrors: guest bookings.
var reservationErrors = []errorRule{
	{reservations.ErrPriceChanged, reply{http.StatusConflict, "price_changed", "The price changed. Review the current menu and confirm again."}},
	{reservations.ErrItemUnavailable, reply{http.StatusConflict, "item_unavailable", "A selected item is no longer available."}},
	{appfault.ErrConflict, reply{http.StatusConflict, "slot_unavailable", "This time is no longer available. Choose another slot."}},
	{reservations.ErrHoldExpired, reply{http.StatusConflict, "hold_expired", "The reservation hold expired. Choose a time again."}},
	{appfault.ErrInvalidInput, reply{http.StatusUnprocessableEntity, "invalid_input", "Check the booking details."}},
	{appfault.ErrNotFound, reply{http.StatusNotFound, "not_found", "Venue not found."}},
	{nil, reply{http.StatusInternalServerError, "internal_server_error", "The reservation could not be saved."}},
}

// publicVenueErrors: the public venue page in Dish.
var publicVenueErrors = []errorRule{
	{appfault.ErrInvalidInput, reply{http.StatusUnprocessableEntity, "invalid_input", "Invalid venue ID."}},
	{appfault.ErrNotFound, reply{http.StatusNotFound, "not_found", "Venue not found."}},
}

// customerReservationErrors: a signed-in guest's own bookings.
var customerReservationErrors = []errorRule{
	{appfault.ErrNotFound, reply{http.StatusNotFound, "not_found", "Reservation not found."}},
	{appfault.ErrInvalidInput, reply{http.StatusUnprocessableEntity, "invalid_input", "Invalid reservation ID."}},
}

// tableOrderErrors: guest orders and bills at a table.
var tableOrderErrors = []errorRule{
	{dinein.ErrQRInvalid, reply{http.StatusNotFound, "qr_invalid", "This table QR is no longer valid. Ask the staff for a new code."}},
	{dinein.ErrPriceChanged, reply{http.StatusConflict, "price_changed", "Review the updated menu prices."}},
	{dinein.ErrItemUnavailable, reply{http.StatusConflict, "item_unavailable", "A selected dish is no longer available."}},
	{dinein.ErrBillLocked, reply{http.StatusConflict, "bill_locked", "Ordering is currently unavailable. Contact the staff."}},
	{appfault.ErrInvalidInput, reply{http.StatusUnprocessableEntity, "invalid_input", "Check the order details."}},
	{appfault.ErrNotFound, reply{http.StatusNotFound, "not_found", "Order not found."}},
	{appfault.ErrConflict, reply{http.StatusConflict, "conflict", "This request was already used for a different order."}},
	{nil, reply{http.StatusInternalServerError, "internal_server_error", "The order could not be saved."}},
}

// writeError answers with the first rule that matches err: the area rules first, then the common
// ones. Unanticipated failures are logged; their details never reach the client.
func (api *API) writeError(w http.ResponseWriter, r *http.Request, err error, areaRules ...[]errorRule) {
	for _, rules := range append(areaRules, commonErrors) {
		for _, rule := range rules {
			if rule.err != nil && !errors.Is(err, rule.err) {
				continue
			}
			if rule.reply.status >= http.StatusInternalServerError {
				api.logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
			}
			writeReply(w, rule.reply)
			return
		}
	}
}

func writeReply(w http.ResponseWriter, reply reply) {
	writeAPIError(w, reply.status, reply.code, reply.message)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// failure is the reply for unanticipated errors of one request, when it deserves its own message.
func failure(message string) []errorRule {
	return []errorRule{{nil, reply{http.StatusInternalServerError, "internal_server_error", message}}}
}
