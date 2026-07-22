package httpapi

import (
	"net/http"

	"pocket-mvp-backend/internal/modules/catalog"
)

func (api *API) ownerCategories(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID := r.PathValue("venueID")
	switch r.Method {
	case http.MethodGet:
		items, err := api.catalog.ListCategories(r.Context(), user.ID, venueID)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"categories": items})
	case http.MethodPost:
		var input catalog.CategoryInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.catalog.CreateCategory(r.Context(), user.ID, venueID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"category": item})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (api *API) ownerCategory(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID, categoryID := r.PathValue("venueID"), r.PathValue("resourceID")
	switch r.Method {
	case http.MethodPatch:
		var input catalog.CategoryInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.catalog.UpdateCategory(r.Context(), user.ID, venueID, categoryID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"category": item})
	case http.MethodDelete:
		if err := api.catalog.DeleteCategory(r.Context(), user.ID, venueID, categoryID); err != nil {
			api.writeOwnerError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (api *API) ownerCategoryOrder(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input catalog.ReorderInput
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	if err := api.catalog.ReorderCategories(r.Context(), user.ID, r.PathValue("venueID"), input.IDs); err != nil {
		api.writeOwnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) ownerMenuItems(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID := r.PathValue("venueID")
	switch r.Method {
	case http.MethodGet:
		items, err := api.catalog.ListMenuItems(r.Context(), user.ID, venueID)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var input catalog.MenuItemInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.catalog.CreateMenuItem(r.Context(), user.ID, venueID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"item": item})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (api *API) ownerMenuItem(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID, itemID := r.PathValue("venueID"), r.PathValue("resourceID")
	switch r.Method {
	case http.MethodPatch:
		var input catalog.MenuItemInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.catalog.UpdateMenuItem(r.Context(), user.ID, venueID, itemID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"item": item})
	case http.MethodDelete:
		if err := api.catalog.DeleteMenuItem(r.Context(), user.ID, venueID, itemID); err != nil {
			api.writeOwnerError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (api *API) ownerMenuItemOrder(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input catalog.ReorderMenuItemsInput
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	if err := api.catalog.ReorderMenuItems(r.Context(), user.ID, r.PathValue("venueID"), input.CategoryID, input.IDs); err != nil {
		api.writeOwnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
