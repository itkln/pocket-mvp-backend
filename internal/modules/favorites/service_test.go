package favorites

import (
	"context"
	"errors"
	"testing"

	"pocket-mvp-backend/internal/appfault"
)

const (
	firstUser  = "11111111-1111-4111-8111-111111111111"
	secondUser = "22222222-2222-4222-8222-222222222222"
	venueID    = "33333333-3333-4333-8333-333333333333"
)

type memoryRepository struct{ byUser map[string]map[string]bool }

func (r *memoryRepository) ListIDs(_ context.Context, userID string) ([]string, error) {
	ids := []string{}
	for id := range r.byUser[userID] {
		ids = append(ids, id)
	}
	return ids, nil
}
func (r *memoryRepository) Add(_ context.Context, userID, id string) error {
	if r.byUser[userID] == nil {
		r.byUser[userID] = map[string]bool{}
	}
	r.byUser[userID][id] = true
	return nil
}
func (r *memoryRepository) Remove(_ context.Context, userID, id string) error {
	delete(r.byUser[userID], id)
	return nil
}

type publishedStub bool

func (v publishedStub) IsPublished(context.Context, string) (bool, error) { return bool(v), nil }

func TestFavoritesAreIdempotentAndIsolatedByUser(t *testing.T) {
	ctx := context.Background()
	s := NewService(&memoryRepository{byUser: map[string]map[string]bool{}}, publishedStub(true))
	for i := 0; i < 2; i++ {
		if err := s.Add(ctx, firstUser, venueID); err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.List(ctx, firstUser)
	if err != nil || len(first) != 1 || first[0] != venueID {
		t.Fatalf("first user: %v, %v", first, err)
	}
	second, err := s.List(ctx, secondUser)
	if err != nil || len(second) != 0 {
		t.Fatalf("second user: %v, %v", second, err)
	}
	for i := 0; i < 2; i++ {
		if err := s.Remove(ctx, firstUser, venueID); err != nil {
			t.Fatal(err)
		}
	}
	first, err = s.List(ctx, firstUser)
	if err != nil || len(first) != 0 {
		t.Fatalf("remove was not idempotent: %v, %v", first, err)
	}
}

func TestFavoritesRejectInvalidAndUnpublishedVenues(t *testing.T) {
	ctx := context.Background()
	s := NewService(&memoryRepository{byUser: map[string]map[string]bool{}}, publishedStub(false))
	if err := s.Add(ctx, firstUser, "not-an-id"); !errors.Is(err, appfault.ErrInvalidInput) {
		t.Fatalf("invalid ID: %v", err)
	}
	if err := s.Add(ctx, firstUser, venueID); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("unpublished venue: %v", err)
	}
	if err := s.Remove(ctx, firstUser, "not-an-id"); !errors.Is(err, appfault.ErrInvalidInput) {
		t.Fatalf("invalid remove: %v", err)
	}
}
