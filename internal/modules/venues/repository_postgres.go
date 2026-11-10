package venues

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

// venueColumns are the columns scanVenue reads, in its order.
const venueColumns = `id::text, name, slug, COALESCE(description, ''), COALESCE(cuisine_type, ''),
	COALESCE(phone, ''), COALESCE(email, ''), address_line1, city,
	COALESCE(postal_code, ''), country_code, timezone, currency, status, settings, created_at`

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

func (r *PostgresRepository) IsPublished(ctx context.Context, venueID string) (bool, error) {
	var published bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM venues WHERE id = $1 AND status = 'active' AND deleted_at IS NULL
	)`, venueID).Scan(&published)
	return published, err
}

func (r *PostgresRepository) ListPublishedMenu(ctx context.Context, venueID string) ([]PublicMenuCategory, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id::text, c.name, c.preorder_enabled, i.id::text, i.name, COALESCE(i.description, ''),
		       i.price_minor, i.currency,
		       COALESCE((SELECT public_url FROM menu_item_images
		                 WHERE menu_item_id = i.id ORDER BY sort_order, id LIMIT 1), ''),
               i.portion_weight_g, i.energy_kcal, i.protein_g, i.fat_g, i.carbs_g
		FROM menu_categories c
		JOIN menu_items i ON i.category_id = c.id AND i.venue_id = c.venue_id
		JOIN venues v ON v.id = c.venue_id
		WHERE c.venue_id = $1 AND c.is_active AND i.is_available AND i.deleted_at IS NULL
		  AND v.status = 'active' AND v.deleted_at IS NULL AND COALESCE(v.settings->>'public_category', 'restaurants') = 'restaurants'
		ORDER BY c.sort_order, c.created_at, c.id, i.sort_order, i.created_at, i.id`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := []PublicMenuCategory{}
	for rows.Next() {
		var categoryID, categoryName string
		var preorderEnabled bool
		var item PublicMenuItem
		if err := rows.Scan(&categoryID, &categoryName, &preorderEnabled, &item.ID, &item.Name,
			&item.Description, &item.PriceMinor, &item.Currency, &item.ImageURL,
			&item.PortionWeightG, &item.EnergyKcal, &item.ProteinG, &item.FatG, &item.CarbsG); err != nil {
			return nil, err
		}
		if len(categories) == 0 || categories[len(categories)-1].ID != categoryID {
			categories = append(categories, PublicMenuCategory{ID: categoryID, Name: categoryName, PreorderEnabled: preorderEnabled, Items: []PublicMenuItem{}})
		}
		categories[len(categories)-1].Items = append(categories[len(categories)-1].Items, item)
	}
	return categories, rows.Err()
}

func (r *PostgresRepository) ListPublishedReviews(ctx context.Context, venueID string) ([]PublicReview, error) {
	rows, err := r.db.Query(ctx, `
		SELECT r.id::text, r.rating, COALESCE(r.body, ''), COALESCE(r.owner_reply, ''), r.created_at
		FROM reviews r
		JOIN venues v ON v.id = r.venue_id
		WHERE r.venue_id = $1 AND r.status = 'published'
		  AND v.status = 'active' AND v.deleted_at IS NULL AND COALESCE(v.settings->>'public_category', 'restaurants') = 'restaurants'
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT 50`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reviews := []PublicReview{}
	for rows.Next() {
		var review PublicReview
		if err := rows.Scan(&review.ID, &review.Rating, &review.Body, &review.OwnerReply, &review.CreatedAt); err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (r *PostgresRepository) GetPublished(ctx context.Context, venueID string) (PublicVenue, error) {
	row := r.db.QueryRow(ctx, `
		SELECT v.id::text, v.slug, v.name, COALESCE(v.description, ''),
		       CASE WHEN v.settings ->> 'public_category' IN (
		         'restaurants', 'entertainment', 'activities', 'sports', 'yoga-pilates',
		         'spa-massage', 'beauty', 'masterclasses'
		       ) THEN v.settings ->> 'public_category' ELSE 'restaurants' END,
		       COALESCE(v.cuisine_type, ''), v.address_line1, v.city, v.country_code,
		       v.settings, COALESCE(ROUND(AVG(reviews.rating)::numeric, 1)::float8, 0),
		       COUNT(reviews.id)::int, v.created_at
		FROM venues v
		LEFT JOIN reviews ON reviews.venue_id = v.id AND reviews.status = 'published'
		WHERE v.id = $1 AND v.status = 'active' AND v.deleted_at IS NULL AND COALESCE(v.settings->>'public_category', 'restaurants') = 'restaurants'
		GROUP BY v.id`, venueID)
	var item PublicVenue
	var settings []byte
	err := row.Scan(&item.ID, &item.Slug, &item.Name, &item.Description, &item.CategorySlug,
		&item.CuisineType, &item.Address, &item.City, &item.CountryCode, &settings,
		&item.Rating, &item.ReviewCount, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicVenue{}, appfault.ErrNotFound
	}
	if err != nil {
		return PublicVenue{}, err
	}
	var publicSettings map[string]any
	if err := json.Unmarshal(settings, &publicSettings); err != nil {
		return PublicVenue{}, fmt.Errorf("decode public venue settings: %w", err)
	}
	item.ImageURL = settingString(publicSettings, "cover_image_url")
	item.LogoURL = settingString(publicSettings, "logo_image_url")
	item.PreorderEnabled = settingBool(publicSettings, "preorder_enabled")
	item.Tags = settingStrings(publicSettings, "public_tags")
	item.Filters = settingStrings(publicSettings, "public_filters")
	if item.Tags == nil {
		item.Tags = []string{}
	}
	if item.Filters == nil {
		item.Filters = []string{}
	}
	return item, nil
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

func (r *PostgresRepository) Get(ctx context.Context, venueID string) (Venue, error) {
	venue, err := scanVenue(r.db.QueryRow(ctx, `SELECT `+venueColumns+` FROM venues WHERE id = $1 AND deleted_at IS NULL`, venueID))
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

func settingString(settings map[string]any, key string) string {
	value, _ := settings[key].(string)
	return value
}

func settingBool(settings map[string]any, key string) bool {
	value, _ := settings[key].(bool)
	return value
}

func settingStrings(settings map[string]any, key string) []string {
	values, ok := settings[key].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			result = append(result, text)
		}
	}
	return result
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
	row  venueScanner
	role *string
}

func (w withRole) Scan(dest ...any) error { return w.row.Scan(append(dest, w.role)...) }
