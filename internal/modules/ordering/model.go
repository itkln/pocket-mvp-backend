package ordering

import "time"

type Order struct {
	ID         string    `json:"id"`
	Number     int64     `json:"number"`
	Channel    string    `json:"channel"`
	Source     string    `json:"source"`
	GuestName  string    `json:"guest_name,omitempty"`
	TotalMinor int       `json:"total_minor"`
	Currency   string    `json:"currency"`
	Status     string    `json:"status"`
	Notes      string    `json:"notes,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
