package venues

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/security"
)

type VenueAuthorizer interface {
	Require(context.Context, string, string, access.Capability) error
}

type Service struct {
	repository Repository
	authorizer VenueAuthorizer
}

func NewService(repository Repository, authorizer VenueAuthorizer) *Service {
	return &Service{repository: repository, authorizer: authorizer}
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

// Get returns the full venue profile to those who may edit it.
func (s *Service) Get(ctx context.Context, userID, venueID string) (Venue, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.ManageSettings); err != nil {
		return Venue{}, err
	}
	return s.repository.Get(ctx, venueID)
}

func (s *Service) Update(ctx context.Context, userID, venueID string, input Input) (Venue, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.ManageSettings); err != nil {
		return Venue{}, err
	}
	input = normalizeInput(input)
	if !validInput(input) {
		return Venue{}, appfault.ErrInvalidInput
	}
	return s.repository.Update(ctx, venueID, input)
}

// Delete stays with the owner: managers edit the venue but may not remove it.
func (s *Service) Delete(ctx context.Context, userID, venueID string) error {
	if err := s.authorizer.Require(ctx, userID, venueID, access.ManageVenue); err != nil {
		return err
	}
	return s.repository.Delete(ctx, venueID)
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

// ListWorkplaces returns the venues a user works at. Those who may edit a venue get its full
// profile; the others get operational context only, without private settings or owner contacts.
func (s *Service) ListWorkplaces(ctx context.Context, userID string) ([]Venue, error) {
	items, err := s.repository.ListWorkplaces(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i, venue := range items {
		if !access.Allows(venue.AccessRole, access.ManageSettings) {
			items[i] = operationalView(venue)
		}
	}
	return items, nil
}

func operationalView(venue Venue) Venue {
	return Venue{
		AccessRole: venue.AccessRole, ID: venue.ID, Name: venue.Name, Slug: venue.Slug,
		Address: venue.Address, City: venue.City, CountryCode: venue.CountryCode,
		Timezone: venue.Timezone, Currency: venue.Currency, Status: venue.Status,
		Settings: map[string]any{},
	}
}
