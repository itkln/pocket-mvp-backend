package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// The API has one group per audience. Public routes serve guests in Dish; every other group
// needs a session (requireUser), and its responses are never cached (noStore).
//
//   - /public    guests: venue pages, bookings, table QR orders and bills;
//   - /auth      sign-up, sign-in, password reset, and the signed-in user's own account;
//   - /customer  a signed-in guest's own bookings;
//   - /owner     the owner's workspace as a whole: their venues and the subscription;
//   - /staff     where a user works, and staff invitations;
//   - /venues/{venueID}  one venue's workspace. Whether the user may do something there is the
//     venue role's capability, checked by the module (access.Capability); the URL carries no role.
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

	router.Route("/api/v1", func(v1 chi.Router) {
		v1.Route("/public", api.publicRoutes)
		v1.Route("/auth", api.authRoutes)
		v1.Group(func(private chi.Router) {
			private.Use(noStore, api.requireUser)
			private.Route("/customer", api.customerRoutes)
			private.Route("/owner", api.ownerRoutes)
			private.Route("/staff", api.staffRoutes)
			private.Route("/venues/{venueID}", api.venueRoutes)
		})
	})
	return router
}

func (api *API) publicRoutes(public chi.Router) {
	public.Get("/venues/{venueID}", api.getPublicVenue)
	public.Get("/venues/{venueID}/menu", api.getPublicVenueMenu)
	public.Get("/venues/{venueID}/reviews", api.getPublicVenueReviews)
	public.Get("/venues/{venueID}/availability", api.getPublicVenueAvailability)
	public.Post("/venues/{venueID}/reservations", api.createPublicReservation)
	public.Post("/venues/{venueID}/reservations/holds", api.holdPublicReservation)
	public.Post("/venues/{venueID}/reservations/{reservationID}/confirm", api.confirmPublicReservation)
	public.Get("/venues/{venueID}/reservations/{reservationID}", api.getPublicReservation)

	public.Post("/table-qr/resolve", api.resolveTableQR)
	public.Post("/table-bill", api.currentTableBill)
	public.Post("/table-orders", api.createTableOrder)
	public.Post("/table-orders/lookup", api.lookupTableOrder)
	public.Post("/table-orders/history", api.tableOrderHistory)
}

func (api *API) authRoutes(auth chi.Router) {
	auth.Use(noStore)
	auth.Group(func(entry chi.Router) {
		entry.Use(limitBody(maxAuthBody))
		entry.Post("/register", api.register)
		entry.Post("/login", api.login)
		entry.Post("/logout", api.logout)
		entry.Post("/password-reset/request", api.requestPasswordReset)
		entry.Post("/password-reset/confirm", api.resetPassword)
	})
	auth.Group(func(account chi.Router) {
		account.Use(api.requireUser)
		account.Get("/me", api.me)
		account.Get("/me/avatar", api.avatar)
		account.Post("/me/avatar", api.updateAvatar)
		account.With(limitBody(maxAuthBody)).Patch("/me", api.updateProfile)
		account.With(limitBody(maxAuthBody)).Post("/password/change", api.changePassword)
		account.With(limitBody(maxAuthBody)).Post("/email/change", api.changeEmail)
	})
}

func (api *API) customerRoutes(customer chi.Router) {
	customer.Get("/reservations", api.listCustomerReservations)
	customer.Post("/reservations/{reservationID}/cancel", api.cancelCustomerReservation)
}

func (api *API) ownerRoutes(owner chi.Router) {
	owner.Get("/venues", api.listOwnerVenues)
	owner.Post("/venues", api.createOwnerVenue)
	owner.Get("/subscription", api.getOwnerSubscription)
	owner.Put("/subscription", api.updateOwnerSubscription)
}

func (api *API) staffRoutes(staff chi.Router) {
	staff.Get("/venues", api.listWorkplaces)
	staff.Post("/invitations/inspect", api.inspectStaffInvitation)
	staff.Post("/invitations/accept", api.acceptStaffInvitation)
}

func (api *API) venueRoutes(venue chi.Router) {
	// Settings: managers and the owner; deleting the venue: the owner.
	venue.Patch("/", api.updateVenue)
	venue.Delete("/", api.deleteVenue)
	venue.Get("/public-page", api.venuePublicPage)

	// Orders: the whole team.
	venue.Get("/orders", api.listOrders)
	venue.Get("/orders/{orderID}", api.getOrder)
	venue.Patch("/orders/{orderID}", api.updateOrderStatus)
	venue.Get("/order-alert-settings", api.getOrderAlertSettings)
	venue.Put("/order-alert-settings", api.saveOrderAlertSettings)

	// Menu: managers and the owner.
	venue.Get("/categories", api.listCategories)
	venue.Post("/categories", api.createCategory)
	venue.Put("/categories/order", api.reorderCategories)
	venue.Patch("/categories/{categoryID}", api.updateCategory)
	venue.Delete("/categories/{categoryID}", api.deleteCategory)
	venue.Get("/menu-items", api.listMenuItems)
	venue.Post("/menu-items", api.createMenuItem)
	venue.Put("/menu-items/order", api.reorderMenuItems)
	venue.Patch("/menu-items/{itemID}", api.updateMenuItem)
	venue.Delete("/menu-items/{itemID}", api.deleteMenuItem)

	// Floor: managers and the owner.
	venue.Get("/floor-plan", api.getFloorPlan)
	venue.Put("/floor-plan", api.updateFloorPlan)
	venue.Post("/table-qr", api.issueTableQR)
	venue.Get("/floor", api.getLiveFloor)
	venue.Post("/floor/reservations/{reservationID}/seat", api.seatReservation)
	venue.Post("/floor/walk-ins", api.seatWalkIn)
	venue.Post("/floor/release", api.releaseTable)

	// The rest: the owner.
	venue.Get("/dashboard", api.getDashboard)
	venue.Get("/team", api.listTeam)
	venue.Post("/team", api.createTeamMember)
	venue.Patch("/team/{memberID}", api.updateTeamMember)
	venue.Delete("/team/{memberID}", api.deleteTeamMember)
	venue.Post("/team/{memberID}/invite", api.resendTeamInvitation)
	venue.Get("/reviews", api.listReviews)
	venue.Patch("/reviews/{reviewID}", api.replyToReview)
	venue.Get("/payments", api.listPayments)
}
