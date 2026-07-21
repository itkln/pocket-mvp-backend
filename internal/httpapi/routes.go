package httpapi

import "net/http"

func (api *API) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", api.health)
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /readyz", api.ready)
	mux.HandleFunc("GET /api/v1", api.serviceInfo)

	mux.HandleFunc("POST /api/v1/auth/register", api.register)
	mux.HandleFunc("POST /api/v1/auth/login", api.login)
	mux.HandleFunc("POST /api/v1/auth/logout", api.logout)
	mux.HandleFunc("GET /api/v1/auth/me", api.me)

	mux.HandleFunc("/api/v1/owner/venues", api.ownerVenues)
	mux.HandleFunc("/api/v1/owner/venues/{venueID}", api.ownerVenue)
	mux.HandleFunc("GET /api/v1/owner/venues/{venueID}/dashboard", api.ownerDashboard)
	mux.HandleFunc("/api/v1/owner/venues/{venueID}/categories", api.ownerCategories)
	mux.HandleFunc("/api/v1/owner/venues/{venueID}/categories/{resourceID}", api.ownerCategory)
	mux.HandleFunc("/api/v1/owner/venues/{venueID}/menu-items", api.ownerMenuItems)
	mux.HandleFunc("/api/v1/owner/venues/{venueID}/menu-items/{resourceID}", api.ownerMenuItem)
	mux.HandleFunc("/api/v1/owner/venues/{venueID}/staff", api.ownerStaff)
	mux.HandleFunc("/api/v1/owner/venues/{venueID}/staff/{resourceID}", api.ownerStaffMember)
	mux.HandleFunc("GET /api/v1/owner/venues/{venueID}/orders", api.ownerOrders)
	mux.HandleFunc("PATCH /api/v1/owner/venues/{venueID}/orders/{resourceID}", api.ownerOrder)
	mux.HandleFunc("GET /api/v1/owner/venues/{venueID}/reviews", api.ownerReviews)
	mux.HandleFunc("PATCH /api/v1/owner/venues/{venueID}/reviews/{resourceID}", api.ownerReview)
	mux.HandleFunc("GET /api/v1/owner/venues/{venueID}/payments", api.ownerPayments)
	mux.HandleFunc("/api/v1/owner/venues/{venueID}/floor-plan", api.ownerFloorPlan)
	mux.HandleFunc("/api/v1/owner/subscription", api.ownerSubscription)
	return mux
}
