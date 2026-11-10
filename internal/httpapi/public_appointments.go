package httpapi

import (
	"net/http"
	"pocket-mvp-backend/internal/appointments"
)

func (api *API) listPublicServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	items, err := appointments.New(appointments.NewPostgresRepository(api.database)).List(r.Context(), pathParam(r, "venueID"))
	if err != nil {
		api.writeReservationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": items})
}
func (api *API) getServiceAvailability(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	result, err := appointments.New(appointments.NewPostgresRepository(api.database)).Availability(r.Context(), pathParam(r, "venueID"), r.URL.Query().Get("service_id"), r.URL.Query().Get("date"))
	if err != nil {
		api.writeReservationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (api *API) createAppointment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input appointments.Input
	if !decodeOwnerJSON(w, r, &input) {
		return
	}
	if cookie, err := r.Cookie(api.sessionCookie); err == nil {
		user, err := api.identity.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "Sign in again.")
			return
		}
		input.CustomerID = user.ID
	}
	result, err := appointments.New(appointments.NewPostgresRepository(api.database)).Create(r.Context(), pathParam(r, "venueID"), input)
	if err != nil {
		api.writeReservationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"reservation": result})
}
