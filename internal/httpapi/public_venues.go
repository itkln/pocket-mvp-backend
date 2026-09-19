package httpapi

import (
	"errors"
	"net/http"
	"strconv"

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
	result, err := api.venues.SearchPublished(r.Context(), venues.PublicVenueQuery{
		City: r.URL.Query().Get("city"), Search: r.URL.Query().Get("q"),
		Category: r.URL.Query().Get("category"), Filter: r.URL.Query().Get("filter"),
		Page: page, PageSize: pageSize,
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

func positiveQueryInt(r *http.Request, name string, fallback int) (int, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed > 0
}
