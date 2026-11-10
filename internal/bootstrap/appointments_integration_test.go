package bootstrap

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/appointments"
	"pocket-mvp-backend/internal/modules/reservations"
)

func TestServiceAppointmentsIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	_, err := db.Exec(ctx, `INSERT INTO users(id,email,password_hash,first_name,last_name,account_role) VALUES('10000000-0000-4000-8000-000000000001','services@example.test','test','Test','Owner','venue_owner')`)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := os.ReadFile("../../scripts/seed-service-examples.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = db.Exec(ctx, string(seed)); err != nil {
			t.Fatal(err)
		}
	}
	service := appointments.New(appointments.NewPostgresRepository(db))
	// Each master has a separate resource, but services of one master conflict.
	barber := "20000000-0000-4000-8000-000000000206"
	barberItems, err := service.List(ctx, barber)
	if err != nil || len(barberItems) != 4 {
		t.Fatalf("master services: %v %v", barberItems, err)
	}
	barberDate := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	firstSlots, err := service.Availability(ctx, barber, barberItems[0].ID, barberDate)
	if err != nil || len(firstSlots.Slots) == 0 {
		t.Fatal("missing barber slots", err)
	}
	start := firstSlots.Slots[0].StartsAt
	for _, index := range []int{0, 2} {
		item := barberItems[index]
		_, err = service.Create(ctx, barber, appointments.Input{ServiceID: item.ID, StartsAt: start, Name: "Test Guest", Phone: "123456789", ExpectedPrice: item.Price, Key: "master-booking-" + item.ID})
		if err != nil {
			t.Fatalf("independent master: %v", err)
		}
	}
	sameMaster, err := service.Availability(ctx, barber, barberItems[1].ID, barberDate)
	if err != nil {
		t.Fatal(err)
	}
	for _, slot := range sameMaster.Slots {
		if slot.StartsAt.Equal(start) {
			t.Fatal("same master double booking")
		}
	}
	venueID := "20000000-0000-4000-8000-000000000201"
	items, err := service.List(ctx, venueID)
	if err != nil || len(items) != 2 {
		t.Fatalf("services: %v %v", items, err)
	}
	date := time.Now().AddDate(0, 0, 2).Format("2006-01-02")
	slots, err := service.Availability(ctx, venueID, items[0].ID, date)
	if err != nil || len(slots.Slots) == 0 {
		t.Fatalf("slots: %+v %v", slots, err)
	}
	if slots.DurationMinutes != 60 {
		t.Fatalf("duration %d", slots.DurationMinutes)
	}
	input := appointments.Input{ServiceID: items[0].ID, StartsAt: slots.Slots[0].StartsAt, Name: "Тест записи", Phone: "+421900000000", ExpectedPrice: items[0].Price, Key: "service-appointment-test-one"}
	// A stale price cannot create a reservation.
	stale := input
	stale.ExpectedPrice++
	if _, err = service.Create(ctx, venueID, stale); !errors.Is(err, reservations.ErrPriceChanged) {
		t.Fatalf("stale price: %v", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, key := range []string{input.Key, "service-appointment-test-two"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			copy := input
			copy.Key = key
			_, err := service.Create(ctx, venueID, copy)
			results <- err
		}(key)
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, appfault.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("concurrency: %d success %d conflicts", success, conflicts)
	}
	var winnerKey, id string
	if err = db.QueryRow(ctx, `SELECT id::text,idempotency_key FROM reservations WHERE venue_id=$1`, venueID).Scan(&id, &winnerKey); err != nil {
		t.Fatal(err)
	}
	input.Key = winnerKey
	replay, err := service.Create(ctx, venueID, input)
	if err != nil || replay.ID != id {
		t.Fatalf("retry: %+v %v", replay, err)
	}
	after, err := service.Availability(ctx, venueID, items[0].ID, date)
	if err != nil {
		t.Fatal(err)
	}
	for _, slot := range after.Slots {
		if slot.StartsAt.Before(input.StartsAt.Add(time.Hour)) {
			t.Fatal("overlapping slot remained available")
		}
	}
	if _, err = service.Availability(ctx, "20000000-0000-4000-8000-000000000202", items[0].ID, date); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("cross venue service: %v", err)
	}
	if _, err = db.Exec(ctx, `UPDATE reservations SET status='cancelled' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	freed, err := service.Availability(ctx, venueID, items[0].ID, date)
	if err != nil || len(freed.Slots) != len(slots.Slots) {
		t.Fatalf("cancel did not release slot: %v", err)
	}
}
