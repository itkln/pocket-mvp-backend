package venues

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

func (r *PostgresRepository) List(ctx context.Context, ownerID string) ([]Venue, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, name, slug, COALESCE(description, ''), COALESCE(cuisine_type, ''),
		       COALESCE(phone, ''), COALESCE(email, ''), address_line1, city,
		       COALESCE(postal_code, ''), country_code, timezone, currency, status, settings, created_at
		FROM venues
		WHERE owner_user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Venue{}
	for rows.Next() {
		venue, scanErr := scanVenue(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, venue)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) Create(ctx context.Context, ownerID, slug string, input Input) (Venue, error) {
	settings, err := json.Marshal(input.Settings)
	if err != nil {
		return Venue{}, appfault.ErrInvalidInput
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Venue{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		INSERT INTO venues (
			owner_user_id, slug, name, description, cuisine_type, phone, email,
			address_line1, city, postal_code, country_code, timezone, currency, status, settings
		)
		VALUES (
			$1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''),
			$8, $9, NULLIF($10, ''), $11, $12, $13, $14, $15
		)
		RETURNING id::text, name, slug, COALESCE(description, ''), COALESCE(cuisine_type, ''),
		          COALESCE(phone, ''), COALESCE(email, ''), address_line1, city,
		          COALESCE(postal_code, ''), country_code, timezone, currency, status, settings, created_at`,
		ownerID, slug, input.Name, input.Description, input.CuisineType, input.Phone, input.Email,
		input.Address, input.City, input.PostalCode, input.CountryCode, input.Timezone,
		input.Currency, input.Status, settings,
	)
	venue, err := scanVenue(row)
	if err != nil {
		return Venue{}, appfault.MapWriteError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET account_role = 'venue_owner' WHERE id = $1`, ownerID); err != nil {
		return Venue{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Venue{}, err
	}
	return venue, nil
}

func (r *PostgresRepository) Update(ctx context.Context, ownerID, venueID string, input Input) (Venue, error) {
	settings, err := json.Marshal(input.Settings)
	if err != nil {
		return Venue{}, appfault.ErrInvalidInput
	}
	row := r.db.QueryRow(ctx, `
		UPDATE venues
		SET name = $3, description = NULLIF($4, ''), cuisine_type = NULLIF($5, ''),
		    phone = NULLIF($6, ''), email = NULLIF($7, ''), address_line1 = $8,
		    city = $9, postal_code = NULLIF($10, ''), country_code = $11,
		    timezone = $12, currency = $13, status = $14,
		    settings = settings || ($15::jsonb - 'floor_plan')
		WHERE id = $2 AND owner_user_id = $1 AND deleted_at IS NULL
		RETURNING id::text, name, slug, COALESCE(description, ''), COALESCE(cuisine_type, ''),
		          COALESCE(phone, ''), COALESCE(email, ''), address_line1, city,
		          COALESCE(postal_code, ''), country_code, timezone, currency, status, settings, created_at`,
		ownerID, venueID, input.Name, input.Description, input.CuisineType, input.Phone,
		input.Email, input.Address, input.City, input.PostalCode, input.CountryCode,
		input.Timezone, input.Currency, input.Status, settings,
	)
	venue, err := scanVenue(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Venue{}, appfault.ErrNotFound
	}
	return venue, appfault.MapWriteError(err)
}

func (r *PostgresRepository) Delete(ctx context.Context, ownerID, venueID string) error {
	result, err := r.db.Exec(ctx, `
		UPDATE venues
		SET deleted_at = now(), status = 'closed'
		WHERE id = $2 AND owner_user_id = $1 AND deleted_at IS NULL`, ownerID, venueID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return appfault.ErrNotFound
	}
	return nil
}

type venueScanner interface {
	Scan(...any) error
}

func scanVenue(row venueScanner) (Venue, error) {
	var venue Venue
	var settings []byte
	err := row.Scan(
		&venue.ID, &venue.Name, &venue.Slug, &venue.Description, &venue.CuisineType,
		&venue.Phone, &venue.Email, &venue.Address, &venue.City, &venue.PostalCode,
		&venue.CountryCode, &venue.Timezone, &venue.Currency, &venue.Status, &settings,
		&venue.CreatedAt,
	)
	if err != nil {
		return Venue{}, err
	}
	if err := json.Unmarshal(settings, &venue.Settings); err != nil {
		return Venue{}, fmt.Errorf("decode venue settings: %w", err)
	}
	if venue.Settings == nil {
		venue.Settings = map[string]any{}
	}
	return venue, nil
}
