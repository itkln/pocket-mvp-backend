package catalog

import (
	"context"
	"pocket-mvp-backend/internal/access"
	"strings"
	"unicode/utf8"

	"pocket-mvp-backend/internal/appfault"
)

func (s *Service) ListCategories(ctx context.Context, userID, venueID string) ([]Category, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.ManageMenu); err != nil {
		return nil, err
	}
	return s.repository.ListCategories(ctx, venueID)
}

func (s *Service) CreateCategory(ctx context.Context, userID, venueID string, input CategoryInput) (Category, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.ManageMenu); err != nil {
		return Category{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 80 {
		return Category{}, appfault.ErrInvalidInput
	}
	return s.repository.CreateCategory(ctx, venueID, input)
}

func (s *Service) UpdateCategory(ctx context.Context, userID, venueID, categoryID string, input CategoryInput) (Category, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.ManageMenu); err != nil {
		return Category{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 80 {
		return Category{}, appfault.ErrInvalidInput
	}
	return s.repository.UpdateCategory(ctx, venueID, categoryID, input)
}

func (s *Service) DeleteCategory(ctx context.Context, userID, venueID, categoryID string) error {
	if err := s.authorizer.Require(ctx, userID, venueID, access.ManageMenu); err != nil {
		return err
	}
	return s.repository.DeleteCategory(ctx, venueID, categoryID)
}
