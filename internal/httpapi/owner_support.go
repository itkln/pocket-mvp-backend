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
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "Проверьте введенные данные")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "Проверьте введенные данные")
		return false
	}
	return true
}

func (api *API) writeOwnerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, appfault.ErrInvalidInput):
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Проверьте введенные данные")
	case errors.Is(err, appfault.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "Объект не найден или недоступен")
	case errors.Is(err, appfault.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", "Такой объект уже существует или используется")
	default:
		api.logger.Error("owner request failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "Не удалось выполнить запрос")
	}
}
