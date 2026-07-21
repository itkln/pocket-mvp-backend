package floorplan

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/appfault"
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

func (s *Service) Get(ctx context.Context, ownerID, venueID string) (json.RawMessage, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	var plan []byte
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(settings -> 'floor_plan', '[]'::jsonb)
		FROM venues
		WHERE id = $1`, venueID).Scan(&plan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, appfault.ErrNotFound
	}
	return json.RawMessage(plan), err
}

func (s *Service) Save(ctx context.Context, ownerID, venueID string, plan json.RawMessage) (json.RawMessage, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	if len(plan) == 0 || !json.Valid(plan) {
		return nil, appfault.ErrInvalidInput
	}
	result, err := s.db.Exec(ctx, `
		UPDATE venues
		SET settings = jsonb_set(settings, '{floor_plan}', $3::jsonb, true)
		WHERE id = $2 AND owner_user_id = $1 AND deleted_at IS NULL`, ownerID, venueID, []byte(plan))
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() == 0 {
		return nil, appfault.ErrNotFound
	}
	return plan, nil
}
