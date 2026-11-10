package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/buildinfo"
	"pocket-mvp-backend/internal/modules/availability"
	"pocket-mvp-backend/internal/modules/billing"
	"pocket-mvp-backend/internal/modules/catalog"
	"pocket-mvp-backend/internal/modules/dinein"
	"pocket-mvp-backend/internal/modules/feedback"
	"pocket-mvp-backend/internal/modules/identity"
	"pocket-mvp-backend/internal/modules/ordering"
	"pocket-mvp-backend/internal/modules/reporting"
	"pocket-mvp-backend/internal/modules/reservations"
	"pocket-mvp-backend/internal/modules/venues"
	"pocket-mvp-backend/internal/modules/workforce"
)

type IdentityService interface {
	Register(context.Context, identity.RegisterInput) (identity.User, identity.Session, error)
	Login(context.Context, identity.LoginInput) (identity.User, identity.Session, error)
	Authenticate(context.Context, string) (identity.User, error)
	UpdateProfile(context.Context, string, identity.UpdateProfileInput) (identity.User, error)
	Logout(context.Context, string) error
	RequestPasswordReset(context.Context, identity.PasswordResetRequest) error
	ResetPassword(context.Context, identity.PasswordResetConfirmation) error
	ChangePassword(context.Context, identity.ChangePasswordInput) error
	ChangeEmail(context.Context, identity.ChangeEmailInput) (identity.User, error)
	UpdateAvatar(context.Context, string, string, []byte) (identity.User, error)
	Avatar(context.Context, string) (identity.Avatar, error)
}

type VenueService interface {
	List(context.Context, string) ([]venues.Venue, error)
	ListWorkplaces(context.Context, string) ([]venues.Venue, error)
	SearchPublished(context.Context, venues.PublicVenueQuery) (venues.PublicVenuePage, error)
	GetPublished(context.Context, string) (venues.PublicVenue, error)
	ListPublishedMenu(context.Context, string) ([]venues.PublicMenuCategory, error)
	ListPublishedReviews(context.Context, string) ([]venues.PublicReview, error)
	Create(context.Context, string, venues.Input) (venues.Venue, error)
	Update(context.Context, string, string, venues.Input) (venues.Venue, error)
	Delete(context.Context, string, string) error
}

type AvailabilityService interface {
	Get(context.Context, string, string, int) (availability.Result, error)
}

type ReservationService interface {
	ListCustomer(context.Context, string) ([]reservations.CustomerReservation, error)
	CancelCustomer(context.Context, string, string) (reservations.CustomerReservation, error)
	Create(context.Context, string, reservations.CreateInput) (reservations.Result, error)
	Hold(context.Context, string, reservations.CreateInput) (reservations.HoldResult, error)
	Confirm(context.Context, string, string, reservations.CreateInput) (reservations.Result, error)
	GetConfirmed(context.Context, string, string, string) (reservations.Result, error)
}

type CatalogService interface {
	ListCategories(context.Context, string, string) ([]catalog.Category, error)
	CreateCategory(context.Context, string, string, catalog.CategoryInput) (catalog.Category, error)
	UpdateCategory(context.Context, string, string, string, catalog.CategoryInput) (catalog.Category, error)
	DeleteCategory(context.Context, string, string, string) error
	ReorderCategories(context.Context, string, string, []string) error
	ListMenuItems(context.Context, string, string) ([]catalog.MenuItem, error)
	CreateMenuItem(context.Context, string, string, catalog.MenuItemInput) (catalog.MenuItem, error)
	UpdateMenuItem(context.Context, string, string, string, catalog.MenuItemInput) (catalog.MenuItem, error)
	DeleteMenuItem(context.Context, string, string, string) error
	ReorderMenuItems(context.Context, string, string, string, []string) error
}

type WorkforceService interface {
	SendInvitation(context.Context, string, string, string) error
	Invitation(context.Context, string, string, string, bool) (workforce.Invitation, error)
	List(context.Context, string, string) ([]workforce.StaffMember, error)
	Create(context.Context, string, string, workforce.StaffInput) (workforce.StaffMember, error)
	Update(context.Context, string, string, string, workforce.StaffInput) (workforce.StaffMember, error)
	Delete(context.Context, string, string, string) error
}

type OrderingService interface {
	GetAlertSettings(context.Context, string, string) (ordering.AlertSettings, error)
	SaveAlertSettings(context.Context, string, string, ordering.AlertSettings) (ordering.AlertSettings, error)
	ListPage(context.Context, string, string, ordering.Query) (ordering.Page, error)
	Get(context.Context, string, string, string) (ordering.Order, error)
	List(context.Context, string, string) ([]ordering.Order, error)
	UpdateStatus(context.Context, string, string, string, string, string, ...ordering.StatusChange) (ordering.Order, error)
}

type DineInService interface {
	CurrentBill(context.Context, string) (*dinein.Bill, error)
	OwnerQRCode(context.Context, string, string, string, bool) (dinein.QRCode, error)
	Resolve(context.Context, string) (dinein.Table, error)
	Create(context.Context, dinein.CreateInput) (dinein.Order, error)
	Lookup(context.Context, string, string, ...dinein.HistoryInput) (dinein.Order, error)
	History(context.Context, dinein.HistoryInput) (dinein.HistoryPage, error)
}

type FeedbackService interface {
	List(context.Context, string, string) ([]feedback.Review, error)
	Reply(context.Context, string, string, string, string) (feedback.Review, error)
}

type BillingService interface {
	ListPayments(context.Context, string, string) ([]billing.Payment, error)
	GetSubscription(context.Context, string) (*billing.Subscription, error)
	UpsertSubscription(context.Context, string, billing.SubscriptionInput) (billing.Subscription, error)
}

type FloorPlanService interface {
	Get(context.Context, string, string) (json.RawMessage, error)
	Save(context.Context, string, string, json.RawMessage) (json.RawMessage, error)
}

type ReportingService interface {
	Dashboard(context.Context, string, string) (reporting.Dashboard, error)
}

type Dependencies struct {
	DishBaseURL    string
	Database       *pgxpool.Pool
	Logger         *slog.Logger
	AllowedOrigins []string
	Build          buildinfo.Info
	Identity       IdentityService
	Venues         VenueService
	Availability   AvailabilityService
	Reservations   ReservationService
	Catalog        CatalogService
	Workforce      WorkforceService
	Ordering       OrderingService
	DineIn         DineInService
	Feedback       FeedbackService
	Billing        BillingService
	FloorPlan      FloorPlanService
	Reporting      ReportingService
	SessionCookie  string
	SessionSecure  bool
}

type API struct {
	dishBaseURL    string
	database       *pgxpool.Pool
	logger         *slog.Logger
	allowedOrigins []string
	build          buildinfo.Info
	startedAt      time.Time
	identity       IdentityService
	venues         VenueService
	availability   AvailabilityService
	reservations   ReservationService
	catalog        CatalogService
	workforce      WorkforceService
	ordering       OrderingService
	dineIn         DineInService
	feedback       FeedbackService
	billing        BillingService
	floorPlan      FloorPlanService
	reporting      ReportingService
	sessionCookie  string
	sessionSecure  bool
}
