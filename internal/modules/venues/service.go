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

func (s *Service) SearchPublished(ctx context.Context, query PublicVenueQuery) (PublicVenuePage, error) {
	query = normalizePublicQuery(query)
	if !validPublicQuery(query) {
		return PublicVenuePage{}, appfault.ErrInvalidInput
	}
	items, total, err := s.repository.SearchPublished(ctx, query)
	if err != nil {
		return PublicVenuePage{}, err
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + query.PageSize - 1) / query.PageSize
	}
	if items == nil {
		items = []PublicVenue{}
	}
	return PublicVenuePage{
		Venues: items, Page: query.Page, PageSize: query.PageSize, Total: total,
		TotalPages: totalPages, HasNext: query.Page < totalPages,
	}, nil
}

func (s *Service) GetPublished(ctx context.Context, venueID string) (PublicVenue, error) {
	if !validVenueID(venueID) {
		return PublicVenue{}, appfault.ErrInvalidInput
	}
	return s.repository.GetPublished(ctx, venueID)
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

func normalizePublicQuery(query PublicVenueQuery) PublicVenueQuery {
	query.City = strings.TrimSpace(query.City)
	query.Search = strings.TrimSpace(query.Search)
	query.Category = strings.ToLower(strings.TrimSpace(query.Category))
	query.Filter = strings.TrimSpace(query.Filter)
	if query.Filter == "Все" || strings.EqualFold(query.Filter, "all") {
		query.Filter = ""
	}
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 24
	}
	if query.IDs == nil {
		query.IDs = []string{}
	}
	return query
}

func validPublicQuery(query PublicVenueQuery) bool {
	validCategory := regexp.MustCompile(`^[a-z0-9-]*$`).MatchString(query.Category)
	if len(query.IDs) > 50 {
		return false
	}
	for _, id := range query.IDs {
		if !validVenueID(id) {
			return false
		}
	}
	return query.Page > 0 && query.Page <= 100000 && query.PageSize > 0 && query.PageSize <= 50 &&
		len(query.City) <= 120 && len(query.Search) <= 120 && len(query.Filter) <= 80 && validCategory
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
		suffix = suffix[:7]
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
