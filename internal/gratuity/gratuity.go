// Package gratuity calculates optional tips in integer minor units.
package gratuity

import (
	"pocket-mvp-backend/internal/appfault"
)

const MaxTotal = 2_000_000_000

type Selection struct {
	Mode        string `json:"mode"`
	Percent     int    `json:"percent,omitempty"`
	AmountMinor int    `json:"amount_minor,omitempty"`
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
func Calculate(subtotal int, s *Selection) (int, error) {
	if err := Validate(s); err != nil {
		return 0, err
	}
	if subtotal < 0 || subtotal > MaxTotal {
		return 0, appfault.ErrInvalidInput
	}
	var amount int64
	if s != nil {
		if s.Mode == "percent" {
			amount = (int64(subtotal)*int64(s.Percent) + 50) / 100
		}
		if s.Mode == "custom" {
			amount = int64(s.AmountMinor)
		}
	}
	if int64(subtotal)+amount > MaxTotal || (subtotal == 0 && amount != 0) {
		return 0, appfault.ErrInvalidInput
	}
	return int(amount), nil
}
