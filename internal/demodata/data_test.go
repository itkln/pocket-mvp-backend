package demodata

import "testing"

func TestDemoWorkspaceShape(t *testing.T) {
	t.Parallel()

	if got := len(demoVenues()); got != 2 {
		t.Fatalf("demo venues = %d, want 2", got)
	}

	assertVenueShape(t, mainVenueID, venueShape{
		categories: 5,
		items:      11,
		staff:      5,
		layouts:    3,
		tables:     11,
		orders:     9,
		payments:   5,
		reviews:    4,
	})
	assertVenueShape(t, secondaryVenueID, venueShape{
		categories: 3,
		items:      4,
		staff:      2,
		layouts:    1,
		tables:     3,
		orders:     3,
		payments:   2,
		reviews:    1,
	})
}

func TestDemoCatalogReferencesExistingCategories(t *testing.T) {
	t.Parallel()

	for _, venueID := range []string{mainVenueID, secondaryVenueID} {
		categoryIDs := make(map[string]struct{})
		for _, category := range catalogCategories(venueID) {
			categoryIDs[category.ID] = struct{}{}
		}

		for _, item := range catalogItems(venueID) {
			if _, ok := categoryIDs[item.CategoryID]; !ok {
				t.Errorf("venue %s item %s references missing category %s", venueID, item.Name, item.CategoryID)
			}
			if item.ImageURL == "" {
				t.Errorf("venue %s item %s has no demo image", venueID, item.Name)
			}
		}
	}
}

type venueShape struct {
	categories int
	items      int
	staff      int
	layouts    int
	tables     int
	orders     int
	payments   int
	reviews    int
}

func assertVenueShape(t *testing.T, venueID string, want venueShape) {
	t.Helper()

	got := venueShape{
		categories: len(catalogCategories(venueID)),
		items:      len(catalogItems(venueID)),
		staff:      len(workforceMembers(venueID)),
		layouts:    len(floorLayouts(venueID)),
		tables:     len(floorTables(venueID)),
		orders:     len(demoOrders(venueID)),
		payments:   len(demoPayments(venueID)),
		reviews:    len(demoReviews(venueID)),
	}
	if got != want {
		t.Fatalf("venue %s shape = %+v, want %+v", venueID, got, want)
	}
}
