package ordering

import (
	"encoding/json"
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
	kitchenOnly         bool
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
	TipMinor            int         `json:"tip_minor"`
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
	ID             string `json:"id,omitempty"`
	TotalMinor     int    `json:"total_minor"`
	Notes          string `json:"notes,omitempty"`
	Status         string `json:"status,omitempty"`
	Name           string `json:"name"`
	Quantity       int    `json:"quantity"`
	UnitPriceMinor int    `json:"unit_price_minor"`
}

// Kitchen transport deliberately excludes financial and guest identity fields.
func (o Order) MarshalJSON() ([]byte, error) {
	type raw Order
	if !o.kitchenOnly {
		return json.Marshal(raw(o))
	}
	type line struct {
		ID       string `json:"id,omitempty"`
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
		Notes    string `json:"notes,omitempty"`
		Status   string `json:"status,omitempty"`
	}
	items := make([]line, 0, len(o.Items))
	for _, i := range o.Items {
		items = append(items, line{i.ID, i.Name, i.Quantity, i.Notes, i.Status})
	}
	return json.Marshal(map[string]any{
		"id": o.ID, "number": o.Number, "channel": o.Channel, "source": o.Source, "status": o.Status, "notes": o.Notes,
		"created_at": o.CreatedAt, "updated_at": o.UpdatedAt, "placed_at": o.PlacedAt, "completed_at": o.CompletedAt,
		"reservation_status": o.ReservationStatus, "reservation_starts_at": o.ReservationStartsAt, "reservation_id": o.ReservationID, "table_id": o.TableID, "table_label": o.TableLabel,
		"allowed_actions": o.AllowedActions, "items": items,
	})
}
