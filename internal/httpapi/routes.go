package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (api *API) routes() http.Handler {
	router := chi.NewRouter()
	router.Use(api.recoverPanic)
	router.Use(api.requestLogger)
	router.Use(api.securityHeaders)
	router.Use(api.cors)

	router.Get("/health", api.health)
	router.Get("/healthz", api.health)
	router.Get("/readyz", api.ready)
	router.Get("/api/v1", api.serviceInfo)
	router.Get("/api/v1/public/venues", api.searchPublicVenues)
	router.Post("/api/v1/public/table-qr/resolve", api.resolveTableQR)
	router.Post("/api/v1/public/table-bill", api.currentTableBill)
	router.Post("/api/v1/staff/invitations/inspect", api.inspectStaffInvitation)
	router.Post("/api/v1/staff/invitations/accept", api.acceptStaffInvitation)
	router.Get("/api/v1/staff/venues", api.listStaffVenues)
	router.Get("/api/v1/staff/venues/{venueID}/order-alert-settings", api.getOrderAlertSettings)
	router.Get("/api/v1/staff/venues/{venueID}/orders", api.listOwnerOrders)
	router.Get("/api/v1/staff/venues/{venueID}/orders/{resourceID}", api.getOwnerOrder)
	router.Patch("/api/v1/staff/venues/{venueID}/orders/{resourceID}", api.updateOwnerOrderStatus)
	router.Post("/api/v1/public/table-orders", api.createTableOrder)
	router.Post("/api/v1/public/table-orders/lookup", api.lookupTableOrder)
	router.Post("/api/v1/public/table-orders/history", api.tableOrderHistory)
	router.Get("/api/v1/public/venues/{venueID}", api.getPublicVenue)
	router.Get("/api/v1/public/venues/{venueID}/menu", api.getPublicVenueMenu)
	router.Get("/api/v1/public/venues/{venueID}/reviews", api.getPublicVenueReviews)
	router.Get("/api/v1/public/venues/{venueID}/availability", api.getPublicVenueAvailability)
	router.Post("/api/v1/public/venues/{venueID}/reservations", api.createPublicReservation)
	router.Post("/api/v1/public/venues/{venueID}/reservations/holds", api.holdPublicReservation)
	router.Post("/api/v1/public/venues/{venueID}/reservations/{reservationID}/confirm", api.confirmPublicReservation)
	router.Get("/api/v1/public/venues/{venueID}/reservations/{reservationID}", api.getPublicReservation)
	router.Get("/api/v1/spots/me/saved-venues", api.listSavedVenues)
	router.Put("/api/v1/spots/me/saved-venues/{venueID}", api.addSavedVenue)
	router.Delete("/api/v1/spots/me/saved-venues/{venueID}", api.removeSavedVenue)
	router.Get("/api/v1/spots/me/preorders", api.listCustomerPreorders)
	router.Get("/api/v1/spots/me/reservations", api.listCustomerReservations)
	router.Post("/api/v1/spots/me/reservations/{reservationID}/cancel", api.cancelCustomerReservation)

	router.Route("/api/v1/auth", func(router chi.Router) {
		router.Post("/register", api.register)
		router.Post("/login", api.login)
		router.Post("/logout", api.logout)
		router.Get("/me", api.me)
		router.Patch("/me", api.updateProfile)
		router.Get("/me/avatar", api.avatar)
		router.Post("/me/avatar", api.updateAvatar)
		router.Post("/password-reset/request", api.requestPasswordReset)
		router.Post("/password-reset/confirm", api.resetPassword)
		router.Post("/password/change", api.changePassword)
		router.Post("/email/change", api.changeEmail)
	})

	router.Route("/api/v1/owner", func(router chi.Router) {
		router.Get("/venues", api.listOwnerVenues)
		router.Post("/venues", api.createOwnerVenue)
		router.Get("/venues/{venueID}/public-page", api.ownerPublicPage)
		router.Patch("/venues/{venueID}", api.updateOwnerVenue)
		router.Delete("/venues/{venueID}", api.deleteOwnerVenue)
		router.Get("/venues/{venueID}/dashboard", api.getOwnerDashboard)

		router.Get("/venues/{venueID}/categories", api.listOwnerCategories)
		router.Post("/venues/{venueID}/categories", api.createOwnerCategory)
		router.Put("/venues/{venueID}/categories/order", api.reorderOwnerCategories)
		router.Patch("/venues/{venueID}/categories/{resourceID}", api.updateOwnerCategory)
		router.Delete("/venues/{venueID}/categories/{resourceID}", api.deleteOwnerCategory)

		router.Get("/venues/{venueID}/menu-items", api.listOwnerMenuItems)
		router.Post("/venues/{venueID}/menu-items", api.createOwnerMenuItem)
		router.Put("/venues/{venueID}/menu-items/order", api.reorderOwnerMenuItems)
		router.Patch("/venues/{venueID}/menu-items/{resourceID}", api.updateOwnerMenuItem)
		router.Delete("/venues/{venueID}/menu-items/{resourceID}", api.deleteOwnerMenuItem)

		router.Get("/venues/{venueID}/staff", api.listOwnerStaff)
		router.Post("/venues/{venueID}/staff", api.createOwnerStaffMember)
		router.Post("/venues/{venueID}/staff/{resourceID}/invite", api.resendStaffInvitation)
		router.Patch("/venues/{venueID}/staff/{resourceID}", api.updateOwnerStaffMember)
		router.Delete("/venues/{venueID}/staff/{resourceID}", api.deleteOwnerStaffMember)

		router.Get("/venues/{venueID}/order-alert-settings", api.getOrderAlertSettings)
		router.Put("/venues/{venueID}/order-alert-settings", api.saveOrderAlertSettings)
		router.Get("/venues/{venueID}/orders", api.listOwnerOrders)
		router.Get("/venues/{venueID}/orders/{resourceID}", api.getOwnerOrder)
		router.Patch("/venues/{venueID}/orders/{resourceID}", api.updateOwnerOrderStatus)
		router.Get("/venues/{venueID}/reviews", api.listOwnerReviews)
		router.Patch("/venues/{venueID}/reviews/{resourceID}", api.replyToOwnerReview)
		router.Get("/venues/{venueID}/payments", api.listOwnerPayments)
		router.Get("/venues/{venueID}/floor-plan", api.getOwnerFloorPlan)
		router.Post("/venues/{venueID}/table-qr", api.issueTableQR)
		router.Put("/venues/{venueID}/floor-plan", api.updateOwnerFloorPlan)
		router.Get("/subscription", api.getOwnerSubscription)
		router.Put("/subscription", api.updateOwnerSubscription)
	})

	return router
}

func pathParam(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}
