package favorites

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) ListIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT venue_id::text FROM spots_saved_venues
		WHERE user_id = $1 ORDER BY saved_at DESC, venue_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *PostgresRepository) Add(ctx context.Context, userID, venueID string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO spots_saved_venues (user_id, venue_id)
		VALUES ($1, $2) ON CONFLICT (user_id, venue_id) DO NOTHING`, userID, venueID)
	return err
}

func (r *PostgresRepository) Remove(ctx context.Context, userID, venueID string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM spots_saved_venues WHERE user_id = $1 AND venue_id = $2`, userID, venueID)
	return err
}
