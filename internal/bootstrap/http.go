package bootstrap

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/buildinfo"
	"pocket-mvp-backend/internal/config"
	"pocket-mvp-backend/internal/httpapi"
	"pocket-mvp-backend/internal/modules/availability"
	"pocket-mvp-backend/internal/modules/billing"
	"pocket-mvp-backend/internal/modules/catalog"
	"pocket-mvp-backend/internal/modules/dinein"
	"pocket-mvp-backend/internal/modules/feedback"
	"pocket-mvp-backend/internal/modules/floorplan"
	"pocket-mvp-backend/internal/modules/identity"
	"pocket-mvp-backend/internal/modules/ordering"
	"pocket-mvp-backend/internal/modules/reporting"
	"pocket-mvp-backend/internal/modules/reservations"
	"pocket-mvp-backend/internal/modules/seating"
	"pocket-mvp-backend/internal/modules/venues"
	"pocket-mvp-backend/internal/modules/workforce"
	"pocket-mvp-backend/internal/notifications"
	"pocket-mvp-backend/internal/security"
)

func NewHTTPHandler(db *pgxpool.Pool, cfg config.Config, logger *slog.Logger) (http.Handler, error) {
	protector, err := security.NewDataProtector(cfg.DataEncryptionKey, cfg.DataLookupKey)
	if err != nil {
		return nil, err
	}

	venueAuthorizer := access.NewVenueAuthorizer(db)
	capabilityReader := access.NewCapabilityReader(db)
	resetSender := notifications.NewPasswordResetSender(
		logger,
		cfg.SMTPAddress,
		cfg.SMTPUsername,
		cfg.SMTPPassword,
		cfg.SMTPFrom,
	)
	identityService, err := identity.NewService(
		identity.NewPostgresRepository(db),
		protector,
		capabilityReader,
		cfg.SessionTTL,
		identity.PasswordResetOptions{
			Sender:      resetSender,
			BaseURL:     cfg.AppBaseURL,
			DishBaseURL: cfg.DishAppBaseURL,
			TTL:         cfg.PasswordResetTTL,
		},
	)
	if err != nil {
		return nil, err
	}
	venueService := venues.NewService(venues.NewPostgresRepository(db), venueAuthorizer)
	availabilityRepository := availability.NewPostgresRepository(db)
	availabilityService := availability.NewService(availabilityRepository)

	return httpapi.New(httpapi.Dependencies{
		DishBaseURL:    cfg.DishAppBaseURL,
		Database:       db,
		Logger:         logger,
		AllowedOrigins: cfg.AllowedOrigins,
		Build:          buildinfo.Current(),
		Identity:       identityService,
		Venues:         venueService,
		Availability:   availabilityService,
		Reservations:   reservations.NewService(reservations.NewPostgresRepository(db), reservationAvailability{availabilityService, availabilityRepository}),
		Catalog:        catalog.NewService(catalog.NewPostgresRepository(db), venueAuthorizer),
		Workforce:      workforce.NewService(workforce.NewPostgresRepository(db), venueAuthorizer, workforce.InvitationOptions{Sender: resetSender, BaseURL: cfg.AppBaseURL, Protector: protector}),
		Ordering:       ordering.NewService(ordering.NewPostgresRepository(db, protector), venueAuthorizer),
		DineIn:         dinein.NewService(dinein.NewPostgresRepository(db), venueAuthorizer, protector, cfg.DishAppBaseURL),
		Feedback:       feedback.NewService(feedback.NewPostgresRepository(db), venueAuthorizer),
		Billing:        billing.NewService(billing.NewPostgresRepository(db), venueAuthorizer),
		FloorPlan:      floorplan.NewService(floorplan.NewPostgresRepository(db), venueAuthorizer),
		Seating:        seating.NewService(seating.NewPostgresRepository(db), venueAuthorizer),
		Reporting:      reporting.NewService(reporting.NewPostgresRepository(db), venueAuthorizer),
		SessionCookie:  cfg.SessionCookieName,
		SessionSecure:  cfg.CookieSecure,
	}), nil
}
