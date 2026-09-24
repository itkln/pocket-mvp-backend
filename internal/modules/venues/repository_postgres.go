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

func (r *PostgresRepository) SearchPublished(ctx context.Context, query PublicVenueQuery) ([]PublicVenue, int, error) {
	searchTerms := strings.Fields(query.Search)
	rows, err := r.db.Query(ctx, `
		WITH catalog AS (
			SELECT v.id::text, v.slug, v.name, COALESCE(v.description, '') AS description,
			       CASE WHEN v.settings ->> 'public_category' IN (
			         'restaurants', 'entertainment', 'activities', 'sports', 'yoga-pilates',
			         'spa-massage', 'beauty', 'masterclasses'
			       ) THEN v.settings ->> 'public_category' ELSE 'restaurants' END AS category_slug,
			       COALESCE(v.cuisine_type, '') AS cuisine_type, v.address_line1, v.city,
			       v.country_code, v.settings, v.created_at,
			       COALESCE(ROUND(AVG(r.rating)::numeric, 1)::float8, 0) AS rating,
			       COUNT(r.id)::int AS review_count
			FROM venues v
			LEFT JOIN reviews r ON r.venue_id = v.id AND r.status = 'published'
			WHERE v.status = 'active' AND v.deleted_at IS NULL
			  AND ($7::text[] = '{}'::text[] OR v.id::text = ANY($7::text[]))
			GROUP BY v.id
		), searchable_catalog AS (
			SELECT *, concat_ws(' ', name, description, cuisine_type, address_line1, city,
			       COALESCE(settings ->> 'public_search_terms', '')) AS searchable
			FROM catalog
		), filtered AS (
			SELECT *
			FROM searchable_catalog
			WHERE ($1 = '' OR lower(city) = lower($1))
			  AND ($2::text[] = '{}'::text[] OR NOT EXISTS (
				SELECT 1 FROM unnest($2::text[]) term WHERE searchable NOT ILIKE '%' || term || '%'
			  ))
			  AND ($3 = '' OR category_slug = $3)
		)
		SELECT id, slug, name, description, category_slug, cuisine_type, address_line1,
		       city, country_code, settings, rating, review_count, created_at,
		       COUNT(*) OVER()::bigint
		FROM filtered
		WHERE $4 = ''
		   OR (lower($4) = lower('Лучшие') AND rating >= 4.5)
		   OR lower($4) = lower('Открыто')
		   OR (lower($4) = lower('С предзаказом') AND CASE
		         WHEN jsonb_typeof(settings -> 'preorder_enabled') = 'boolean'
		         THEN (settings ->> 'preorder_enabled')::boolean ELSE false END)
		   OR cuisine_type ILIKE '%' || $4 || '%'
		   OR EXISTS (
			 SELECT 1
			 FROM jsonb_array_elements_text(
			   CASE WHEN jsonb_typeof(settings -> 'public_filters') = 'array'
			        THEN settings -> 'public_filters' ELSE '[]'::jsonb END
			 ) item
			 WHERE lower(item) = lower($4)
		   )
		ORDER BY rating DESC, review_count DESC, created_at DESC, id
		LIMIT $5 OFFSET $6`,
		query.City, searchTerms, query.Category, query.Filter,
		query.PageSize, (query.Page-1)*query.PageSize, query.IDs,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []PublicVenue{}
	total := 0
	for rows.Next() {
		var item PublicVenue
		var settings []byte
		var rowTotal int64
		if err := rows.Scan(
			&item.ID, &item.Slug, &item.Name, &item.Description, &item.CategorySlug,
			&item.CuisineType, &item.Address, &item.City, &item.CountryCode, &settings,
			&item.Rating, &item.ReviewCount, &item.CreatedAt, &rowTotal,
		); err != nil {
			return nil, 0, err
		}
		var publicSettings map[string]any
		if err := json.Unmarshal(settings, &publicSettings); err != nil {
			return nil, 0, fmt.Errorf("decode public venue settings: %w", err)
		}
		item.ImageURL = settingString(publicSettings, "cover_image_url")
		item.PreorderEnabled = settingBool(publicSettings, "preorder_enabled")
		item.Tags = settingStrings(publicSettings, "public_tags")
		item.Filters = settingStrings(publicSettings, "public_filters")
		if item.Tags == nil {
			item.Tags = []string{}
		}
		if item.Filters == nil {
			item.Filters = []string{}
		}
		items = append(items, item)
		total = int(rowTotal)
	}
	return items, total, rows.Err()
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
		SELECT c.id::text, c.name, i.id::text, i.name, COALESCE(i.description, ''),
		       i.price_minor, i.currency,
		       COALESCE((SELECT public_url FROM menu_item_images
		                 WHERE menu_item_id = i.id ORDER BY sort_order, id LIMIT 1), '')
		FROM menu_categories c
		JOIN menu_items i ON i.category_id = c.id AND i.venue_id = c.venue_id
		JOIN venues v ON v.id = c.venue_id
		WHERE c.venue_id = $1 AND c.is_active AND i.is_available AND i.deleted_at IS NULL
		  AND v.status = 'active' AND v.deleted_at IS NULL
		ORDER BY c.sort_order, c.created_at, c.id, i.sort_order, i.created_at, i.id`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := []PublicMenuCategory{}
	for rows.Next() {
		var categoryID, categoryName string
		var item PublicMenuItem
		if err := rows.Scan(&categoryID, &categoryName, &item.ID, &item.Name,
			&item.Description, &item.PriceMinor, &item.Currency, &item.ImageURL); err != nil {
			return nil, err
		}
		if len(categories) == 0 || categories[len(categories)-1].ID != categoryID {
			categories = append(categories, PublicMenuCategory{ID: categoryID, Name: categoryName, Items: []PublicMenuItem{}})
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
		  AND v.status = 'active' AND v.deleted_at IS NULL
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
		WHERE v.id = $1 AND v.status = 'active' AND v.deleted_at IS NULL
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
