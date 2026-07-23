package httpapi

import (
	"net/http"

	"pocket-mvp-backend/internal/modules/catalog"
)

func (api *API) listOwnerCategories(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID := pathParam(r, "venueID")
	items, err := api.catalog.ListCategories(r.Context(), user.ID, venueID)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": items})
}

func (api *API) createOwnerCategory(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input catalog.CategoryInput
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	item, err := api.catalog.CreateCategory(r.Context(), user.ID, pathParam(r, "venueID"), input)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"category": item})
}

func (api *API) updateOwnerCategory(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input catalog.CategoryInput
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	item, err := api.catalog.UpdateCategory(
		r.Context(),
		user.ID,
		pathParam(r, "venueID"),
		pathParam(r, "resourceID"),
		input,
	)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": item})
}

func (api *API) deleteOwnerCategory(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	if err := api.catalog.DeleteCategory(
		r.Context(),
		user.ID,
		pathParam(r, "venueID"),
		pathParam(r, "resourceID"),
	); err != nil {
		api.writeOwnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) reorderOwnerCategories(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input catalog.ReorderInput
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	if err := api.catalog.ReorderCategories(r.Context(), user.ID, pathParam(r, "venueID"), input.IDs); err != nil {
		api.writeOwnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) listOwnerMenuItems(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID := pathParam(r, "venueID")
	items, err := api.catalog.ListMenuItems(r.Context(), user.ID, venueID)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (api *API) createOwnerMenuItem(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input catalog.MenuItemInput
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	item, err := api.catalog.CreateMenuItem(r.Context(), user.ID, pathParam(r, "venueID"), input)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (api *API) updateOwnerMenuItem(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input catalog.MenuItemInput
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	item, err := api.catalog.UpdateMenuItem(
		r.Context(),
		user.ID,
		pathParam(r, "venueID"),
		pathParam(r, "resourceID"),
		input,
	)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (api *API) deleteOwnerMenuItem(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	if err := api.catalog.DeleteMenuItem(
		r.Context(),
		user.ID,
		pathParam(r, "venueID"),
		pathParam(r, "resourceID"),
	); err != nil {
		api.writeOwnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) reorderOwnerMenuItems(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input catalog.ReorderMenuItemsInput
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	if err := api.catalog.ReorderMenuItems(r.Context(), user.ID, pathParam(r, "venueID"), input.CategoryID, input.IDs); err != nil {
		api.writeOwnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
