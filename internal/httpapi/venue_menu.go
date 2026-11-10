package httpapi

import (
	"net/http"

	"pocket-mvp-backend/internal/modules/catalog"
)

func (api *API) listCategories(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	venueID := pathParam(r, "venueID")
	items, err := api.catalog.ListCategories(r.Context(), user.ID, venueID)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": items})
}

func (api *API) createCategory(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input catalog.CategoryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.catalog.CreateCategory(r.Context(), user.ID, pathParam(r, "venueID"), input)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"category": item})
}

func (api *API) updateCategory(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input catalog.CategoryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.catalog.UpdateCategory(
		r.Context(),
		user.ID,
		pathParam(r, "venueID"),
		pathParam(r, "categoryID"),
		input,
	)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": item})
}

func (api *API) deleteCategory(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if err := api.catalog.DeleteCategory(
		r.Context(),
		user.ID,
		pathParam(r, "venueID"),
		pathParam(r, "categoryID"),
	); err != nil {
		api.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) reorderCategories(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input catalog.ReorderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := api.catalog.ReorderCategories(r.Context(), user.ID, pathParam(r, "venueID"), input.IDs); err != nil {
		api.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) listMenuItems(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	venueID := pathParam(r, "venueID")
	items, err := api.catalog.ListMenuItems(r.Context(), user.ID, venueID)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (api *API) createMenuItem(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input catalog.MenuItemInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.catalog.CreateMenuItem(r.Context(), user.ID, pathParam(r, "venueID"), input)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (api *API) updateMenuItem(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input catalog.MenuItemInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := api.catalog.UpdateMenuItem(
		r.Context(),
		user.ID,
		pathParam(r, "venueID"),
		pathParam(r, "itemID"),
		input,
	)
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (api *API) deleteMenuItem(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if err := api.catalog.DeleteMenuItem(
		r.Context(),
		user.ID,
		pathParam(r, "venueID"),
		pathParam(r, "itemID"),
	); err != nil {
		api.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) reorderMenuItems(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var input catalog.ReorderMenuItemsInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := api.catalog.ReorderMenuItems(r.Context(), user.ID, pathParam(r, "venueID"), input.CategoryID, input.IDs); err != nil {
		api.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
