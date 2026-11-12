package availability

import (
	"context"
	"errors"
	"testing"
	"time"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/openinghours"
)

const testVenueID = "11111111-1111-4111-8111-111111111111"

type repositoryStub struct {
	venue  Venue
	tables []Table
}

func (r repositoryStub) GetVenue(context.Context, string) (Venue, error) { return r.venue, nil }
func (r repositoryStub) ListTables(context.Context, string, time.Time, time.Time) ([]Table, error) {
	return r.tables, nil
}

func TestAvailabilityFiltersCapacityStateAndOverlaps(t *testing.T) {
	location, err := time.LoadLocation("Europe/Bratislava")
	if err != nil {
		t.Fatal(err)
	}
	booked := time.Date(2026, 10, 3, 10, 0, 0, 0, location)
	service := NewService(repositoryStub{
		venue: Venue{Timezone: "Europe/Bratislava", ScheduleEnabled: true, SameDayBooking: true},
		tables: []Table{
			{ID: "small", Seats: 2, Status: "available"},
			{ID: "disabled", Seats: 6, Status: "disabled"},
			{ID: "occupied-now", Seats: 4, Status: "occupied"},
			{ID: "booked", Seats: 4, Status: "available", Bookings: []Interval{{Start: booked, End: booked.Add(2 * time.Hour)}}},
			{ID: "free", Seats: 4, Status: "available"},
		},
	})
	service.now = func() time.Time { return time.Date(2026, 10, 2, 12, 0, 0, 0, location) }
	result, err := service.Get(context.Background(), testVenueID, "2026-10-03", 4)
	if err != nil {
		t.Fatal(err)
	}
	if result.Timezone != "Europe/Bratislava" || result.DurationMinutes != 120 || len(result.Slots) == 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	for _, slot := range result.Slots {
		if slot.Time == "12:00" && (len(slot.Tables) != 3 || slot.Tables[0].ID != "occupied-now") {
			t.Fatalf("12:00 should start exactly when booking ends: %+v", slot)
		}
		if slot.Time == "10:00" && (len(slot.Tables) != 2 || slot.Tables[0].ID != "occupied-now") {
			t.Fatalf("overlap and capacity were not filtered: %+v", slot)
		}
	}
}

func TestAvailabilityRejectsPastAndInvalidDates(t *testing.T) {
	service := NewService(repositoryStub{venue: Venue{Timezone: "Europe/Bratislava", ScheduleEnabled: true}})
	service.now = func() time.Time { return time.Date(2026, 9, 27, 23, 30, 0, 0, time.UTC) }
	for _, date := range []string{"2026-09-27", "2026-02-30", "2026-11-01"} {
		_, err := service.Get(context.Background(), testVenueID, date, 2)
		if !errors.Is(err, appfault.ErrInvalidInput) {
			t.Errorf("date %s: got %v", date, err)
		}
	}
	result, err := service.Get(context.Background(), testVenueID, "", 2)
	if err != nil || result.Date != "2026-09-28" {
		t.Fatalf("venue-local today: %+v, %v", result, err)
	}
}

func TestAvailabilityRespectsSameDaySetting(t *testing.T) {
	service := NewService(repositoryStub{venue: Venue{Timezone: "UTC", ScheduleEnabled: true}, tables: []Table{{ID: "one", Seats: 4, Status: "available"}}})
	service.now = func() time.Time { return time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC) }
	result, err := service.Get(context.Background(), testVenueID, "", 2)
	if err != nil || len(result.Slots) != 0 {
		t.Fatalf("same day should be closed: %+v, %v", result, err)
	}
}

// Slots follow the venue's opening hours: none on a closed day, and on an open one they start at
// opening time and end by closing time.
func TestAvailabilityFollowsOpeningHours(t *testing.T) {
	hours := openinghours.Default()
	hours["sat"] = &openinghours.Day{Open: "12:00", Close: "16:00"}
	hours["sun"] = nil
	service := NewService(repositoryStub{
		venue:  Venue{Timezone: "UTC", ScheduleEnabled: true, Hours: hours},
		tables: []Table{{ID: "one", Seats: 4, Status: "available"}},
	})
	service.now = func() time.Time { return time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC) }
	result, err := service.Get(context.Background(), testVenueID, "2026-10-03", 2) // Saturday
	if err != nil {
		t.Fatal(err)
	}
	var times []string
	for _, slot := range result.Slots {
		times = append(times, slot.Time)
	}
	if len(times) != 5 || times[0] != "12:00" || times[4] != "14:00" {
		t.Fatalf("saturday 12:00–16:00 slots: %v", times)
	}
	if result, err = service.Get(context.Background(), testVenueID, "2026-10-04", 2); err != nil || len(result.Slots) != 0 {
		t.Fatalf("closed sunday: %+v, %v", result.Slots, err)
	}
}
