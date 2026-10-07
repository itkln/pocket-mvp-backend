package catalog

import (
	"math"
	"testing"
)

func TestOptionalNutritionValidation(t *testing.T) {
	number := func(value float64) *float64 { return &value }
	weight := func(value int) *int { return &value }
	for _, test := range []struct {
		name  string
		patch func(*MenuItemInput)
		valid bool
	}{
		{"missing values", func(*MenuItemInput) {}, true},
		{"zero nutrients", func(i *MenuItemInput) {
			i.EnergyKcal = number(0)
			i.ProteinG = number(0)
			i.FatG = number(0)
			i.CarbsG = number(0)
		}, true},
		{"decimal values", func(i *MenuItemInput) {
			i.PortionWeightG = weight(250)
			i.EnergyKcal = number(196.5)
			i.ProteinG = number(8.25)
		}, true},
		{"negative energy", func(i *MenuItemInput) { i.EnergyKcal = number(-1) }, false},
		{"too much protein", func(i *MenuItemInput) { i.ProteinG = number(100.01) }, false},
		{"negative fat", func(i *MenuItemInput) { i.FatG = number(-0.1) }, false},
		{"nonfinite carbs", func(i *MenuItemInput) { i.CarbsG = number(math.Inf(1)) }, false},
		{"NaN energy", func(i *MenuItemInput) { i.EnergyKcal = number(math.NaN()) }, false},
		{"overflow energy", func(i *MenuItemInput) { i.EnergyKcal = number(1000000) }, false},
		{"empty portion", func(i *MenuItemInput) { i.PortionWeightG = weight(0) }, false},
		{"negative portion", func(i *MenuItemInput) { i.PortionWeightG = weight(-10) }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := MenuItemInput{CategoryID: "category", Name: "Dish", PriceMinor: 100, Currency: "EUR"}
			test.patch(&input)
			if got := validMenuItem(input); got != test.valid {
				t.Fatalf("valid=%v, want %v", got, test.valid)
			}
		})
	}
}

func TestMenuItemRequiresCategory(t *testing.T) {
	input := normalizeMenuItem(MenuItemInput{Name: "Dish", PriceMinor: 100, Currency: "eur"})
	if validMenuItem(input) {
		t.Fatal("menu item without category must be rejected")
	}
	if input.Currency != "EUR" {
		t.Fatalf("currency was not normalized: %q", input.Currency)
	}
}

func TestCatalogOrderRejectsEmptyAndDuplicateIDs(t *testing.T) {
	if validOrder(nil) || validOrder([]string{"one", "one"}) || validOrder([]string{"one", " "}) {
		t.Fatal("invalid catalog order must be rejected")
	}
	if !validOrder([]string{"one", "two"}) {
		t.Fatal("unique catalog order must be accepted")
	}
}
