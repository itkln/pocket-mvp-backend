package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/auth"
	"pocket-mvp-backend/internal/buildinfo"
	"pocket-mvp-backend/internal/owner"
)

type AuthService interface {
	Register(context.Context, auth.RegisterInput) (auth.User, auth.Session, error)
	Login(context.Context, auth.LoginInput) (auth.User, auth.Session, error)
	Authenticate(context.Context, string) (auth.User, error)
	Logout(context.Context, string) error
}

type OwnerService interface {
	ListVenues(context.Context, string) ([]owner.Venue, error)
	CreateVenue(context.Context, string, owner.VenueInput) (owner.Venue, error)
	UpdateVenue(context.Context, string, string, owner.VenueInput) (owner.Venue, error)
	DeleteVenue(context.Context, string, string) error
	Dashboard(context.Context, string, string) (owner.Dashboard, error)
	ListCategories(context.Context, string, string) ([]owner.Category, error)
	CreateCategory(context.Context, string, string, owner.CategoryInput) (owner.Category, error)
	UpdateCategory(context.Context, string, string, string, owner.CategoryInput) (owner.Category, error)
	DeleteCategory(context.Context, string, string, string) error
	ListMenuItems(context.Context, string, string) ([]owner.MenuItem, error)
	CreateMenuItem(context.Context, string, string, owner.MenuItemInput) (owner.MenuItem, error)
	UpdateMenuItem(context.Context, string, string, string, owner.MenuItemInput) (owner.MenuItem, error)
	DeleteMenuItem(context.Context, string, string, string) error
	ListStaff(context.Context, string, string) ([]owner.StaffMember, error)
	CreateStaff(context.Context, string, string, owner.StaffInput) (owner.StaffMember, error)
	UpdateStaff(context.Context, string, string, string, owner.StaffInput) (owner.StaffMember, error)
	DeleteStaff(context.Context, string, string, string) error
	ListOrders(context.Context, string, string) ([]owner.Order, error)
	UpdateOrderStatus(context.Context, string, string, string, string) (owner.Order, error)
	ListReviews(context.Context, string, string) ([]owner.Review, error)
	ReplyReview(context.Context, string, string, string, string) (owner.Review, error)
	ListPayments(context.Context, string, string) ([]owner.Payment, error)
	GetFloorPlan(context.Context, string, string) (json.RawMessage, error)
	SaveFloorPlan(context.Context, string, string, json.RawMessage) (json.RawMessage, error)
	GetSubscription(context.Context, string) (*owner.Subscription, error)
	UpsertSubscription(context.Context, string, owner.SubscriptionInput) (owner.Subscription, error)
}

type Dependencies struct {
	Database       *pgxpool.Pool
	Logger         *slog.Logger
	AllowedOrigins []string
	Build          buildinfo.Info
	Auth           AuthService
	Owner          OwnerService
	SessionCookie  string
	SessionSecure  bool
}

type API struct {
	database       *pgxpool.Pool
	logger         *slog.Logger
	allowedOrigins []string
	build          buildinfo.Info
	startedAt      time.Time
	auth           AuthService
	owner          OwnerService
	sessionCookie  string
	sessionSecure  bool
}

func New(deps Dependencies) http.Handler {
	api := &API{
		database:       deps.Database,
		logger:         deps.Logger,
		allowedOrigins: deps.AllowedOrigins,
		build:          deps.Build,
		startedAt:      time.Now().UTC(),
		auth:           deps.Auth,
		owner:          deps.Owner,
		sessionCookie:  deps.SessionCookie,
		sessionSecure:  deps.SessionSecure,
	}

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

	return api.recoverPanic(api.requestLogger(api.securityHeaders(api.cors(mux))))
}

func (api *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "pocket-mvp-backend",
		"uptime":  time.Since(api.startedAt).Round(time.Second).String(),
	})
}

func (api *API) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := api.database.Ping(ctx); err != nil {
		api.logger.Warn("readiness check failed", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (api *API) serviceInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":  "Pocket API",
		"build": api.build,
	})
}

func (api *API) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		originAllowed := origin != "" && slices.Contains(api.allowedOrigins, origin)
		if originAllowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			if origin != "" && !originAllowed {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "origin_not_allowed"})
				return
			}
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if origin != "" && r.Method != http.MethodGet && r.Method != http.MethodHead && !originAllowed {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "origin_not_allowed"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (api *API) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (api *API) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				api.logger.Error("panic recovered", "error", recovered, "stack", string(debug.Stack()))
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_server_error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (api *API) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		api.logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
