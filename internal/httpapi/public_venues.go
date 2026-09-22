package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/venues"
)

func (api *API) searchPublicVenues(w http.ResponseWriter, r *http.Request) {
	page, ok := positiveQueryInt(r, "page", 1)
	if !ok {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Проверьте параметры каталога")
		return
	}
	pageSize, ok := positiveQueryInt(r, "page_size", 24)
	if !ok {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Проверьте параметры каталога")
		return
	}
	var ids []string
	if raw, present := r.URL.Query()["ids"]; present {
		if len(raw) != 1 || strings.TrimSpace(raw[0]) == "" {
			writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Проверьте параметры каталога")
			return
		}
		ids = strings.Split(raw[0], ",")
	}
	result, err := api.venues.SearchPublished(r.Context(), venues.PublicVenueQuery{
		City: r.URL.Query().Get("city"), Search: r.URL.Query().Get("q"),
		Category: r.URL.Query().Get("category"), Filter: r.URL.Query().Get("filter"),
		IDs: ids, Page: page, PageSize: pageSize,
	})
	if err != nil {
		if errors.Is(err, appfault.ErrInvalidInput) {
			writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Проверьте параметры каталога")
			return
		}
		api.logger.Error("public venue search failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "Не удалось загрузить каталог")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *API) getPublicVenue(w http.ResponseWriter, r *http.Request) {
	item, err := api.venues.GetPublished(r.Context(), pathParam(r, "venueID"))
	if err != nil {
		switch {
		case errors.Is(err, appfault.ErrInvalidInput):
			writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Некорректный ID заведения")
		case errors.Is(err, appfault.ErrNotFound):
			writeAPIError(w, http.StatusNotFound, "not_found", "Заведение не найдено")
		default:
			api.logger.Error("public venue details failed", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "Не удалось загрузить заведение")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"venue": item})
}

func positiveQueryInt(r *http.Request, name string, fallback int) (int, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed > 0
}
