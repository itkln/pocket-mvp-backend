package dinein

import (
	"context"
	"time"

	"pocket-mvp-backend/internal/money"
)

// Bill is the current table visit. No guest identities or private order notes
// are exposed to other phones holding the table QR capability.
type Bill struct {
	ID              string    `json:"id"`
	VenueID         string    `json:"venue_id"`
	TableID         string    `json:"table_id"`
	TableIdentifier string    `json:"table_identifier"`
	Status          string    `json:"status"`
	Currency        string    `json:"currency"`
	OpenedAt        time.Time `json:"opened_at"`
	// PaymentRequestedAt: when a guest called the staff to settle (V-TBL-05). It is not a payment.
	PaymentRequestedAt *time.Time  `json:"payment_requested_at"`
	SubtotalMinor      money.Minor `json:"subtotal_minor"`
	TipMinor           money.Minor `json:"tip_minor"`
	TaxMinor           money.Minor `json:"tax_minor"`
	ServiceFeeMinor    money.Minor `json:"service_fee_minor"`
	TotalMinor         money.Minor `json:"total_minor"`
	Items              []BillItem  `json:"items"`
}
type BillItem struct {
	OrderID        string      `json:"order_id"`
	OrderNumber    int64       `json:"order_number"`
	Name           string      `json:"name"`
	Quantity       int         `json:"quantity"`
	UnitPriceMinor money.Minor `json:"unit_price_minor"`
	TotalMinor     money.Minor `json:"total_minor"`
}

func (s *Service) CurrentBill(ctx context.Context, token string) (*Bill, error) {
	if !validToken(token) {
		return nil, ErrQRInvalid
	}
	return s.repository.CurrentBill(ctx, tokenHash(token))
}

// CallStaff asks the staff to come and settle the current visit. Repeated calls keep the first
// time and create nothing new; the visit stays open (and orderable) until a staff member confirms
// the payment. Without an active visit there is nothing to settle (not_found).
func (s *Service) CallStaff(ctx context.Context, token string) (*Bill, error) {
	if !validToken(token) {
		return nil, ErrQRInvalid
	}
	return s.repository.CallStaff(ctx, tokenHash(token))
}
