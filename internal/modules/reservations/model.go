package reservations

import (
	"time"

	"pocket-mvp-backend/internal/gratuity"
	"pocket-mvp-backend/internal/money"
)

type PreorderItem struct {
	MenuItemID         string      `json:"menu_item_id"`
	Quantity           int         `json:"quantity"`
	ExpectedPriceMinor money.Minor `json:"expected_price_minor"`
}

type CreateInput struct {
	Tip                *gratuity.Selection `json:"tip,omitempty"`
	ExpectedTotalMinor *money.Minor        `json:"expected_total_minor,omitempty"`
	StartsAt           time.Time           `json:"starts_at"`
	Guests             int                 `json:"guests"`
	CustomerUserID     string              `json:"-"`
	GuestName          string              `json:"guest_name"`
	GuestPhone         string              `json:"guest_phone"`
	GuestEmail         string              `json:"guest_email"`
	Notes              string              `json:"notes"`
	Items              []PreorderItem      `json:"items"`
	IdempotencyKey     string              `json:"idempotency_key"`
}

type Result struct {
	PreorderSubtotalMinor money.Minor `json:"preorder_subtotal_minor"`
	PreorderTipMinor      money.Minor `json:"preorder_tip_minor"`
	ID                    string      `json:"id"`
	Code                  string      `json:"code"`
	StartsAt              time.Time   `json:"starts_at"`
	GuestCount            int         `json:"guest_count"`
	PreorderTotalMinor    money.Minor `json:"preorder_total_minor"`
	PreorderOrderNumber   int64       `json:"preorder_order_number,omitempty"`
	Currency              string      `json:"currency"`
}

type HoldResult struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Slot struct {
	TableID  string
	StartsAt time.Time
	EndsAt   time.Time
}
