// Package gratuity calculates optional tips in integer minor units.
package gratuity

import (
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/money"
)

// MaxTotal caps an order or bill total, tip included (it must fit the integer columns).
const MaxTotal money.Minor = 2_000_000_000

type Selection struct {
	Mode        string      `json:"mode"`
	Percent     int         `json:"percent,omitempty"`
	AmountMinor money.Minor `json:"amount_minor,omitempty"`
}

func Validate(s *Selection) error {
	if s == nil {
		return nil
	}
	switch s.Mode {
	case "none":
		if s.Percent == 0 && s.AmountMinor == 0 {
			return nil
		}
	case "percent":
		if s.Percent >= 0 && s.Percent <= 100 && s.AmountMinor == 0 {
			return nil
		}
	case "custom":
		if s.Percent == 0 && s.AmountMinor >= 0 && s.AmountMinor <= MaxTotal {
			return nil
		}
	}
	return appfault.ErrInvalidInput
}

// Calculate rounds a percentage half up to the nearest minor unit, once per order.
func Calculate(subtotal money.Minor, s *Selection) (money.Minor, error) {
	if err := Validate(s); err != nil {
		return 0, err
	}
	if subtotal < 0 || subtotal > MaxTotal {
		return 0, appfault.ErrInvalidInput
	}
	var amount money.Minor
	if s != nil {
		if s.Mode == "percent" {
			amount = (subtotal.Times(s.Percent) + 50) / 100
		}
		if s.Mode == "custom" {
			amount = s.AmountMinor
		}
	}
	if subtotal+amount > MaxTotal || (subtotal == 0 && amount != 0) {
		return 0, appfault.ErrInvalidInput
	}
	return amount, nil
}
