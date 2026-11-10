package ordering

import (
	"pocket-mvp-backend/internal/money"
	"time"
)

type StatusEvent struct {
	ID             string    `json:"id"`
	ActorID        string    `json:"actor_id,omitempty"`
	ActorName      string    `json:"actor_name"`
	PreviousStatus string    `json:"previous_status"`
	NewStatus      string    `json:"new_status"`
	Reason         string    `json:"reason,omitempty"`
	Comment        string    `json:"comment,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}
type Order struct {
	CancellationBlocked string        `json:"cancellation_blocked,omitempty"`
	StatusEvents        []StatusEvent `json:"status_events,omitempty"`
	cancelAllowed       bool
	ReservationStatus   string      `json:"reservation_status,omitempty"`
	AllowedActions      []string    `json:"allowed_actions"`
	UpdatedAt           *time.Time  `json:"updated_at,omitempty"`
	PlacedAt            *time.Time  `json:"placed_at"`
	CompletedAt         *time.Time  `json:"completed_at"`
	TableID             string      `json:"table_id,omitempty"`
	TableLabel          string      `json:"table_label,omitempty"`
	ReservationID       string      `json:"reservation_id,omitempty"`
	BillID              string      `json:"bill_id,omitempty"`
	ID                  string      `json:"id"`
	Number              int64       `json:"number"`
	Channel             string      `json:"channel"`
	Source              string      `json:"source"`
	GuestName           string      `json:"guest_name,omitempty"`
	TipMinor            money.Minor `json:"tip_minor"`
	TotalMinor          money.Minor `json:"total_minor"`
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
	ID             string      `json:"id,omitempty"`
	TotalMinor     money.Minor `json:"total_minor"`
	Notes          string      `json:"notes,omitempty"`
	Status         string      `json:"status,omitempty"`
	Name           string      `json:"name"`
	Quantity       int         `json:"quantity"`
	UnitPriceMinor money.Minor `json:"unit_price_minor"`
}
