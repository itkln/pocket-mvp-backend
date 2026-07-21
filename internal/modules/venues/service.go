package venues

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/security"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) List(ctx context.Context, ownerID string) ([]Venue, error) {
	rows, err := s.db.Query(ctx, `
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

func (s *Service) Create(ctx context.Context, ownerID string, input Input) (Venue, error) {
	input = normalizeInput(input)
	if !validInput(input) {
		return Venue{}, appfault.ErrInvalidInput
	}

	slug, err := uniqueSlug(input.Name)
	if err != nil {
		return Venue{}, err
	}
	settings, err := json.Marshal(input.Settings)
	if err != nil {
		return Venue{}, appfault.ErrInvalidInput
	}

	tx, err := s.db.Begin(ctx)
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

func (s *Service) Update(ctx context.Context, ownerID, venueID string, input Input) (Venue, error) {
	input = normalizeInput(input)
	if !validInput(input) {
		return Venue{}, appfault.ErrInvalidInput
	}
	settings, err := json.Marshal(input.Settings)
	if err != nil {
		return Venue{}, appfault.ErrInvalidInput
	}

	row := s.db.QueryRow(ctx, `
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

func (s *Service) Delete(ctx context.Context, ownerID, venueID string) error {
	result, err := s.db.Exec(ctx, `
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

func normalizeInput(input Input) Input {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.CuisineType = strings.TrimSpace(input.CuisineType)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Email = strings.TrimSpace(input.Email)
	input.Address = strings.TrimSpace(input.Address)
	input.City = strings.TrimSpace(input.City)
	input.PostalCode = strings.TrimSpace(input.PostalCode)
	input.CountryCode = strings.ToUpper(strings.TrimSpace(input.CountryCode))
	input.Timezone = strings.TrimSpace(input.Timezone)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Status = strings.TrimSpace(input.Status)

	if input.Address == "" {
		input.Address = "Адрес не указан"
	}
	if input.CountryCode == "" {
		input.CountryCode = "SK"
	}
	if input.Timezone == "" {
		input.Timezone = "Europe/Bratislava"
	}
	if input.Currency == "" {
		input.Currency = "EUR"
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.Settings == nil {
		input.Settings = map[string]any{}
	}
	return input
}

func validInput(input Input) bool {
	validStatus := regexp.MustCompile(`^(draft|active|paused|closed)$`).MatchString(input.Status)
	return input.Name != "" && input.City != "" && len(input.CountryCode) == 2 && len(input.Currency) == 3 && validStatus
}

func uniqueSlug(name string) (string, error) {
	token, err := security.NewSessionToken()
	if err != nil {
		return "", err
	}
	base := slugify(name)
	if base == "" {
		base = "venue"
	}
	suffix := slugify(token)
	if len(suffix) > 7 {
		suffix = suffix[:7]
	}
	if suffix == "" {
		return "", errors.New("failed to generate venue slug")
	}
	return fmt.Sprintf("%s-%s", base, suffix), nil
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}
