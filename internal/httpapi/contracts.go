package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/buildinfo"
	"pocket-mvp-backend/internal/modules/billing"
	"pocket-mvp-backend/internal/modules/catalog"
	"pocket-mvp-backend/internal/modules/feedback"
	"pocket-mvp-backend/internal/modules/identity"
	"pocket-mvp-backend/internal/modules/ordering"
	"pocket-mvp-backend/internal/modules/reporting"
	"pocket-mvp-backend/internal/modules/venues"
	"pocket-mvp-backend/internal/modules/workforce"
)

type IdentityService interface {
	Register(context.Context, identity.RegisterInput) (identity.User, identity.Session, error)
	Login(context.Context, identity.LoginInput) (identity.User, identity.Session, error)
	Authenticate(context.Context, string) (identity.User, error)
	Logout(context.Context, string) error
}

type VenueService interface {
	List(context.Context, string) ([]venues.Venue, error)
	Create(context.Context, string, venues.Input) (venues.Venue, error)
	Update(context.Context, string, string, venues.Input) (venues.Venue, error)
	Delete(context.Context, string, string) error
}

type CatalogService interface {
	ListCategories(context.Context, string, string) ([]catalog.Category, error)
	CreateCategory(context.Context, string, string, catalog.CategoryInput) (catalog.Category, error)
	UpdateCategory(context.Context, string, string, string, catalog.CategoryInput) (catalog.Category, error)
	DeleteCategory(context.Context, string, string, string) error
	ListMenuItems(context.Context, string, string) ([]catalog.MenuItem, error)
	CreateMenuItem(context.Context, string, string, catalog.MenuItemInput) (catalog.MenuItem, error)
	UpdateMenuItem(context.Context, string, string, string, catalog.MenuItemInput) (catalog.MenuItem, error)
	DeleteMenuItem(context.Context, string, string, string) error
}

type WorkforceService interface {
	List(context.Context, string, string) ([]workforce.StaffMember, error)
	Create(context.Context, string, string, workforce.StaffInput) (workforce.StaffMember, error)
	Update(context.Context, string, string, string, workforce.StaffInput) (workforce.StaffMember, error)
	Delete(context.Context, string, string, string) error
}

type OrderingService interface {
	List(context.Context, string, string) ([]ordering.Order, error)
	UpdateStatus(context.Context, string, string, string, string) (ordering.Order, error)
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
	Database       *pgxpool.Pool
	Logger         *slog.Logger
	AllowedOrigins []string
	Build          buildinfo.Info
	Identity       IdentityService
	Venues         VenueService
	Catalog        CatalogService
	Workforce      WorkforceService
	Ordering       OrderingService
	Feedback       FeedbackService
	Billing        BillingService
	FloorPlan      FloorPlanService
	Reporting      ReportingService
	SessionCookie  string
	SessionSecure  bool
}

type API struct {
	database       *pgxpool.Pool
	logger         *slog.Logger
	allowedOrigins []string
	build          buildinfo.Info
	startedAt      time.Time
	identity       IdentityService
	venues         VenueService
	catalog        CatalogService
	workforce      WorkforceService
	ordering       OrderingService
	feedback       FeedbackService
	billing        BillingService
	floorPlan      FloorPlanService
	reporting      ReportingService
	sessionCookie  string
	sessionSecure  bool
}
