package reservations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"pocket-mvp-backend/internal/validate"
	"strings"
	"unicode/utf8"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/gratuity"
)

var ErrPriceChanged = errors.New("preorder price changed")
var ErrItemUnavailable = errors.New("preorder item unavailable")
var ErrHoldExpired = errors.New("reservation hold expired")

type Service struct {
	repository   Repository
	availability Availability
}

func NewService(repository Repository, availability Availability) *Service {
	return &Service{repository: repository, availability: availability}
}

func normalizeAndValidate(venueID string, input CreateInput) (CreateInput, error) {
	if err := gratuity.Validate(input.Tip); err != nil {
		return CreateInput{}, err
	}
	// Tips are offered only when ordering at a table, never with a reservation.
	if input.Tip != nil && (input.Tip.Percent > 0 || input.Tip.AmountMinor > 0) {
		return CreateInput{}, appfault.ErrInvalidInput
	}
	if input.ExpectedTotalMinor != nil && (*input.ExpectedTotalMinor < 0 || *input.ExpectedTotalMinor > gratuity.MaxTotal) {
		return CreateInput{}, appfault.ErrInvalidInput
	}
	input.GuestName = strings.TrimSpace(input.GuestName)
	input.GuestPhone = strings.TrimSpace(input.GuestPhone)
	input.GuestEmail = strings.TrimSpace(input.GuestEmail)
	input.Notes = strings.TrimSpace(input.Notes)
	if !validate.UUID(venueID) || input.StartsAt.IsZero() || input.Guests < 1 || input.Guests > 20 || len(input.GuestName) < 2 || len(input.GuestName) > 120 ||
		(input.GuestPhone == "" && input.GuestEmail == "") || (input.GuestPhone != "" && len(input.GuestPhone) < 5) || len(input.GuestPhone) > 40 || len(input.GuestEmail) > 254 || utf8.RuneCountInString(input.Notes) > 500 ||
		(input.CustomerUserID != "" && !validate.UUID(input.CustomerUserID)) ||
		len(input.Items) > 30 || len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 {
		return CreateInput{}, appfault.ErrInvalidInput
	}
	seen := make(map[string]bool, len(input.Items))
	for _, item := range input.Items {
		if !validate.UUID(item.MenuItemID) || item.Quantity < 1 || item.Quantity > 30 || item.ExpectedPriceMinor < 0 || seen[item.MenuItemID] {
			return CreateInput{}, appfault.ErrInvalidInput
		}
		seen[item.MenuItemID] = true
	}
	return input, nil
}

func requestHash(input CreateInput) string {
	var hashInput any = input
	if input.CustomerUserID != "" {
		hashInput = struct {
			CreateInput
			CustomerID string `json:"customer_user_id"`
		}{input, input.CustomerUserID}
	}
	encoded, _ := json.Marshal(hashInput)
	checksum := sha256.Sum256(encoded)
	return hex.EncodeToString(checksum[:])
}

func holdHash(input CreateInput) string {
	return requestHash(CreateInput{
		StartsAt: input.StartsAt, Guests: input.Guests, CustomerUserID: input.CustomerUserID,
		IdempotencyKey: input.IdempotencyKey,
	})
}

func (s *Service) Create(ctx context.Context, venueID string, input CreateInput) (Result, error) {
	var err error
	input, err = normalizeAndValidate(venueID, input)
	if err != nil {
		return Result{}, err
	}
	requestHash := requestHash(input)
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

func (s *Service) Hold(ctx context.Context, venueID string, input CreateInput) (HoldResult, error) {
	var err error
	input, err = normalizeAndValidate(venueID, input)
	if err != nil {
		return HoldResult{}, err
	}
	slot, err := s.availability.FindSlot(ctx, venueID, input.StartsAt, input.Guests)
	if err != nil {
		if errors.Is(err, appfault.ErrConflict) {
			// The same operation may already own the only free table.
			if existing, lookupErr := s.repository.Hold(ctx, venueID, input, holdHash(input), Slot{}); lookupErr == nil {
				return existing, nil
			} else if errors.Is(lookupErr, ErrHoldExpired) {
				return HoldResult{}, lookupErr
			}
		}
		return HoldResult{}, err
	}
	return s.repository.Hold(ctx, venueID, input, holdHash(input), slot)
}

func (s *Service) Confirm(ctx context.Context, venueID, holdID string, input CreateInput) (Result, error) {
	var err error
	input, err = normalizeAndValidate(venueID, input)
	if err != nil || !validate.UUID(holdID) {
		return Result{}, appfault.ErrInvalidInput
	}
	return s.repository.Confirm(ctx, venueID, holdID, input, holdHash(input), requestHash(input))
}

func (s *Service) GetConfirmed(ctx context.Context, venueID, reservationID, key string) (Result, error) {
	if !validate.UUID(venueID) || !validate.UUID(reservationID) || len(key) < 16 || len(key) > 128 {
		return Result{}, appfault.ErrInvalidInput
	}
	return s.repository.GetConfirmed(ctx, venueID, reservationID, key)
}
