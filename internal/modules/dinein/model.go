package dinein

import (
	"encoding/json"
	"pocket-mvp-backend/internal/gratuity"
	"pocket-mvp-backend/internal/money"
	"time"
)

type Table struct {
	VenueName       string `json:"venue_name"`
	VenueID         string `json:"venue_id"`
	ID              string `json:"table_id"`
	Identifier      string `json:"table_identifier"`
	Version         int    `json:"version"`
	Currency        string `json:"currency"`
	OrderingEnabled bool   `json:"ordering_enabled"`
}
type QRCode struct {
	Table
	URL       string `json:"url"`
	Encrypted string `json:"-"`
}
type ItemInput struct {
	MenuItemID         string      `json:"menu_item_id"`
	Quantity           int         `json:"quantity"`
	ExpectedPriceMinor money.Minor `json:"expected_price_minor"`
}
type CreateInput struct {
	Tip                *gratuity.Selection `json:"tip,omitempty"`
	ExpectedTotalMinor *money.Minor        `json:"expected_total_minor,omitempty"`
	Token              string              `json:"token"`
	IdempotencyKey     string              `json:"idempotency_key"`
	Notes              string              `json:"notes"`
	Items              []ItemInput         `json:"items"`
	CustomerUserID     string              `json:"-"`
	GuestToken         string              `json:"-"`
	GuestHash          string              `json:"-"`
	LegacyRequestHash  string              `json:"-"`
}
type Item struct {
	MenuItemID     string          `json:"menu_item_id,omitempty"`
	Modifiers      json.RawMessage `json:"modifiers,omitempty"`
	Name           string          `json:"name"`
	Quantity       int             `json:"quantity"`
	UnitPriceMinor money.Minor     `json:"unit_price_minor"`
}
type Order struct {
	SubtotalMinor   money.Minor `json:"subtotal_minor"`
	TipMinor        money.Minor `json:"tip_minor"`
	VenueID         string      `json:"venue_id"`
	QRVersion       *int        `json:"table_qr_version,omitempty"`
	ID              string      `json:"id"`
	Number          int64       `json:"number"`
	Status          string      `json:"status"`
	TableIdentifier string      `json:"table_identifier"`
	TotalMinor      money.Minor `json:"total_minor"`
	Currency        string      `json:"currency"`
	Notes           string      `json:"notes"`
	CreatedAt       time.Time   `json:"created_at"`
	Items           []Item      `json:"items"`
}
