package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"pocket-mvp-backend/internal/appfault"
)

const maxOwnerBody = 1 << 20

func decodeOwnerJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxOwnerBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "Check the submitted data.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "Check the submitted data.")
		return false
	}
	return true
}

func (api *API) writeOwnerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, appfault.ErrForbidden):
		writeAPIError(w, http.StatusForbidden, "forbidden", "This role cannot perform this action.")
	case errors.Is(err, appfault.ErrInvalidInput):
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Check the submitted data.")
	case errors.Is(err, appfault.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "The resource was not found or is unavailable.")
	case errors.Is(err, appfault.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", "The resource already exists or is in use.")
	default:
		api.logger.Error("owner request failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "The request could not be completed.")
	}
}
