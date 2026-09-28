package reservations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"pocket-mvp-backend/internal/appfault"
)

var ErrPriceChanged = errors.New("preorder price changed")
var ErrItemUnavailable = errors.New("preorder item unavailable")
var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type PreorderItem struct {
	MenuItemID         string `json:"menu_item_id"`
	Quantity           int    `json:"quantity"`
	ExpectedPriceMinor int    `json:"expected_price_minor"`
}

type CreateInput struct {
	StartsAt       time.Time      `json:"starts_at"`
	Guests         int            `json:"guests"`
	GuestName      string         `json:"guest_name"`
	GuestPhone     string         `json:"guest_phone"`
	GuestEmail     string         `json:"guest_email"`
	Notes          string         `json:"notes"`
	Items          []PreorderItem `json:"items"`
	IdempotencyKey string         `json:"idempotency_key"`
}

type Result struct {
	ID                 string    `json:"id"`
	Code               string    `json:"code"`
	StartsAt           time.Time `json:"starts_at"`
	GuestCount         int       `json:"guest_count"`
	PreorderTotalMinor int       `json:"preorder_total_minor"`
	Currency           string    `json:"currency"`
}

type Slot struct {
	TableID  string
	StartsAt time.Time
	EndsAt   time.Time
}

type Availability interface {
	FindSlot(context.Context, string, time.Time, int) (Slot, error)
}

type Repository interface {
	FindByKey(context.Context, string, string) (Result, string, bool, error)
	Create(context.Context, string, CreateInput, string, Slot) (Result, error)
}

type Service struct {
	repository   Repository
	availability Availability
}

func NewService(repository Repository, availability Availability) *Service {
	return &Service{repository: repository, availability: availability}
}

func (s *Service) Create(ctx context.Context, venueID string, input CreateInput) (Result, error) {
	input.GuestName = strings.TrimSpace(input.GuestName)
	input.GuestPhone = strings.TrimSpace(input.GuestPhone)
	input.GuestEmail = strings.TrimSpace(input.GuestEmail)
	input.Notes = strings.TrimSpace(input.Notes)
	if !uuidPattern.MatchString(venueID) || input.StartsAt.IsZero() || input.Guests < 1 || input.Guests > 20 || len(input.GuestName) < 2 || len(input.GuestName) > 120 ||
		len(input.GuestPhone) < 5 || len(input.GuestPhone) > 40 || len(input.GuestEmail) > 254 || len(input.Notes) > 500 ||
		len(input.Items) > 30 || len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 {
		return Result{}, appfault.ErrInvalidInput
	}
	seen := make(map[string]bool, len(input.Items))
	for _, item := range input.Items {
		if !uuidPattern.MatchString(item.MenuItemID) || item.Quantity < 1 || item.Quantity > 30 || item.ExpectedPriceMinor < 0 || seen[item.MenuItemID] {
			return Result{}, appfault.ErrInvalidInput
		}
		seen[item.MenuItemID] = true
	}
	encoded, _ := json.Marshal(input)
	checksum := sha256.Sum256(encoded)
	requestHash := hex.EncodeToString(checksum[:])
	if existing, oldHash, found, err := s.repository.FindByKey(ctx, venueID, input.IdempotencyKey); err != nil {
		return Result{}, err
	} else if found {
		if oldHash != requestHash {
			return Result{}, appfault.ErrConflict
		}
		return existing, nil
	}
	slot, err := s.availability.FindSlot(ctx, venueID, input.StartsAt, input.Guests)
	if err != nil {
		if errors.Is(err, appfault.ErrConflict) {
			if existing, oldHash, found, lookupErr := s.repository.FindByKey(ctx, venueID, input.IdempotencyKey); lookupErr == nil && found && oldHash == requestHash {
				return existing, nil
			}
		}
		return Result{}, err
	}
	return s.repository.Create(ctx, venueID, input, requestHash, slot)
}
