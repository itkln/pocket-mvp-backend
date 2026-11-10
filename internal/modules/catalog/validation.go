package catalog

import (
	"math"
	"strings"
)

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func normalizeMenuItem(input MenuItemInput) MenuItemInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if input.Currency == "" {
		input.Currency = "EUR"
	}
	return input
}

func validMenuItem(input MenuItemInput) bool {
	if input.CategoryID == "" || input.Name == "" || input.PriceMinor < 0 || len(input.Currency) != 3 {
		return false
	}
	if input.PortionWeightG != nil && (*input.PortionWeightG <= 0 || *input.PortionWeightG > math.MaxInt32) {
		return false
	}
	for _, nutrient := range []struct {
		value   *float64
		maximum float64
	}{
		{input.EnergyKcal, 999999.99}, {input.ProteinG, 100}, {input.FatG, 100}, {input.CarbsG, 100},
	} {
		if nutrient.value != nil && (math.IsNaN(*nutrient.value) || math.IsInf(*nutrient.value, 0) || *nutrient.value < 0 || *nutrient.value > nutrient.maximum) {
			return false
		}
	}
	return true
}
