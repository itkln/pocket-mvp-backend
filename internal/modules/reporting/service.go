package reporting

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type VenueAuthorizer interface {
	RequireOwner(context.Context, string, string) error
}

type Service struct {
	db         *pgxpool.Pool
	authorizer VenueAuthorizer
}

func NewService(db *pgxpool.Pool, authorizer VenueAuthorizer) *Service {
	return &Service{db: db, authorizer: authorizer}
}

func (s *Service) Dashboard(ctx context.Context, ownerID, venueID string) (Dashboard, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return Dashboard{}, err
	}
	var result Dashboard
	err := s.db.QueryRow(ctx, `
		SELECT
		  COALESCE((SELECT SUM(total_minor) FROM orders WHERE venue_id = $1 AND status <> 'cancelled' AND created_at >= CURRENT_DATE), 0),
		  COALESCE((SELECT COUNT(*) FROM orders WHERE venue_id = $1 AND created_at >= CURRENT_DATE), 0),
		  COALESCE((SELECT AVG(total_minor)::bigint FROM orders WHERE venue_id = $1 AND status <> 'cancelled' AND created_at >= CURRENT_DATE), 0),
		  COALESCE((SELECT COUNT(*) FROM venue_tables WHERE venue_id = $1 AND status = 'occupied' AND deleted_at IS NULL), 0),
		  COALESCE((SELECT COUNT(*) FROM venue_tables WHERE venue_id = $1 AND deleted_at IS NULL), 0),
		  COALESCE((SELECT COUNT(*) FROM orders WHERE venue_id = $1 AND status = 'new'), 0),
		  COALESCE((SELECT AVG(rating)::float8 FROM reviews WHERE venue_id = $1 AND status = 'published'), 0)`, venueID,
	).Scan(
		&result.RevenueMinor, &result.OrdersToday, &result.AverageOrderMinor,
		&result.ActiveTables, &result.TotalTables, &result.NewOrders, &result.AverageRating,
	)
	return result, err
}
