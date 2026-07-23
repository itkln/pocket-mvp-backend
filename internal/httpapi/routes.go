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
		router.Get("/venues", api.ownerVenues)
		router.Post("/venues", api.ownerVenues)
		router.Get("/venues/{venueID}", api.ownerVenue)
		router.Patch("/venues/{venueID}", api.ownerVenue)
		router.Delete("/venues/{venueID}", api.ownerVenue)
		router.Get("/venues/{venueID}/dashboard", api.ownerDashboard)

		router.Get("/venues/{venueID}/categories", api.ownerCategories)
		router.Post("/venues/{venueID}/categories", api.ownerCategories)
		router.Put("/venues/{venueID}/categories/order", api.ownerCategoryOrder)
		router.Patch("/venues/{venueID}/categories/{resourceID}", api.ownerCategory)
		router.Delete("/venues/{venueID}/categories/{resourceID}", api.ownerCategory)

		router.Get("/venues/{venueID}/menu-items", api.ownerMenuItems)
		router.Post("/venues/{venueID}/menu-items", api.ownerMenuItems)
		router.Put("/venues/{venueID}/menu-items/order", api.ownerMenuItemOrder)
		router.Patch("/venues/{venueID}/menu-items/{resourceID}", api.ownerMenuItem)
		router.Delete("/venues/{venueID}/menu-items/{resourceID}", api.ownerMenuItem)

		router.Get("/venues/{venueID}/staff", api.ownerStaff)
		router.Post("/venues/{venueID}/staff", api.ownerStaff)
		router.Patch("/venues/{venueID}/staff/{resourceID}", api.ownerStaffMember)
		router.Delete("/venues/{venueID}/staff/{resourceID}", api.ownerStaffMember)

		router.Get("/venues/{venueID}/orders", api.ownerOrders)
		router.Patch("/venues/{venueID}/orders/{resourceID}", api.ownerOrder)
		router.Get("/venues/{venueID}/reviews", api.ownerReviews)
		router.Patch("/venues/{venueID}/reviews/{resourceID}", api.ownerReview)
		router.Get("/venues/{venueID}/payments", api.ownerPayments)
		router.Get("/venues/{venueID}/floor-plan", api.ownerFloorPlan)
		router.Put("/venues/{venueID}/floor-plan", api.ownerFloorPlan)
		router.Get("/subscription", api.ownerSubscription)
		router.Put("/subscription", api.ownerSubscription)
	})

	return router
}

func pathParam(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}
