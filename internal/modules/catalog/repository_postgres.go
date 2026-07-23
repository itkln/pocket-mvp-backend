package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/appfault"
)

const menuItemSelect = `
	SELECT i.id::text, i.category_id::text, c.name, i.name, COALESCE(i.description, ''),
	       i.price_minor, i.currency, i.is_available, i.is_popular, i.sort_order,
	       COALESCE((SELECT public_url FROM menu_item_images WHERE menu_item_id = i.id ORDER BY sort_order LIMIT 1), '')
	FROM menu_items i
	JOIN menu_categories c ON c.id = i.category_id`

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) ListCategories(ctx context.Context, venueID string) ([]Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id::text, c.name, c.sort_order, c.is_active, COUNT(i.id)
		FROM menu_categories c
		LEFT JOIN menu_items i ON i.category_id = c.id AND i.deleted_at IS NULL
		WHERE c.venue_id = $1
		GROUP BY c.id
		ORDER BY c.sort_order, c.created_at`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Category{}
	for rows.Next() {
		var category Category
		if err := rows.Scan(&category.ID, &category.Name, &category.SortOrder, &category.IsActive, &category.ItemCount); err != nil {
			return nil, err
		}
		result = append(result, category)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) CreateCategory(ctx context.Context, venueID string, input CategoryInput) (Category, error) {
	var category Category
	err := r.db.QueryRow(ctx, `
		INSERT INTO menu_categories (venue_id, name, sort_order, is_active)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, name, sort_order, is_active, 0`,
		venueID, input.Name, input.SortOrder, boolValue(input.IsActive, true),
	).Scan(&category.ID, &category.Name, &category.SortOrder, &category.IsActive, &category.ItemCount)
	return category, appfault.MapWriteError(err)
}

func (r *PostgresRepository) UpdateCategory(ctx context.Context, venueID, categoryID string, input CategoryInput) (Category, error) {
	var category Category
	err := r.db.QueryRow(ctx, `
		UPDATE menu_categories
		SET name = $3, sort_order = $4, is_active = $5
		WHERE id = $2 AND venue_id = $1
		RETURNING id::text, name, sort_order, is_active,
		          (SELECT COUNT(*) FROM menu_items WHERE category_id = $2 AND deleted_at IS NULL)`,
		venueID, categoryID, input.Name, input.SortOrder, boolValue(input.IsActive, true),
	).Scan(&category.ID, &category.Name, &category.SortOrder, &category.IsActive, &category.ItemCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, appfault.ErrNotFound
	}
	return category, appfault.MapWriteError(err)
}

func (r *PostgresRepository) DeleteCategory(ctx context.Context, venueID, categoryID string) error {
	result, err := r.db.Exec(ctx, `DELETE FROM menu_categories WHERE id = $2 AND venue_id = $1`, venueID, categoryID)
	if err != nil {
		return appfault.MapWriteError(err)
	}
	if result.RowsAffected() == 0 {
		return appfault.ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) ListMenuItems(ctx context.Context, venueID string) ([]MenuItem, error) {
	rows, err := r.db.Query(ctx, menuItemSelect+`
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

func (r *PostgresRepository) CreateMenuItem(ctx context.Context, venueID string, input MenuItemInput) (MenuItem, error) {
	tx, err := r.db.Begin(ctx)
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

func (r *PostgresRepository) UpdateMenuItem(ctx context.Context, venueID, itemID string, input MenuItemInput) (MenuItem, error) {
	var item MenuItem
	err := r.db.QueryRow(ctx, `
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

func (r *PostgresRepository) DeleteMenuItem(ctx context.Context, venueID, itemID string) error {
	result, err := r.db.Exec(ctx, `
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

func (r *PostgresRepository) ReorderCategories(ctx context.Context, venueID string, ids []string) error {
	return r.reorder(ctx, `
		UPDATE menu_categories AS category
		SET sort_order = ordered.position::integer - 1
		FROM unnest($2::text[]) WITH ORDINALITY AS ordered(id, position)
		WHERE category.venue_id = $1 AND category.id::text = ordered.id`, venueID, ids)
}

func (r *PostgresRepository) ReorderMenuItems(ctx context.Context, venueID, categoryID string, ids []string) error {
	return r.reorder(ctx, `
		UPDATE menu_items AS item
		SET sort_order = ordered.position::integer - 1
		FROM unnest($3::text[]) WITH ORDINALITY AS ordered(id, position)
		WHERE item.venue_id = $1 AND item.category_id::text = $2
		  AND item.deleted_at IS NULL AND item.id::text = ordered.id`, venueID, categoryID, ids)
}

func (r *PostgresRepository) reorder(ctx context.Context, query string, arguments ...any) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin catalog reorder: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, query, arguments...)
	if err != nil {
		return fmt.Errorf("reorder catalog: %w", err)
	}
	ids := arguments[len(arguments)-1].([]string)
	if int(result.RowsAffected()) != len(ids) {
		return appfault.ErrInvalidInput
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit catalog reorder: %w", err)
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
