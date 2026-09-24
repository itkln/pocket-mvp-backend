package venues

import (
	"context"
	"testing"
)

type repositorySpy struct {
	query     PublicVenueQuery
	published bool
	menu      []PublicMenuCategory
}

func (r *repositorySpy) List(context.Context, string) ([]Venue, error) { return nil, nil }
func (r *repositorySpy) SearchPublished(_ context.Context, query PublicVenueQuery) ([]PublicVenue, int, error) {
	r.query = query
	return []PublicVenue{{ID: "venue-1"}}, 25, nil
}
func (r *repositorySpy) GetPublished(context.Context, string) (PublicVenue, error) {
	return PublicVenue{}, nil
}
func (r *repositorySpy) IsPublished(context.Context, string) (bool, error) { return r.published, nil }
func (r *repositorySpy) ListPublishedMenu(context.Context, string) ([]PublicMenuCategory, error) {
	return r.menu, nil
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

func TestSearchPublishedRejectsInvalidAndOversizedIDBatches(t *testing.T) {
	repository := &repositorySpy{}
	service := NewService(repository)
	if _, err := service.SearchPublished(context.Background(), PublicVenueQuery{IDs: []string{"not-a-uuid"}}); err == nil {
		t.Fatal("invalid ID was accepted")
	}
	ids := make([]string, 51)
	for i := range ids {
		ids[i] = "11111111-1111-4111-8111-111111111111"
	}
	if _, err := service.SearchPublished(context.Background(), PublicVenueQuery{IDs: ids}); err == nil {
		t.Fatal("oversized ID batch was accepted")
	}
}

func TestListPublishedMenuExcludesUnpublishedVenueAndNormalizesEmptyList(t *testing.T) {
	repository := &repositorySpy{}
	service := NewService(repository)
	id := "11111111-1111-4111-8111-111111111111"
	if _, err := service.ListPublishedMenu(context.Background(), "invalid"); err == nil {
		t.Fatal("invalid ID was accepted")
	}
	if _, err := service.ListPublishedMenu(context.Background(), id); err == nil {
		t.Fatal("unpublished venue was accepted")
	}
	repository.published = true
	menu, err := service.ListPublishedMenu(context.Background(), id)
	if err != nil || menu == nil || len(menu) != 0 {
		t.Fatalf("unexpected empty menu: %#v, %v", menu, err)
	}
}
