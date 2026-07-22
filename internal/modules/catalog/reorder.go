package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
)

const maxReorderItems = 500

func (s *Service) ReorderCategories(ctx context.Context, ownerID, venueID string, ids []string) error {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return err
	}
	if !validOrder(ids) {
		return appfault.ErrInvalidInput
	}
	return s.reorder(ctx, `
		UPDATE menu_categories AS category
		SET sort_order = ordered.position::integer - 1
		FROM unnest($2::text[]) WITH ORDINALITY AS ordered(id, position)
		WHERE category.venue_id = $1 AND category.id::text = ordered.id`, venueID, ids)
}

func (s *Service) ReorderMenuItems(ctx context.Context, ownerID, venueID, categoryID string, ids []string) error {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return err
	}
	if strings.TrimSpace(categoryID) == "" || !validOrder(ids) {
		return appfault.ErrInvalidInput
	}
	return s.reorder(ctx, `
		UPDATE menu_items AS item
		SET sort_order = ordered.position::integer - 1
		FROM unnest($3::text[]) WITH ORDINALITY AS ordered(id, position)
		WHERE item.venue_id = $1 AND item.category_id::text = $2
		  AND item.deleted_at IS NULL AND item.id::text = ordered.id`, venueID, categoryID, ids)
}

func (s *Service) reorder(ctx context.Context, query string, arguments ...any) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
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

func validOrder(ids []string) bool {
	if len(ids) == 0 || len(ids) > maxReorderItems {
		return false
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return false
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
}
