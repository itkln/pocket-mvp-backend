package catalog

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
)

const menuItemSelect = `
	SELECT i.id::text, i.category_id::text, c.name, i.name, COALESCE(i.description, ''),
	       i.price_minor, i.currency, i.is_available, i.is_popular, i.sort_order,
	       COALESCE((SELECT public_url FROM menu_item_images WHERE menu_item_id = i.id ORDER BY sort_order LIMIT 1), '')
	FROM menu_items i
	JOIN menu_categories c ON c.id = i.category_id`

func (s *Service) ListMenuItems(ctx context.Context, ownerID, venueID string) ([]MenuItem, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, menuItemSelect+`
		WHERE i.venue_id = $1 AND i.deleted_at IS NULL
		ORDER BY c.sort_order, i.sort_order, i.created_at`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []MenuItem{}
	for rows.Next() {
		item, scanErr := scanMenuItem(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Service) CreateMenuItem(ctx context.Context, ownerID, venueID string, input MenuItemInput) (MenuItem, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return MenuItem{}, err
	}
	input = normalizeMenuItem(input)
	if !validMenuItem(input) {
		return MenuItem{}, appfault.ErrInvalidInput
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return MenuItem{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var item MenuItem
	err = tx.QueryRow(ctx, `
		INSERT INTO menu_items (
			venue_id, category_id, name, description, price_minor, currency,
			is_available, is_popular, sort_order
		)
		SELECT $1, c.id, $3, NULLIF($4, ''), $5, $6, $7, $8, $9
		FROM menu_categories c
		WHERE c.id = $2 AND c.venue_id = $1
		RETURNING id::text, category_id::text,
		          (SELECT name FROM menu_categories WHERE id = category_id),
		          name, COALESCE(description, ''), price_minor, currency,
		          is_available, is_popular, sort_order, ''`,
		venueID, input.CategoryID, input.Name, input.Description, input.PriceMinor,
		input.Currency, boolValue(input.IsAvailable, true), input.IsPopular, input.SortOrder,
	).Scan(
		&item.ID, &item.CategoryID, &item.Category, &item.Name, &item.Description,
		&item.PriceMinor, &item.Currency, &item.IsAvailable, &item.IsPopular,
		&item.SortOrder, &item.ImageURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return MenuItem{}, appfault.ErrInvalidInput
	}
	if err != nil {
		return MenuItem{}, appfault.MapWriteError(err)
	}

	if input.ImageURL != "" {
		_, err = tx.Exec(ctx, `
			INSERT INTO menu_item_images (menu_item_id, storage_key, public_url, content_type, byte_size)
			VALUES ($1, $2, $3, 'image/external', 1)`, item.ID, "external/"+item.ID, input.ImageURL)
		if err != nil {
			return MenuItem{}, err
		}
		item.ImageURL = input.ImageURL
	}
	if err = tx.Commit(ctx); err != nil {
		return MenuItem{}, err
	}
	return item, nil
}

func (s *Service) UpdateMenuItem(ctx context.Context, ownerID, venueID, itemID string, input MenuItemInput) (MenuItem, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return MenuItem{}, err
	}
	input = normalizeMenuItem(input)
	if !validMenuItem(input) {
		return MenuItem{}, appfault.ErrInvalidInput
	}

	var item MenuItem
	err := s.db.QueryRow(ctx, `
		UPDATE menu_items i
		SET category_id = $3, name = $4, description = NULLIF($5, ''), price_minor = $6,
		    currency = $7, is_available = $8, is_popular = $9, sort_order = $10
		FROM menu_categories c
		WHERE i.id = $2 AND i.venue_id = $1 AND c.id = $3 AND c.venue_id = $1 AND i.deleted_at IS NULL
		RETURNING i.id::text, i.category_id::text, c.name, i.name, COALESCE(i.description, ''),
		          i.price_minor, i.currency, i.is_available, i.is_popular, i.sort_order,
		          COALESCE((SELECT public_url FROM menu_item_images WHERE menu_item_id = i.id ORDER BY sort_order LIMIT 1), '')`,
		venueID, itemID, input.CategoryID, input.Name, input.Description, input.PriceMinor,
		input.Currency, boolValue(input.IsAvailable, true), input.IsPopular, input.SortOrder,
	).Scan(
		&item.ID, &item.CategoryID, &item.Category, &item.Name, &item.Description,
		&item.PriceMinor, &item.Currency, &item.IsAvailable, &item.IsPopular,
		&item.SortOrder, &item.ImageURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return MenuItem{}, appfault.ErrNotFound
	}
	return item, appfault.MapWriteError(err)
}

func (s *Service) DeleteMenuItem(ctx context.Context, ownerID, venueID, itemID string) error {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return err
	}
	result, err := s.db.Exec(ctx, `
		UPDATE menu_items
		SET deleted_at = now(), is_available = false
		WHERE id = $2 AND venue_id = $1 AND deleted_at IS NULL`, venueID, itemID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return appfault.ErrNotFound
	}
	return nil
}

type menuItemScanner interface {
	Scan(...any) error
}

func scanMenuItem(row menuItemScanner) (MenuItem, error) {
	var item MenuItem
	err := row.Scan(
		&item.ID, &item.CategoryID, &item.Category, &item.Name, &item.Description,
		&item.PriceMinor, &item.Currency, &item.IsAvailable, &item.IsPopular,
		&item.SortOrder, &item.ImageURL,
	)
	return item, err
}
