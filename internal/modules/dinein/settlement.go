package dinein

import (
	"context"
	"errors"
	"time"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/money"
	"pocket-mvp-backend/internal/validate"
)

var (
	// ErrBillChanged: the bill's amount is not what the staff member checked (an order was added or
	// cancelled meanwhile). Nothing is closed; the staff reloads the bill and checks it again.
	ErrBillChanged = errors.New("bill_changed")
	// ErrBillClosed: the visit was closed without an in-person settlement to repeat.
	ErrBillClosed = errors.New("bill_closed")
)

// Payment methods of an in-person settlement.
const (
	MethodCash = "cash"
	MethodCard = "card"
)

// SettleInput is what the staff member confirms: the amount they checked with the guest and how
// the guest paid.
type SettleInput struct {
	ExpectedTotalMinor money.Minor `json:"expected_total_minor"`
	Method             string      `json:"method"`
}

// Settlement is a visit closed by a staff member after the guest paid in person.
type Settlement struct {
	BillID          string      `json:"bill_id"`
	TableIdentifier string      `json:"table_identifier"`
	Status          string      `json:"status"`
	AmountMinor     money.Minor `json:"amount_minor"`
	Currency        string      `json:"currency"`
	Method          string      `json:"method,omitempty"`
	SettledAt       time.Time   `json:"settled_at"`
	SettledByUserID string      `json:"settled_by_user_id,omitempty"`
}

// OpenBills lists the venue's active table visits, the ones whose guests called the staff first.
func (s *Service) OpenBills(ctx context.Context, userID, venueID string) ([]Bill, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.SettleBills); err != nil {
		return nil, err
	}
	return s.repository.OpenBills(ctx, venueID)
}

// Settle records that the guests paid the whole bill in person and closes the visit, atomically.
// A repeated confirmation returns the first settlement. Orders and their kitchen statuses stay
// as they are; the next order at the table opens a new visit.
func (s *Service) Settle(ctx context.Context, userID, venueID, billID string, input SettleInput) (Settlement, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.SettleBills); err != nil {
		return Settlement{}, err
	}
	if !validate.UUID(billID) || input.ExpectedTotalMinor < 0 || (input.Method != MethodCash && input.Method != MethodCard) {
		return Settlement{}, appfault.ErrInvalidInput
	}
	return s.repository.Settle(ctx, venueID, billID, userID, input)
}
