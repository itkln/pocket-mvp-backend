package catalog

import (
	"context"

	"pocket-mvp-backend/internal/access"

	"pocket-mvp-backend/internal/appfault"
)

func (s *Service) ListMenuItems(ctx context.Context, userID, venueID string) ([]MenuItem, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.ManageMenu); err != nil {
		return nil, err
	}
	return s.repository.ListMenuItems(ctx, venueID)
}

func (s *Service) CreateMenuItem(ctx context.Context, userID, venueID string, input MenuItemInput) (MenuItem, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.ManageMenu); err != nil {
		return MenuItem{}, err
	}
	input = normalizeMenuItem(input)
	if !validMenuItem(input) {
		return MenuItem{}, appfault.ErrInvalidInput
	}
	return s.repository.CreateMenuItem(ctx, venueID, input)
}

func (s *Service) UpdateMenuItem(ctx context.Context, userID, venueID, itemID string, input MenuItemInput) (MenuItem, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.ManageMenu); err != nil {
		return MenuItem{}, err
	}
	input = normalizeMenuItem(input)
	if !validMenuItem(input) {
		return MenuItem{}, appfault.ErrInvalidInput
	}
	return s.repository.UpdateMenuItem(ctx, venueID, itemID, input)
}

func (s *Service) DeleteMenuItem(ctx context.Context, userID, venueID, itemID string) error {
	if err := s.authorizer.Require(ctx, userID, venueID, access.ManageMenu); err != nil {
		return err
	}
	return s.repository.DeleteMenuItem(ctx, venueID, itemID)
}
