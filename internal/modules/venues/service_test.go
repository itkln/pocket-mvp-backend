package venues

import (
	"context"
	"testing"
)

type repositorySpy struct {
	published bool
	menu      []PublicMenuCategory
	reviews   []PublicReview
}

func (r *repositorySpy) List(context.Context, string) ([]Venue, error) { return nil, nil }
func (r *repositorySpy) GetPublished(context.Context, string) (PublicVenue, error) {
	return PublicVenue{}, nil
}
func (r *repositorySpy) IsPublished(context.Context, string) (bool, error) { return r.published, nil }
func (r *repositorySpy) ListPublishedMenu(context.Context, string) ([]PublicMenuCategory, error) {
	return r.menu, nil
}
func (r *repositorySpy) ListPublishedReviews(context.Context, string) ([]PublicReview, error) {
	return r.reviews, nil
}
func (r *repositorySpy) Create(context.Context, string, string, Input) (Venue, error) {
	return Venue{}, nil
}
func (r *repositorySpy) Get(context.Context, string) (Venue, error) { return Venue{}, nil }
func (r *repositorySpy) Update(context.Context, string, Input) (Venue, error) {
	return Venue{}, nil
}
func (r *repositorySpy) Delete(context.Context, string) error { return nil }

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

func TestListPublishedMenuExcludesUnpublishedVenueAndNormalizesEmptyList(t *testing.T) {
	repository := &repositorySpy{}
	service := NewService(repository, nil)
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

func TestListPublishedReviewsExcludesUnpublishedVenueAndNormalizesEmptyList(t *testing.T) {
	repository := &repositorySpy{}
	service := NewService(repository, nil)
	id := "11111111-1111-4111-8111-111111111111"
	if _, err := service.ListPublishedReviews(context.Background(), "invalid"); err == nil {
		t.Fatal("invalid ID was accepted")
	}
	if _, err := service.ListPublishedReviews(context.Background(), id); err == nil {
		t.Fatal("unpublished venue was accepted")
	}
	repository.published = true
	reviews, err := service.ListPublishedReviews(context.Background(), id)
	if err != nil || reviews == nil || len(reviews) != 0 {
		t.Fatalf("unexpected empty reviews: %#v, %v", reviews, err)
	}
}

func TestGeneratedSlugNeverEndsInSeparator(t *testing.T) {
	for i := 0; i < 200; i++ {
		slug, err := uniqueSlug("First Bistro")
		if err != nil {
			t.Fatal(err)
		}
		if slug[len(slug)-1] == '-' {
			t.Fatalf("invalid generated slug: %s", slug)
		}
	}
}

func (r *repositorySpy) ListWorkplaces(context.Context, string) ([]Venue, error) { return nil, nil }

// Opening hours and the timezone must be usable by bookings; the legacy preorder switch is renamed.
func TestValidInputChecksHoursTimezoneAndPreorderKey(t *testing.T) {
	week := map[string]any{}
	for _, day := range []string{"mon", "tue", "wed", "thu", "fri", "sat"} {
		week[day] = map[string]any{"open": "10:00", "close": "22:00"}
	}
	week["sun"] = nil
	input := normalizeInput(Input{Name: "Mokka", City: "Bratislava", Settings: map[string]any{
		"opening_hours": week, "preorder": false,
	}})
	if !validInput(input) {
		t.Fatal("a full week should be valid")
	}
	if _, legacy := input.Settings["preorder"]; legacy || input.Settings["preorder_enabled"] != false {
		t.Fatalf("legacy preorder key kept: %#v", input.Settings)
	}
	week["mon"] = map[string]any{"open": "22:00", "close": "10:00"}
	if validInput(input) {
		t.Fatal("closing before opening was accepted")
	}
	week["mon"] = map[string]any{"open": "10:00", "close": "22:00"}
	input.Timezone = "Mars/Olympus"
	if validInput(input) {
		t.Fatal("unknown timezone was accepted")
	}
}
