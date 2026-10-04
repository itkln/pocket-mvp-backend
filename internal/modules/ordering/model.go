package ordering

import "time"

type Order struct {
	ID                  string      `json:"id"`
	Number              int64       `json:"number"`
	Channel             string      `json:"channel"`
	Source              string      `json:"source"`
	GuestName           string      `json:"guest_name,omitempty"`
	TotalMinor          int         `json:"total_minor"`
	Currency            string      `json:"currency"`
	Status              string      `json:"status"`
	Notes               string      `json:"notes,omitempty"`
	CreatedAt           time.Time   `json:"created_at"`
	ReservationCode     string      `json:"reservation_code,omitempty"`
	ReservationStartsAt *time.Time  `json:"reservation_starts_at,omitempty"`
	GuestPhone          string      `json:"guest_phone,omitempty"`
	GuestCount          int         `json:"guest_count,omitempty"`
	Items               []OrderItem `json:"items,omitempty"`
	VenueName           string      `json:"venue_name,omitempty"`
}

type OrderItem struct {
	MenuItemID     string `json:"menu_item_id,omitempty"`
	Name           string `json:"name"`
	Quantity       int    `json:"quantity"`
	UnitPriceMinor int    `json:"unit_price_minor"`
	TotalMinor     int    `json:"total_minor"`
}

type CreateItem struct {
	MenuItemID         string `json:"menu_item_id"`
	Quantity           int    `json:"quantity"`
	ExpectedPriceMinor int    `json:"expected_price_minor"`
}

type CreateInput struct {
	CustomerUserID string       `json:"-"`
	GuestName      string       `json:"guest_name"`
	GuestEmail     string       `json:"guest_email"`
	Notes          string       `json:"notes"`
	Items          []CreateItem `json:"items"`
	IdempotencyKey string       `json:"idempotency_key"`
}

type Quote struct {
	Currency   string      `json:"currency"`
	TotalMinor int         `json:"total_minor"`
	Items      []OrderItem `json:"items"`
}
