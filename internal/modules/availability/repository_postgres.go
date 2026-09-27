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

func (r *PostgresRepository) ListFloors(ctx context.Context, venueID string) ([]Floor, error) {
	rows, err := r.db.Query(ctx, `
		SELECT l.id::text, l.name, f.id::text, f.fixture_type,
		       f.position_x::float8, f.position_y::float8, f.rotation, f.size::float8
		FROM venue_layouts l
		LEFT JOIN venue_layout_fixtures f ON f.layout_id = l.id
		WHERE l.venue_id = $1 AND l.is_active
		ORDER BY l.created_at, l.id, f.created_at, f.id`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	floors := []Floor{}
	for rows.Next() {
		var id, name string
		var fixtureID, fixtureType *string
		var x, y, size *float64
		var rotation *int
		if err := rows.Scan(&id, &name, &fixtureID, &fixtureType, &x, &y, &rotation, &size); err != nil {
			return nil, err
		}
		if len(floors) == 0 || floors[len(floors)-1].ID != id {
			floors = append(floors, Floor{ID: id, Name: name, Tables: []FloorTable{}, Fixtures: []Fixture{}})
		}
		if fixtureID != nil {
			floors[len(floors)-1].Fixtures = append(floors[len(floors)-1].Fixtures, Fixture{
				ID: *fixtureID, Type: *fixtureType, X: *x, Y: *y, Rotation: *rotation, Size: *size,
			})
		}
	}
	return floors, rows.Err()
}

func (r *PostgresRepository) ListTables(ctx context.Context, venueID string, open, close time.Time) ([]Table, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id::text, t.identifier, t.seats, t.layout_id::text, t.shape,
		       t.position_x::float8, t.position_y::float8, t.width::float8, t.height::float8, t.status,
		       r.starts_at, r.ends_at
		FROM venue_tables t
		JOIN venue_layouts l ON l.id = t.layout_id AND l.is_active
		LEFT JOIN reservations r ON r.venue_table_id = t.id
		  AND r.status IN ('pending', 'confirmed', 'seated')
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
		if err := rows.Scan(&table.ID, &table.Identifier, &table.Seats, &table.LayoutID, &table.Shape,
			&table.X, &table.Y, &table.Width, &table.Height, &table.Status, &start, &end); err != nil {
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
