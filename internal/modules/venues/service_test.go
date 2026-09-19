package venues

import (
	"context"
	"testing"
)

type repositorySpy struct {
	query PublicVenueQuery
}

func (r *repositorySpy) List(context.Context, string) ([]Venue, error) { return nil, nil }
func (r *repositorySpy) SearchPublished(_ context.Context, query PublicVenueQuery) ([]PublicVenue, int, error) {
	r.query = query
	return []PublicVenue{{ID: "venue-1"}}, 25, nil
}
func (r *repositorySpy) Create(context.Context, string, string, Input) (Venue, error) {
	return Venue{}, nil
}
func (r *repositorySpy) Update(context.Context, string, string, Input) (Venue, error) {
	return Venue{}, nil
}
func (r *repositorySpy) Delete(context.Context, string, string) error { return nil }

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

func TestSearchPublishedNormalizesPaginationAndBuildsMetadata(t *testing.T) {
	repository := &repositorySpy{}
	page, err := NewService(repository).SearchPublished(context.Background(), PublicVenueQuery{City: " Bratislava ", Filter: "Все"})
	if err != nil {
		t.Fatal(err)
	}
	if repository.query.Page != 1 || repository.query.PageSize != 24 || repository.query.City != "Bratislava" || repository.query.Filter != "" {
		t.Fatalf("query was not normalized: %#v", repository.query)
	}
	if page.Total != 25 || page.TotalPages != 2 || !page.HasNext || len(page.Venues) != 1 {
		t.Fatalf("unexpected page metadata: %#v", page)
	}
}
