package venues

import (
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// venueColumns are the columns scanVenue reads, in its order.
const venueColumns = `id::text, name, slug, COALESCE(description, ''), COALESCE(cuisine_type, ''),
	COALESCE(phone, ''), COALESCE(email, ''), address_line1, city,
	COALESCE(postal_code, ''), country_code, timezone, currency, status, settings, created_at`

// scanVenue reads the venueColumns of one row (a single-row result or one of many).
func scanVenue(row pgx.Row) (Venue, error) {
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
