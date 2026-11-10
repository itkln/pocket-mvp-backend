// Package appointments composes service appointments with shared availability and
// reservation lifecycle models. Persistence is isolated in the PostgreSQL adapter.
package appointments

import (
	"context"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/availability"
	"pocket-mvp-backend/internal/modules/reservations"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var uuid = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Item struct {
	Category    string `json:"category_name"`
	ResourceID  string `json:"-"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Duration    int    `json:"duration_minutes"`
	Price       int    `json:"price_minor"`
	Currency    string `json:"currency"`
}
type Input struct {
	ServiceID     string    `json:"service_id"`
	StartsAt      time.Time `json:"starts_at"`
	Name          string    `json:"guest_name"`
	Phone         string    `json:"guest_phone"`
	Notes         string    `json:"notes"`
	Key           string    `json:"idempotency_key"`
	ExpectedPrice int       `json:"expected_price_minor"`
	CustomerID    string    `json:"-"`
}

func normalize(input Input) (Input, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Notes = strings.TrimSpace(input.Notes)
	if !uuid.MatchString(input.ServiceID) || input.StartsAt.IsZero() || utf8.RuneCountInString(input.Name) < 2 || utf8.RuneCountInString(input.Name) > 120 || len(input.Phone) < 5 || len(input.Phone) > 40 || utf8.RuneCountInString(input.Notes) > 500 || len(input.Key) < 16 || len(input.Key) > 128 || input.ExpectedPrice < 0 {
		return input, appfault.ErrInvalidInput
	}
	return input, nil
}

type Repository interface {
	List(context.Context, string) ([]Item, error)
	Availability(context.Context, string, string, string) (availability.Result, error)
	Create(context.Context, string, Input) (reservations.Result, error)
}
type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository} }
func (s *Service) List(ctx context.Context, id string) ([]Item, error) {
	return s.repository.List(ctx, id)
}
func (s *Service) Availability(ctx context.Context, id, serviceID, date string) (availability.Result, error) {
	return s.repository.Availability(ctx, id, serviceID, date)
}
func (s *Service) Create(ctx context.Context, id string, input Input) (reservations.Result, error) {
	input, err := normalize(input)
	if err != nil || !uuid.MatchString(id) {
		return reservations.Result{}, appfault.ErrInvalidInput
	}
	return s.repository.Create(ctx, id, input)
}
