package catalog

import (
	"context"
	"pocket-mvp-backend/internal/access"
	"strings"

	"pocket-mvp-backend/internal/appfault"
)

const maxReorderItems = 500

func (s *Service) ReorderCategories(ctx context.Context, userID, venueID string, ids []string) error {
	if err := s.authorizer.Require(ctx, userID, venueID, access.ManageMenu); err != nil {
		return err
	}
	if !validOrder(ids) {
		return appfault.ErrInvalidInput
	}
	return s.repository.ReorderCategories(ctx, venueID, ids)
}

func (s *Service) ReorderMenuItems(ctx context.Context, userID, venueID, categoryID string, ids []string) error {
	if err := s.authorizer.Require(ctx, userID, venueID, access.ManageMenu); err != nil {
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
