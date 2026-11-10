package availability

import (
	"context"
	"pocket-mvp-backend/internal/validate"
	"time"

	"pocket-mvp-backend/internal/appfault"
)

const (
	duration    = 2 * time.Hour
	slotStep    = 30 * time.Minute
	bookingDays = 30
)

type Table struct {
	ID         string     `json:"id"`
	Identifier string     `json:"identifier"`
	Seats      int        `json:"seats"`
	Status     string     `json:"-"`
	Bookings   []Interval `json:"-"`
}

type Interval struct{ Start, End time.Time }

type Venue struct {
	DurationMinutes int
	Timezone        string
	ScheduleEnabled bool
	SameDayBooking  bool
}

type Slot struct {
	Time     string    `json:"time"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Tables   []Table   `json:"tables"`
}

type Result struct {
	VenueID         string `json:"venue_id"`
	Date            string `json:"date"`
	Timezone        string `json:"timezone"`
	Guests          int    `json:"guests"`
	DurationMinutes int    `json:"duration_minutes"`
	Slots           []Slot `json:"slots"`
}

type Repository interface {
	GetVenue(context.Context, string) (Venue, error)
	ListTables(context.Context, string, time.Time, time.Time) ([]Table, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (s *Service) Get(ctx context.Context, venueID, date string, guests int) (Result, error) {
	if !validate.UUID(venueID) || guests < 1 || guests > 20 || (date != "" && !validate.Date(date)) {
		return Result{}, appfault.ErrInvalidInput
	}
	venue, err := s.repository.GetVenue(ctx, venueID)
	if err != nil {
		return Result{}, err
	}
	bookingDuration := duration
	if venue.DurationMinutes > 0 {
		bookingDuration = time.Duration(venue.DurationMinutes) * time.Minute
	}
	location, err := time.LoadLocation(venue.Timezone)
	if err != nil {
		return Result{}, err
	}
	now := s.now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	if date == "" {
		date = today.Format("2006-01-02")
	}
	selected, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil || selected.Format("2006-01-02") != date {
		return Result{}, appfault.ErrInvalidInput
	}
	if selected.Before(today) || selected.After(today.AddDate(0, 0, bookingDays)) {
		return Result{}, appfault.ErrInvalidInput
	}
	result := Result{VenueID: venueID, Date: date, Timezone: venue.Timezone, Guests: guests, DurationMinutes: int(bookingDuration.Minutes()), Slots: []Slot{}}
	if !venue.ScheduleEnabled || (selected.Equal(today) && !venue.SameDayBooking) {
		return result, nil
	}
	// The configured MVP booking window is 09:00–22:00 in the venue's timezone.
	open := time.Date(selected.Year(), selected.Month(), selected.Day(), 9, 0, 0, 0, location)
	close := time.Date(selected.Year(), selected.Month(), selected.Day(), 22, 0, 0, 0, location)
	tables, err := s.repository.ListTables(ctx, venueID, open, close)
	if err != nil {
		return Result{}, err
	}
	for start := open; !start.Add(bookingDuration).After(close); start = start.Add(slotStep) {
		end := start.Add(bookingDuration)
		if !start.After(now) {
			continue
		}
		// Skip invalid or repeated local clock times at DST transitions.
		if start.In(location).YearDay() != selected.YearDay() || start.In(location).Hour() < 9 {
			continue
		}
		slot := Slot{Time: start.In(location).Format("15:04"), StartsAt: start, EndsAt: end, Tables: []Table{}}
		for _, table := range tables {
			if table.Seats < guests || table.Status == "disabled" ||
				(selected.Equal(today) && start.Before(now.Add(bookingDuration)) && table.Status != "available") {
				continue
			}
			busy := false
			for _, booking := range table.Bookings {
				if start.Before(booking.End) && booking.Start.Before(end) {
					busy = true
					break
				}
			}
			if !busy {
				table.Bookings = nil
				slot.Tables = append(slot.Tables, table)
			}
		}
		if len(slot.Tables) > 0 {
			result.Slots = append(result.Slots, slot)
		}
	}
	return result, nil
}
