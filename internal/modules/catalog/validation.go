package catalog

import "strings"

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
	return input.CategoryID != "" && input.Name != "" && input.PriceMinor >= 0 && len(input.Currency) == 3
}
