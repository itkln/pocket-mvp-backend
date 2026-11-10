package dinein

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"pocket-mvp-backend/internal/validate"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/gratuity"
)

var (
	ErrQRInvalid       = errors.New("qr_invalid")
	ErrPriceChanged    = errors.New("price_changed")
	ErrItemUnavailable = errors.New("item_unavailable")
	ErrBillLocked      = errors.New("bill_locked")
)

type Service struct {
	repository Repository
	authorizer Authorizer
	protector  Protector
	baseURL    string
}

func NewService(r Repository, a Authorizer, p Protector, baseURL string) *Service {
	return &Service{r, a, p, strings.TrimRight(baseURL, "/")}
}
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func validToken(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == 32
}
func (s *Service) Create(ctx context.Context, input CreateInput) (Order, error) {
	if !validToken(input.Token) {
		return Order{}, ErrQRInvalid
	}
	if err := gratuity.Validate(input.Tip); err != nil {
		return Order{}, err
	}
	if input.ExpectedTotalMinor != nil && (*input.ExpectedTotalMinor < 0 || *input.ExpectedTotalMinor > gratuity.MaxTotal) {
		return Order{}, appfault.ErrInvalidInput
	}
	input.Notes = strings.TrimSpace(input.Notes)
	if len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 || utf8.RuneCountInString(input.Notes) > 500 || len(input.Items) == 0 || len(input.Items) > 100 {
		return Order{}, appfault.ErrInvalidInput
	}
	seen := map[string]bool{}
	for _, item := range input.Items {
		if !validate.UUID(item.MenuItemID) || item.Quantity < 1 || item.Quantity > 30 || item.ExpectedPriceMinor < 0 || seen[item.MenuItemID] {
			return Order{}, appfault.ErrInvalidInput
		}
		seen[item.MenuItemID] = true
	}
	// The QR capability and account are included so a key cannot move an order to another table/user.
	body, _ := json.Marshal(input)
	sum := sha256.Sum256(append(body, []byte(input.CustomerUserID)...))
	input.LegacyRequestHash = hex.EncodeToString(sum[:])
	if input.CustomerUserID == "" && input.GuestToken != "" {
		if !validToken(input.GuestToken) {
			return Order{}, appfault.ErrInvalidInput
		}
		input.GuestHash = tokenHash(input.GuestToken)
		sum = sha256.Sum256(append(body, []byte(input.CustomerUserID+"|guest:"+input.GuestHash)...))
	}
	return s.repository.Create(ctx, tokenHash(input.Token), hex.EncodeToString(sum[:]), input)
}
func (s *Service) Lookup(ctx context.Context, token, key string, identities ...HistoryInput) (Order, error) {
	if !validToken(token) {
		return Order{}, ErrQRInvalid
	}
	if len(key) < 16 || len(key) > 128 {
		return Order{}, appfault.ErrInvalidInput
	}
	customerID, guestHash := "", ""
	if len(identities) > 0 {
		customerID = identities[0].CustomerUserID
		if customerID == "" && validToken(identities[0].GuestToken) {
			guestHash = tokenHash(identities[0].GuestToken)
		}
	}
	return s.repository.Lookup(ctx, tokenHash(token), key, customerID, guestHash)
}
