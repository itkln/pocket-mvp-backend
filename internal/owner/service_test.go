package owner

import "testing"

func TestNormalizeVenueInputUsesSafeDefaults(t *testing.T) {
	input := normalizeVenueInput(VenueInput{Name: "  Mokka  ", City: " Bratislava "})
	if input.Name != "Mokka" || input.City != "Bratislava" {
		t.Fatalf("input was not trimmed: %#v", input)
	}
	if input.CountryCode != "SK" || input.Currency != "EUR" || input.Timezone != "Europe/Bratislava" || input.Status != "draft" {
		t.Fatalf("defaults were not applied: %#v", input)
	}
	if !validVenueInput(input) {
		t.Fatal("normalized venue should be valid")
	}
}

func TestOwnerInputValidation(t *testing.T) {
	if validStaffRole("admin") {
		t.Fatal("unsupported staff role must be rejected")
	}
	if validMenuItem(MenuItemInput{Name: "Dish", CategoryID: "", PriceMinor: 100, Currency: "EUR"}) {
		t.Fatal("menu item without category must be rejected")
	}
}
