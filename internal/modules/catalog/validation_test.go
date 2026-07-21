package catalog

import "testing"

func TestMenuItemRequiresCategory(t *testing.T) {
	input := normalizeMenuItem(MenuItemInput{Name: "Dish", PriceMinor: 100, Currency: "eur"})
	if validMenuItem(input) {
		t.Fatal("menu item without category must be rejected")
	}
	if input.Currency != "EUR" {
		t.Fatalf("currency was not normalized: %q", input.Currency)
	}
}
