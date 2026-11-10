package venues

// Managing venues: the owner's list, workplaces of a team member, and editing a venue.
// Authorization happens in the service before these calls.

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
)

func (r *PostgresRepository) List(ctx context.Context, ownerID string) ([]Venue, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+venueColumns+`
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
		RETURNING `+venueColumns,
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

func (r *PostgresRepository) Get(ctx context.Context, venueID string) (Venue, error) {
	venue, err := scanVenue(r.db.QueryRow(ctx, `
		SELECT `+venueColumns+`
		FROM venues
		WHERE id = $1 AND deleted_at IS NULL`, venueID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Venue{}, appfault.ErrNotFound
	}
	return venue, err
}

// Update and Delete trust the service: it has already checked the user's venue capability.
func (r *PostgresRepository) Update(ctx context.Context, venueID string, input Input) (Venue, error) {
	settings, err := json.Marshal(input.Settings)
	if err != nil {
		return Venue{}, appfault.ErrInvalidInput
	}
	row := r.db.QueryRow(ctx, `
		UPDATE venues
		SET name = $2, description = NULLIF($3, ''), cuisine_type = NULLIF($4, ''),
		    phone = NULLIF($5, ''), email = NULLIF($6, ''), address_line1 = $7,
		    city = $8, postal_code = NULLIF($9, ''), country_code = $10,
		    timezone = $11, currency = $12, status = $13,
		    settings = settings || ($14::jsonb - 'floor_plan')
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+venueColumns,
		venueID, input.Name, input.Description, input.CuisineType, input.Phone,
		input.Email, input.Address, input.City, input.PostalCode, input.CountryCode,
		input.Timezone, input.Currency, input.Status, settings,
	)
	venue, err := scanVenue(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Venue{}, appfault.ErrNotFound
	}
	return venue, appfault.MapWriteError(err)
}

func (r *PostgresRepository) Delete(ctx context.Context, venueID string) error {
	result, err := r.db.Exec(ctx, `
		UPDATE venues
		SET deleted_at = now(), status = 'closed'
		WHERE id = $1 AND deleted_at IS NULL`, venueID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return appfault.ErrNotFound
	}
	return nil
}

// ListWorkplaces returns the venues the user owns or works at, with the user's venue role. The
// service decides how much of each venue the role may see.
func (r *PostgresRepository) ListWorkplaces(ctx context.Context, userID string) ([]Venue, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+venueColumns+`, access_role
		FROM (
			SELECT v.*, CASE WHEN v.owner_user_id = $1 THEN 'owner' ELSE s.role END AS access_role
			FROM venues v
			LEFT JOIN venue_staff s ON s.venue_id = v.id AND s.user_id = $1 AND s.status = 'active' AND s.removed_at IS NULL
			WHERE v.deleted_at IS NULL AND (v.owner_user_id = $1 OR s.id IS NOT NULL)
		) workplaces
		ORDER BY created_at, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Venue{}
	for rows.Next() {
		var role string
		venue, err := scanVenue(withRole{rows, &role})
		if err != nil {
			return nil, err
		}
		venue.AccessRole = role
		result = append(result, venue)
	}
	return result, rows.Err()
}

// withRole reads the access role column that follows the venue columns.
type withRole struct {
	row  pgx.Row
	role *string
}

func (w withRole) Scan(dest ...any) error { return w.row.Scan(append(dest, w.role)...) }
