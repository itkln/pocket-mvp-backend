package catalog

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
)

func (s *Service) ListCategories(ctx context.Context, ownerID, venueID string) ([]Category, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
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

func (s *Service) CreateCategory(ctx context.Context, ownerID, venueID string, input CategoryInput) (Category, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return Category{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 80 {
		return Category{}, appfault.ErrInvalidInput
	}

	var category Category
	err := s.db.QueryRow(ctx, `
		INSERT INTO menu_categories (venue_id, name, sort_order, is_active)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, name, sort_order, is_active, 0`,
		venueID, input.Name, input.SortOrder, boolValue(input.IsActive, true),
	).Scan(&category.ID, &category.Name, &category.SortOrder, &category.IsActive, &category.ItemCount)
	return category, appfault.MapWriteError(err)
}

func (s *Service) UpdateCategory(ctx context.Context, ownerID, venueID, categoryID string, input CategoryInput) (Category, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return Category{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 80 {
		return Category{}, appfault.ErrInvalidInput
	}

	var category Category
	err := s.db.QueryRow(ctx, `
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

func (s *Service) DeleteCategory(ctx context.Context, ownerID, venueID, categoryID string) error {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return err
	}
	result, err := s.db.Exec(ctx, `DELETE FROM menu_categories WHERE id = $2 AND venue_id = $1`, venueID, categoryID)
	if err != nil {
		return appfault.MapWriteError(err)
	}
	if result.RowsAffected() == 0 {
		return appfault.ErrNotFound
	}
	return nil
}
