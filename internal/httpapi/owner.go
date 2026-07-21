package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"pocket-mvp-backend/internal/auth"
	"pocket-mvp-backend/internal/owner"
)

const maxOwnerBody = 1 << 20

func (api *API) currentUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	cookie, err := r.Cookie(api.sessionCookie)
	if err != nil {
		api.writeAuthError(w, auth.ErrUnauthorized)
		return auth.User{}, false
	}
	user, err := api.auth.Authenticate(r.Context(), cookie.Value)
	if err != nil {
		api.writeAuthError(w, err)
		return auth.User{}, false
	}
	return user, true
}

func (api *API) ownerVenues(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := api.owner.ListVenues(r.Context(), user.ID)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"venues": items})
	case http.MethodPost:
		var input owner.VenueInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.owner.CreateVenue(r.Context(), user.ID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"venue": item})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (api *API) ownerVenue(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID := r.PathValue("venueID")
	switch r.Method {
	case http.MethodPatch:
		var input owner.VenueInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.owner.UpdateVenue(r.Context(), user.ID, venueID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"venue": item})
	case http.MethodDelete:
		if err := api.owner.DeleteVenue(r.Context(), user.ID, venueID); err != nil {
			api.writeOwnerError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (api *API) ownerDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	result, err := api.owner.Dashboard(r.Context(), user.ID, r.PathValue("venueID"))
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dashboard": result})
}

func (api *API) ownerCategories(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID := r.PathValue("venueID")
	if r.Method == http.MethodGet {
		items, err := api.owner.ListCategories(r.Context(), user.ID, venueID)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"categories": items})
		return
	}
	if r.Method == http.MethodPost {
		var input owner.CategoryInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.owner.CreateCategory(r.Context(), user.ID, venueID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"category": item})
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (api *API) ownerCategory(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID, id := r.PathValue("venueID"), r.PathValue("resourceID")
	if r.Method == http.MethodPatch {
		var input owner.CategoryInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.owner.UpdateCategory(r.Context(), user.ID, venueID, id, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"category": item})
		return
	}
	if r.Method == http.MethodDelete {
		if err := api.owner.DeleteCategory(r.Context(), user.ID, venueID, id); err != nil {
			api.writeOwnerError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (api *API) ownerMenuItems(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID := r.PathValue("venueID")
	if r.Method == http.MethodGet {
		items, err := api.owner.ListMenuItems(r.Context(), user.ID, venueID)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	if r.Method == http.MethodPost {
		var input owner.MenuItemInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.owner.CreateMenuItem(r.Context(), user.ID, venueID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"item": item})
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (api *API) ownerMenuItem(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID, id := r.PathValue("venueID"), r.PathValue("resourceID")
	if r.Method == http.MethodPatch {
		var input owner.MenuItemInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.owner.UpdateMenuItem(r.Context(), user.ID, venueID, id, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"item": item})
		return
	}
	if r.Method == http.MethodDelete {
		if err := api.owner.DeleteMenuItem(r.Context(), user.ID, venueID, id); err != nil {
			api.writeOwnerError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (api *API) ownerStaff(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID := r.PathValue("venueID")
	if r.Method == http.MethodGet {
		items, err := api.owner.ListStaff(r.Context(), user.ID, venueID)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"staff": items})
		return
	}
	if r.Method == http.MethodPost {
		var input owner.StaffInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.owner.CreateStaff(r.Context(), user.ID, venueID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"staff_member": item})
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (api *API) ownerStaffMember(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID, id := r.PathValue("venueID"), r.PathValue("resourceID")
	if r.Method == http.MethodPatch {
		var input owner.StaffInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.owner.UpdateStaff(r.Context(), user.ID, venueID, id, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"staff_member": item})
		return
	}
	if r.Method == http.MethodDelete {
		if err := api.owner.DeleteStaff(r.Context(), user.ID, venueID, id); err != nil {
			api.writeOwnerError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (api *API) ownerOrders(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	items, err := api.owner.ListOrders(r.Context(), user.ID, r.PathValue("venueID"))
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": items})
}

func (api *API) ownerOrder(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	item, err := api.owner.UpdateOrderStatus(r.Context(), user.ID, r.PathValue("venueID"), r.PathValue("resourceID"), input.Status)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": item})
}

func (api *API) ownerReviews(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	items, err := api.owner.ListReviews(r.Context(), user.ID, r.PathValue("venueID"))
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": items})
}

func (api *API) ownerReview(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		OwnerReply string `json:"owner_reply"`
	}
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	item, err := api.owner.ReplyReview(r.Context(), user.ID, r.PathValue("venueID"), r.PathValue("resourceID"), input.OwnerReply)
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"review": item})
}

func (api *API) ownerPayments(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	items, err := api.owner.ListPayments(r.Context(), user.ID, r.PathValue("venueID"))
	if err != nil {
		api.writeOwnerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"payments": items})
}

func (api *API) ownerFloorPlan(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	venueID := r.PathValue("venueID")
	if r.Method == http.MethodGet {
		plan, err := api.owner.GetFloorPlan(r.Context(), user.ID, venueID)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"floor_plan": plan})
		return
	}
	if r.Method == http.MethodPut {
		var input struct {
			FloorPlan json.RawMessage `json:"floor_plan"`
		}
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		plan, err := api.owner.SaveFloorPlan(r.Context(), user.ID, venueID, input.FloorPlan)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"floor_plan": plan})
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (api *API) ownerSubscription(w http.ResponseWriter, r *http.Request) {
	user, ok := api.currentUser(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		item, err := api.owner.GetSubscription(r.Context(), user.ID)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"subscription": item})
		return
	}
	if r.Method == http.MethodPut {
		var input owner.SubscriptionInput
		if !decodeOwnerJSON(w, r, &input) {
			return
		}
		item, err := api.owner.UpsertSubscription(r.Context(), user.ID, input)
		if err != nil {
			api.writeOwnerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"subscription": item})
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

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
	case errors.Is(err, owner.ErrInvalidInput):
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_input", "Проверьте введенные данные")
	case errors.Is(err, owner.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "Объект не найден или недоступен")
	case errors.Is(err, owner.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", "Такой объект уже существует или используется")
	default:
		api.logger.Error("owner request failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal_server_error", "Не удалось выполнить запрос")
	}
}
