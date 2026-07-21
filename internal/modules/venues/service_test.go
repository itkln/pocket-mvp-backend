package venues

import "testing"

func TestNormalizeInputUsesSafeDefaults(t *testing.T) {
	input := normalizeInput(Input{Name: "  Mokka  ", City: " Bratislava "})
	if input.Name != "Mokka" || input.City != "Bratislava" {
		t.Fatalf("input was not trimmed: %#v", input)
	}
	if input.CountryCode != "SK" || input.Currency != "EUR" || input.Timezone != "Europe/Bratislava" || input.Status != "draft" {
		t.Fatalf("defaults were not applied: %#v", input)
	}
	if !validInput(input) {
		t.Fatal("normalized venue should be valid")
	}
}
