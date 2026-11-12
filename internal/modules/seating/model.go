// Package seating is the venue's live floor: which tables are taken, who is expected today, and
// seating parties — guests with a booking and walk-ins. A table is occupied while a party is
// seated at it or its bill is open (guests who ordered by QR); the staff free it when they leave.
package seating

import (
	"errors"
	"time"

	"pocket-mvp-backend/internal/money"
)

var (
	// ErrTableOccupied: a party is already seated at the table.
	ErrTableOccupied = errors.New("table_occupied")
	// ErrTableReserved: the table is booked for the time the party would spend at it.
	ErrTableReserved = errors.New("table_reserved")
	// ErrNotSeatable: the booking is cancelled, finished or not confirmed yet.
	ErrNotSeatable = errors.New("reservation_not_seatable")
)

const (
	// visit is how long a walk-in is expected to stay, as a booking does.
	visit = 2 * time.Hour
	// reservedAhead marks a free table as reserved this long before its next booking.
	reservedAhead = 2 * time.Hour
	// minimumVisit: a walk-in is not seated when a booking of the table starts sooner than this.
	minimumVisit = 30 * time.Minute
)

const (
	StateFree     = "free"
	StateOccupied = "occupied"
	StateReserved = "reserved"
)

// Party is the group seated at a table.
type Party struct {
	ReservationID string    `json:"reservation_id"`
	Code          string    `json:"code"`
	GuestName     string    `json:"guest_name"`
	GuestCount    int       `json:"guest_count"`
	SeatedAt      time.Time `json:"seated_at"`
	WalkIn        bool      `json:"walk_in"`
}

// Bill is the table's open bill: what the guests ordered so far.
type Bill struct {
	ID         string      `json:"id"`
	OpenedAt   time.Time   `json:"opened_at"`
	TotalMinor money.Minor `json:"total_minor"`
	// PaymentRequestedAt: the guests called the staff to settle (V-TBL-05).
	PaymentRequestedAt *time.Time `json:"payment_requested_at,omitempty"`
}

// Table is the live state of one table, matched to the floor plan by its identifier.
type Table struct {
	ID                string `json:"-"`
	Identifier        string `json:"identifier"`
	Seats             int    `json:"seats"`
	State             string `json:"state"`
	Party             *Party `json:"party,omitempty"`
	Bill              *Bill  `json:"bill,omitempty"`
	NextReservationID string `json:"next_reservation_id,omitempty"`
}

// Booking is a confirmed reservation the venue still expects today.
type Booking struct {
	ID              string    `json:"id"`
	Code            string    `json:"code"`
	TableIdentifier string    `json:"table_identifier"`
	GuestName       string    `json:"guest_name"`
	GuestPhone      string    `json:"guest_phone,omitempty"`
	GuestCount      int       `json:"guest_count"`
	StartsAt        time.Time `json:"starts_at"`
	EndsAt          time.Time `json:"ends_at"`
	Notes           string    `json:"notes,omitempty"`
	HasPreorder     bool      `json:"has_preorder"`
}

// Floor is one consistent reading of the venue floor.
type Floor struct {
	Now      time.Time `json:"now"`
	Timezone string    `json:"timezone"`
	Currency string    `json:"currency"`
	Tables   []Table   `json:"tables"`
	Bookings []Booking `json:"bookings"`
}

// WalkIn is a party seated without a booking.
type WalkIn struct {
	TableIdentifier string
	GuestName       string
	Guests          int
}

// resolveStates derives each table's state from who sits at it and what is booked next.
func resolveStates(floor *Floor) {
	next := make(map[string]Booking, len(floor.Bookings))
	for _, booking := range floor.Bookings { // ordered by start
		if _, seen := next[booking.TableIdentifier]; !seen {
			next[booking.TableIdentifier] = booking
		}
	}
	for index := range floor.Tables {
		table := &floor.Tables[index]
		table.State = StateFree
		booking, booked := next[table.Identifier]
		if booked {
			table.NextReservationID = booking.ID
		}
		switch {
		case table.Party != nil || table.Bill != nil:
			table.State = StateOccupied
		case booked && !booking.StartsAt.After(floor.Now.Add(reservedAhead)):
			table.State = StateReserved
		}
	}
}
