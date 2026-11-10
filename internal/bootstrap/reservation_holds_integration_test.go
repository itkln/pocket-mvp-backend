package bootstrap

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/availability"
	"pocket-mvp-backend/internal/modules/reservations"
)

func TestReservationHoldPreventsDoubleBookingAndExpires(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	var layoutID string
	ownerID := seedUser(t, db, "hold-owner@example.test", "venue_owner")
	venueID := seedVenue(t, db, ownerID, venueSeed{Slug: "hold-test", Name: "Hold Test", Settings: `{"schedule_enabled":true,"same_day_booking":true,"public_category":"restaurants"}`})
	if err := db.QueryRow(ctx, `INSERT INTO venue_layouts (venue_id, name) VALUES ($1, 'Main') RETURNING id::text`, venueID).Scan(&layoutID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO venue_tables (venue_id, layout_id, identifier, seats, qr_token_hash)
		VALUES ($1, $2, 'A1', 4, 'hold-test-a1')`, venueID, layoutID); err != nil {
		t.Fatal(err)
	}

	availabilityRepository := availability.NewPostgresRepository(db)
	booking := reservations.NewService(reservations.NewPostgresRepository(db),
		reservationAvailability{availability.NewService(availabilityRepository), availabilityRepository})
	zone, _ := time.LoadLocation("Europe/Bratislava")
	selected := time.Now().In(zone).AddDate(0, 0, 2)
	start := time.Date(selected.Year(), selected.Month(), selected.Day(), 12, 0, 0, 0, zone)
	input := reservations.CreateInput{StartsAt: start, Guests: 2, GuestName: "Hold Guest", GuestPhone: "+421900123456"}

	// Both requests see the same free table; only one may acquire its hold.
	startRequests := make(chan struct{})
	results := make(chan struct {
		hold  reservations.HoldResult
		input reservations.CreateInput
		err   error
	}, 2)
	var workers sync.WaitGroup
	for _, key := range []string{"concurrent-hold-key-1", "concurrent-hold-key-2"} {
		workers.Add(1)
		go func(key string) {
			defer workers.Done()
			candidate := input
			candidate.IdempotencyKey = key
			<-startRequests
			hold, err := booking.Hold(ctx, venueID, candidate)
			results <- struct {
				hold  reservations.HoldResult
				input reservations.CreateInput
				err   error
			}{hold, candidate, err}
		}(key)
	}
	close(startRequests)
	workers.Wait()
	close(results)
	var winner reservations.HoldResult
	var winnerInput reservations.CreateInput
	var success, conflicts int
	for result := range results {
		if result.err == nil {
			success++
			winner, winnerInput = result.hold, result.input
		}
		if errors.Is(result.err, appfault.ErrConflict) {
			conflicts++
		}
		if result.err != nil && !errors.Is(result.err, appfault.ErrConflict) {
			t.Fatal(result.err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("concurrent holds: success=%d conflicts=%d", success, conflicts)
	}
	if winner.ID == "" || !winner.ExpiresAt.After(time.Now()) {
		t.Fatalf("invalid hold: %+v", winner)
	}
	changedContact := winnerInput
	changedContact.GuestName = "Different Guest"
	confirmed, err := booking.Confirm(ctx, venueID, winner.ID, changedContact)
	if err != nil || confirmed.ID != winner.ID || confirmed.Code == "" {
		t.Fatalf("confirm: %+v, %v", confirmed, err)
	}
	var savedName string
	if err := db.QueryRow(ctx, `SELECT guest_name FROM reservations WHERE id = $1`, confirmed.ID).Scan(&savedName); err != nil || savedName != changedContact.GuestName {
		t.Fatalf("updated contact was not saved: %s, %v", savedName, err)
	}
	replayed, err := booking.Confirm(ctx, venueID, winner.ID, changedContact)
	if err != nil || replayed.ID != confirmed.ID {
		t.Fatalf("idempotent confirmation: %+v, %v", replayed, err)
	}
	if _, err := booking.Confirm(ctx, venueID, winner.ID, winnerInput); !errors.Is(err, appfault.ErrConflict) {
		t.Fatalf("changed confirmed request reused the key: %v", err)
	}
	reloaded, err := booking.GetConfirmed(ctx, venueID, confirmed.ID, changedContact.IdempotencyKey)
	if err != nil || reloaded.Code != confirmed.Code {
		t.Fatalf("reload: %+v, %v", reloaded, err)
	}
	if _, err := booking.GetConfirmed(ctx, venueID, confirmed.ID, "someone-elses-key-123"); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("reservation leaked to wrong key: %v", err)
	}

	second := input
	second.StartsAt = start.Add(3 * time.Hour)
	second.IdempotencyKey = "expiring-hold-key-1"
	expiring, err := booking.Hold(ctx, venueID, second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE reservations SET hold_expires_at = now() - interval '1 second' WHERE id = $1`, expiring.ID); err != nil {
		t.Fatal(err)
	}
	listed, err := availability.NewService(availabilityRepository).Get(ctx, venueID, selected.Format("2006-01-02"), 2)
	if err != nil {
		t.Fatal(err)
	}
	available := false
	for _, slot := range listed.Slots {
		if slot.StartsAt.Equal(second.StartsAt) {
			available = true
		}
	}
	if !available {
		t.Fatal("expired hold still hides available time")
	}
	if _, err := booking.Confirm(ctx, venueID, expiring.ID, second); !errors.Is(err, reservations.ErrHoldExpired) {
		t.Fatalf("expired hold confirmed: %v", err)
	}
	second.IdempotencyKey = "replacement-hold-key-1"
	replacement, err := booking.Hold(ctx, venueID, second)
	if err != nil || replacement.ID == expiring.ID {
		t.Fatalf("replacement hold: %+v, %v", replacement, err)
	}
}
