package venues

// The public venue page in Dish: only published (active) venues are read.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
)

func (r *PostgresRepository) IsPublished(ctx context.Context, venueID string) (bool, error) {
	var published bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
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
