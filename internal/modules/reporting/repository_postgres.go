package reporting

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Dashboard(ctx context.Context, venueID string) (Dashboard, error) {
	var result Dashboard
	err := r.db.QueryRow(ctx, `
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
