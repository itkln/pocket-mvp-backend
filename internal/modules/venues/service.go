package venues

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/security"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(ctx context.Context, ownerID string) ([]Venue, error) {
	return s.repository.List(ctx, ownerID)
}

func (s *Service) GetPublished(ctx context.Context, venueID string) (PublicVenue, error) {
	if !validVenueID(venueID) {
		return PublicVenue{}, appfault.ErrInvalidInput
	}
	return s.repository.GetPublished(ctx, venueID)
}

func (s *Service) ListPublishedMenu(ctx context.Context, venueID string) ([]PublicMenuCategory, error) {
	if !validVenueID(venueID) {
		return nil, appfault.ErrInvalidInput
	}
	published, err := s.repository.IsPublished(ctx, venueID)
	if err != nil {
		return nil, err
	}
	if !published {
		return nil, appfault.ErrNotFound
	}
	categories, err := s.repository.ListPublishedMenu(ctx, venueID)
	if categories == nil && err == nil {
		categories = []PublicMenuCategory{}
	}
	return categories, err
}

func (s *Service) ListPublishedReviews(ctx context.Context, venueID string) ([]PublicReview, error) {
	if !validVenueID(venueID) {
		return nil, appfault.ErrInvalidInput
	}
	published, err := s.repository.IsPublished(ctx, venueID)
	if err != nil {
		return nil, err
	}
	if !published {
		return nil, appfault.ErrNotFound
	}
	reviews, err := s.repository.ListPublishedReviews(ctx, venueID)
	if reviews == nil && err == nil {
		reviews = []PublicReview{}
	}
	return reviews, err
}

func (s *Service) IsPublished(ctx context.Context, venueID string) (bool, error) {
	if !validVenueID(venueID) {
		return false, appfault.ErrInvalidInput
	}
	return s.repository.IsPublished(ctx, venueID)
}

var venueIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func validVenueID(id string) bool { return venueIDPattern.MatchString(id) }

func (s *Service) Create(ctx context.Context, ownerID string, input Input) (Venue, error) {
	input = normalizeInput(input)
	if !validInput(input) {
		return Venue{}, appfault.ErrInvalidInput
	}
	slug, err := uniqueSlug(input.Name)
	if err != nil {
		return Venue{}, err
	}
	return s.repository.Create(ctx, ownerID, slug, input)
}

func (s *Service) Update(ctx context.Context, ownerID, venueID string, input Input) (Venue, error) {
	input = normalizeInput(input)
	if !validInput(input) {
		return Venue{}, appfault.ErrInvalidInput
	}
	return s.repository.Update(ctx, ownerID, venueID, input)
}

func (s *Service) Delete(ctx context.Context, ownerID, venueID string) error {
	return s.repository.Delete(ctx, ownerID, venueID)
}

func normalizeInput(input Input) Input {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.CuisineType = strings.TrimSpace(input.CuisineType)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Email = strings.TrimSpace(input.Email)
	input.Address = strings.TrimSpace(input.Address)
	input.City = strings.TrimSpace(input.City)
	input.PostalCode = strings.TrimSpace(input.PostalCode)
	input.CountryCode = strings.ToUpper(strings.TrimSpace(input.CountryCode))
	input.Timezone = strings.TrimSpace(input.Timezone)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Status = strings.TrimSpace(input.Status)

	if input.Address == "" {
		input.Address = "Адрес не указан"
	}
	if input.CountryCode == "" {
		input.CountryCode = "SK"
	}
	if input.Timezone == "" {
		input.Timezone = "Europe/Bratislava"
	}
	if input.Currency == "" {
		input.Currency = "EUR"
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.Settings == nil {
		input.Settings = map[string]any{}
	}
	return input
}

func validInput(input Input) bool {
	validStatus := regexp.MustCompile(`^(draft|active|paused|closed)$`).MatchString(input.Status)
	return input.Name != "" && input.City != "" && len(input.CountryCode) == 2 && len(input.Currency) == 3 && validStatus
}

func uniqueSlug(name string) (string, error) {
	token, err := security.NewSessionToken()
	if err != nil {
		return "", err
	}
	base := slugify(name)
	if base == "" {
		base = "venue"
	}
	suffix := slugify(token)
	if len(suffix) > 7 {
		suffix = strings.TrimRight(suffix[:7], "-")
	}
	if suffix == "" {
		return "", errors.New("failed to generate venue slug")
	}
	return fmt.Sprintf("%s-%s", base, suffix), nil
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}

func (s *Service) ListWorkplaces(ctx context.Context, userID string) ([]Venue, error) {
	return s.repository.ListWorkplaces(ctx, userID)
}
