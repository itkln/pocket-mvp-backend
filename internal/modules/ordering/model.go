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
	Name           string `json:"name"`
	Quantity       int    `json:"quantity"`
	UnitPriceMinor int    `json:"unit_price_minor"`
}
