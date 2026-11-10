package availability

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/appfault"
)

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) GetVenue(ctx context.Context, venueID string) (Venue, error) {
	var venue Venue
	err := r.db.QueryRow(ctx, `
		SELECT timezone,
		       COALESCE((settings ->> 'schedule_enabled')::boolean, false),
		       COALESCE((settings ->> 'same_day_booking')::boolean, false)
		FROM venues
		WHERE id = $1 AND status = 'active' AND deleted_at IS NULL
		  AND COALESCE(settings ->> 'public_category', 'restaurants') = 'restaurants'`, venueID,
	).Scan(&venue.Timezone, &venue.ScheduleEnabled, &venue.SameDayBooking)
	if errors.Is(err, pgx.ErrNoRows) {
		return Venue{}, appfault.ErrNotFound
	}
	return venue, err
}

func (r *PostgresRepository) ListTables(ctx context.Context, venueID string, open, close time.Time) ([]Table, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id::text, t.identifier, t.seats, t.status,
		       r.starts_at, r.ends_at
		FROM venue_tables t
		JOIN venue_layouts l ON l.id = t.layout_id AND l.is_active
		LEFT JOIN reservations r ON r.venue_table_id = t.id
		  AND (r.status IN ('confirmed', 'seated') OR
		       (r.status = 'pending' AND (r.hold_expires_at IS NULL OR r.hold_expires_at > now())))
		  AND r.starts_at < $3 AND r.ends_at > $2
		WHERE t.venue_id = $1 AND t.deleted_at IS NULL
		ORDER BY t.identifier, t.id, r.starts_at`, venueID, open, close)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tables := []Table{}
	for rows.Next() {
		var table Table
		var start, end *time.Time
		if err := rows.Scan(&table.ID, &table.Identifier, &table.Seats, &table.Status, &start, &end); err != nil {
			return nil, err
		}
		if len(tables) == 0 || tables[len(tables)-1].ID != table.ID {
			tables = append(tables, table)
		}
		if start != nil && end != nil {
			last := &tables[len(tables)-1]
			last.Bookings = append(last.Bookings, Interval{Start: *start, End: *end})
		}
	}
	return tables, rows.Err()
}
