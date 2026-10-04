package ordering

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"regexp"
	"strings"

	"pocket-mvp-backend/internal/appfault"
)

var orderStatusPattern = regexp.MustCompile(`^(new|accepted|preparing|ready|served|completed|cancelled)$`)
var orderUUIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var ErrPriceChanged = errors.New("order price changed")
var ErrItemUnavailable = errors.New("order item unavailable")

type VenueAuthorizer interface {
	RequireOwner(context.Context, string, string) error
}

type Service struct {
	repository Repository
	authorizer VenueAuthorizer
}

func NewService(repository Repository, authorizer VenueAuthorizer) *Service {
	return &Service{repository: repository, authorizer: authorizer}
}

func (s *Service) List(ctx context.Context, ownerID, venueID string) ([]Order, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, venueID)
}

func (s *Service) UpdateStatus(ctx context.Context, ownerID, venueID, orderID, status string) (Order, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return Order{}, err
	}
	if !orderStatusPattern.MatchString(status) {
		return Order{}, appfault.ErrInvalidInput
	}
	return s.repository.UpdateStatus(ctx, venueID, orderID, status)
}

func normalizeOrder(venueID string, input CreateInput, submitting bool) (CreateInput, error) {
	input.GuestName = strings.TrimSpace(input.GuestName)
	input.GuestEmail = strings.TrimSpace(input.GuestEmail)
	input.Notes = strings.TrimSpace(input.Notes)
	if !orderUUIDPattern.MatchString(venueID) || len(input.Items) == 0 || len(input.Items) > 30 ||
		len(input.GuestName) > 120 || len(input.GuestEmail) > 254 || len(input.Notes) > 500 ||
		(input.CustomerUserID != "" && !orderUUIDPattern.MatchString(input.CustomerUserID)) ||
		(submitting && (len(input.GuestName) < 2 || !validOrderEmail(input.GuestEmail) ||
			len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128)) {
		return CreateInput{}, appfault.ErrInvalidInput
	}
	seen := make(map[string]bool, len(input.Items))
	for _, item := range input.Items {
		if !orderUUIDPattern.MatchString(item.MenuItemID) || item.Quantity < 1 || item.Quantity > 30 ||
			item.ExpectedPriceMinor < 0 || seen[item.MenuItemID] {
			return CreateInput{}, appfault.ErrInvalidInput
		}
		seen[item.MenuItemID] = true
	}
	return input, nil
}

func validOrderEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value && strings.Contains(value, "@")
}

func (s *Service) Quote(ctx context.Context, venueID string, input CreateInput) (Quote, error) {
	input, err := normalizeOrder(venueID, input, false)
	if err != nil {
		return Quote{}, err
	}
	return s.repository.Quote(ctx, venueID, input)
}

func (s *Service) Submit(ctx context.Context, venueID string, input CreateInput) (Order, error) {
	input, err := normalizeOrder(venueID, input, true)
	if err != nil {
		return Order{}, err
	}
	encoded, _ := json.Marshal(struct {
		VenueID string `json:"venue_id"`
		CreateInput
		CustomerID string `json:"customer_user_id"`
	}{venueID, input, input.CustomerUserID})
	hash := sha256.Sum256(encoded)
	return s.repository.Submit(ctx, venueID, input, hex.EncodeToString(hash[:]))
}

func (s *Service) ListCustomer(ctx context.Context, customerID string) ([]Order, error) {
	if !orderUUIDPattern.MatchString(customerID) {
		return nil, appfault.ErrInvalidInput
	}
	return s.repository.ListCustomer(ctx, customerID)
}

func (s *Service) GetPublic(ctx context.Context, venueID, orderID, key string) (Order, error) {
	if !orderUUIDPattern.MatchString(venueID) || !orderUUIDPattern.MatchString(orderID) || len(key) < 16 || len(key) > 128 {
		return Order{}, appfault.ErrInvalidInput
	}
	return s.repository.GetPublic(ctx, venueID, orderID, key)
}
