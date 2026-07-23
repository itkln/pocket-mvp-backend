package floorplan

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/appfault"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Get(ctx context.Context, venueID string) (json.RawMessage, error) {
	var plan []byte
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(settings -> 'floor_plan', '[]'::jsonb)
		FROM venues
		WHERE id = $1`, venueID).Scan(&plan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, appfault.ErrNotFound
	}
	return json.RawMessage(plan), err
}

func (r *PostgresRepository) Save(ctx context.Context, ownerID, venueID string, plan json.RawMessage) error {
	result, err := r.db.Exec(ctx, `
		UPDATE venues
		SET settings = jsonb_set(settings, '{floor_plan}', $3::jsonb, true)
		WHERE id = $2 AND owner_user_id = $1 AND deleted_at IS NULL`, ownerID, venueID, []byte(plan))
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return appfault.ErrNotFound
	}
	return nil
}
