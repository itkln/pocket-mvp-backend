package reservations

import (
	"context"
	"time"

	"pocket-mvp-backend/internal/money"
	"pocket-mvp-backend/internal/validate"

	"pocket-mvp-backend/internal/appfault"
)

type PreorderSnapshot struct {
	Name           string      `json:"name"`
	Quantity       int         `json:"quantity"`
	UnitPriceMinor money.Minor `json:"unit_price_minor"`
}

type CustomerReservation struct {
	ID                  string             `json:"id"`
	Code                string             `json:"code"`
	VenueID             string             `json:"venue_id"`
	VenueName           string             `json:"venue_name"`
	VenueAddress        string             `json:"venue_address"`
	VenueCity           string             `json:"venue_city"`
	Timezone            string             `json:"timezone"`
	StartsAt            time.Time          `json:"starts_at"`
	EndsAt              time.Time          `json:"ends_at"`
	GuestCount          int                `json:"guest_count"`
	Status              string             `json:"status"`
	GuestName           string             `json:"guest_name"`
	GuestPhone          string             `json:"guest_phone,omitempty"`
	GuestEmail          string             `json:"guest_email,omitempty"`
	Notes               string             `json:"notes,omitempty"`
	PreorderTipMinor    money.Minor        `json:"preorder_tip_minor"`
	PreorderTotalMinor  money.Minor        `json:"preorder_total_minor"`
	PreorderOrderNumber int64              `json:"preorder_order_number,omitempty"`
	Currency            string             `json:"currency"`
	PreorderItems       []PreorderSnapshot `json:"preorder_items"`
	PreorderStatus      string             `json:"preorder_status,omitempty"`
	Cancellation        Cancellation       `json:"cancellation"`
}

func (s *Service) CancelCustomer(ctx context.Context, customerID, reservationID string) (CustomerReservation, error) {
	if !validate.UUID(customerID) || !validate.UUID(reservationID) {
		return CustomerReservation{}, appfault.ErrInvalidInput
	}
	return s.repository.CancelCustomer(ctx, customerID, reservationID)
}

func (s *Service) ListCustomer(ctx context.Context, customerID string) ([]CustomerReservation, error) {
	if !validate.UUID(customerID) {
		return nil, appfault.ErrInvalidInput
	}
	return s.repository.ListCustomer(ctx, customerID)
}
