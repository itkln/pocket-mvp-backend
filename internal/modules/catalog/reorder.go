package catalog

import (
	"context"
	"strings"

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
	return s.repository.ReorderCategories(ctx, venueID, ids)
}

func (s *Service) ReorderMenuItems(ctx context.Context, ownerID, venueID, categoryID string, ids []string) error {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return err
	}
	if strings.TrimSpace(categoryID) == "" || !validOrder(ids) {
		return appfault.ErrInvalidInput
	}
	return s.repository.ReorderMenuItems(ctx, venueID, categoryID, ids)
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
