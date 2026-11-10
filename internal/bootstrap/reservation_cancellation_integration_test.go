package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/availability"
	"pocket-mvp-backend/internal/modules/ordering"
	"pocket-mvp-backend/internal/modules/reservations"
)

func TestReservationCancellationIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	var customerID, otherID, venueID, layoutID, categoryID, itemID string
	must := func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(t, db.QueryRow(ctx, `INSERT INTO users (email, password_hash, first_name, last_name, account_role)
		VALUES ('cancel@example.test', 'test', 'Cancel', 'Guest', 'venue_owner') RETURNING id::text`).Scan(&customerID))
	must(t, db.QueryRow(ctx, `INSERT INTO users (email, password_hash, first_name, last_name, account_role)
		VALUES ('other-cancel@example.test', 'test', 'Other', 'Guest', 'customer') RETURNING id::text`).Scan(&otherID))
	must(t, db.QueryRow(ctx, `INSERT INTO venues (owner_user_id, slug, name, address_line1, city, country_code, timezone, currency, status, settings)
		VALUES ($1, 'cancel-test', 'Cancel Test', 'Main 1', 'Bratislava', 'SK', 'Europe/Bratislava', 'EUR', 'active',
		'{"schedule_enabled":true,"same_day_booking":true,"preorder_enabled":true,"public_category":"restaurants"}') RETURNING id::text`, customerID).Scan(&venueID))
	must(t, db.QueryRow(ctx, `INSERT INTO venue_layouts (venue_id, name) VALUES ($1, 'Main') RETURNING id::text`, venueID).Scan(&layoutID))
	_, err := db.Exec(ctx, `INSERT INTO venue_tables (venue_id, layout_id, identifier, seats, qr_token_hash) VALUES ($1, $2, 'A1', 4, 'cancel-test-a1')`, venueID, layoutID)
	must(t, err)
	must(t, db.QueryRow(ctx, `INSERT INTO menu_categories (venue_id, name) VALUES ($1, 'Food') RETURNING id::text`, venueID).Scan(&categoryID))
	must(t, db.QueryRow(ctx, `INSERT INTO menu_items (venue_id, category_id, name, price_minor, currency) VALUES ($1, $2, 'Pasta', 1200, 'EUR') RETURNING id::text`, venueID, categoryID).Scan(&itemID))
	availabilityRepository := availability.NewPostgresRepository(db)
	availabilityService := availability.NewService(availabilityRepository)
	service := reservations.NewService(reservations.NewPostgresRepository(db), reservationAvailability{availabilityService, availabilityRepository})
	vendor := ordering.NewPostgresRepository(db)
	zone, _ := time.LoadLocation("Europe/Bratislava")
	sequence := 0
	book := func(t *testing.T, preorder bool) (reservations.Result, reservations.CreateInput, string) {
		t.Helper()
		sequence++
		day := time.Now().In(zone).AddDate(0, 0, 2+sequence)
		input := reservations.CreateInput{StartsAt: time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, zone), Guests: 2,
			CustomerUserID: customerID, GuestName: "Another Person", GuestPhone: "+421900123456", Notes: "Window seat", IdempotencyKey: fmt.Sprintf("cancel-integration-%03d", sequence)}
		if preorder {
			input.Items = []reservations.PreorderItem{{MenuItemID: itemID, Quantity: 2, ExpectedPriceMinor: 1200}}
		}
		created, err := service.Create(ctx, venueID, input)
		must(t, err)
		var orderID string
		if preorder {
			must(t, db.QueryRow(ctx, `SELECT id::text FROM orders WHERE reservation_id = $1`, created.ID).Scan(&orderID))
		}
		return created, input, orderID
	}
	assertStatus := func(t *testing.T, id, reservationStatus, orderStatus string) {
		t.Helper()
		var status string
		must(t, db.QueryRow(ctx, `SELECT status FROM reservations WHERE id = $1`, id).Scan(&status))
		if status != reservationStatus {
			t.Fatalf("reservation status = %s, want %s", status, reservationStatus)
		}
		if orderStatus != "" {
			must(t, db.QueryRow(ctx, `SELECT status FROM orders WHERE reservation_id = $1`, id).Scan(&status))
			if status != orderStatus {
				t.Fatalf("order status = %s, want %s", status, orderStatus)
			}
		}
	}
	assertDenied := func(t *testing.T, id, reason string) {
		t.Helper()
		_, err := service.CancelCustomer(ctx, customerID, id)
		var denied *reservations.CancellationDenied
		if !errors.As(err, &denied) || denied.Reason != reason {
			t.Fatalf("expected %s, got %v", reason, err)
		}
		list, err := service.ListCustomer(ctx, customerID)
		must(t, err)
		for _, entry := range list {
			if entry.ID == id && (entry.Cancellation.Allowed || entry.Cancellation.Reason != reason) {
				t.Fatalf("list eligibility differs: %+v", entry.Cancellation)
			}
		}
	}

	t.Run("ownership and invalid IDs", func(t *testing.T) {
		created, _, _ := book(t, true)
		for _, id := range []string{created.ID, "10000000-0000-4000-8000-000000000099"} {
			if _, err := service.CancelCustomer(ctx, otherID, id); !errors.Is(err, appfault.ErrNotFound) {
				t.Fatalf("foreign/missing reservation: %v", err)
			}
		}
		if _, err := service.CancelCustomer(ctx, customerID, "bad-id"); !errors.Is(err, appfault.ErrInvalidInput) {
			t.Fatalf("invalid ID: %v", err)
		}
		assertStatus(t, created.ID, "confirmed", "new")
		_, input, _ := book(t, false)
		input.StartsAt = input.StartsAt.Add(3 * time.Hour)
		input.IdempotencyKey += "-hold"
		hold, err := service.Hold(ctx, venueID, input)
		must(t, err)
		if _, err := service.CancelCustomer(ctx, customerID, hold.ID); !errors.Is(err, appfault.ErrNotFound) {
			t.Fatalf("unfinished hold cancellation: %v", err)
		}
	})
	t.Run("atomic preorder cancellation, concurrent retries, history and slot release", func(t *testing.T) {
		created, input, orderID := book(t, true)
		originalKey := input.IdempotencyKey
		_, err := vendor.UpdateStatus(ctx, venueID, orderID, "accepted")
		must(t, err)
		input.IdempotencyKey += "-replacement"
		if _, err := service.Create(ctx, venueID, input); !errors.Is(err, appfault.ErrConflict) {
			t.Fatalf("occupied slot accepted: %v", err)
		}
		var wg sync.WaitGroup
		failures := make(chan error, 4)
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				cancelled, err := service.CancelCustomer(ctx, customerID, created.ID)
				if err != nil {
					failures <- err
					return
				}
				if cancelled.Status != "cancelled" || cancelled.PreorderStatus != "cancelled" || cancelled.Cancellation.Allowed ||
					len(cancelled.PreorderItems) != 1 || cancelled.PreorderTotalMinor != 2400 || cancelled.Notes != "Window seat" {
					failures <- fmt.Errorf("bad cancellation response: %+v", cancelled)
				}
			}()
		}
		wg.Wait()
		close(failures)
		for err := range failures {
			t.Error(err)
		}
		assertStatus(t, created.ID, "cancelled", "cancelled")
		var count int
		must(t, db.QueryRow(ctx, `SELECT count(*) FROM order_items WHERE order_id = $1 AND status <> 'cancelled'`, orderID).Scan(&count))
		if count != 0 {
			t.Fatal("uncancelled preorder items")
		}
		must(t, db.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE entity_id = $1 AND action = 'reservation.cancelled_by_customer'`, created.ID).Scan(&count))
		if count != 1 {
			t.Fatalf("retry duplicated audit event: %d", count)
		}
		if _, err := vendor.UpdateStatus(ctx, venueID, orderID, "preparing"); !errors.Is(err, appfault.ErrConflict) {
			t.Fatalf("cancelled preorder resurrected: %v", err)
		}
		orders, err := vendor.List(ctx, venueID)
		must(t, err)
		for _, order := range orders {
			if order.ID == orderID && order.Status != "cancelled" {
				t.Fatal("Vendor status stale")
			}
		}
		preorders, err := vendor.ListCustomerPreorders(ctx, customerID)
		must(t, err)
		found := false
		for _, order := range preorders {
			if order.ID == orderID {
				found = true
				if order.Status != "cancelled" {
					t.Fatal("preorder history stale")
				}
			}
		}
		if !found {
			t.Fatal("cancelled preorder disappeared from history")
		}
		if _, err := service.GetConfirmed(ctx, venueID, created.ID, originalKey); !errors.Is(err, appfault.ErrNotFound) {
			t.Fatalf("cancelled reservation restored: %v", err)
		}
		if _, err := service.Create(ctx, venueID, input); err != nil {
			t.Fatalf("cancelled slot not released: %v", err)
		}
	})
	t.Run("ordinary reservation", func(t *testing.T) {
		created, _, _ := book(t, false)
		cancelled, err := service.CancelCustomer(ctx, customerID, created.ID)
		must(t, err)
		if cancelled.Status != "cancelled" || len(cancelled.PreorderItems) != 0 {
			t.Fatalf("plain cancellation: %+v", cancelled)
		}
	})
	t.Run("preparation and cancellation cannot both succeed", func(t *testing.T) {
		for i := 0; i < 4; i++ {
			created, _, orderID := book(t, true)
			_, err := vendor.UpdateStatus(ctx, venueID, orderID, "accepted")
			must(t, err)
			start := make(chan struct{})
			cancelResult, prepareResult := make(chan error, 1), make(chan error, 1)
			go func() { <-start; _, err := service.CancelCustomer(ctx, customerID, created.ID); cancelResult <- err }()
			go func() {
				<-start
				_, err := vendor.UpdateStatus(ctx, venueID, orderID, "preparing")
				prepareResult <- err
			}()
			close(start)
			cancelErr, prepareErr := <-cancelResult, <-prepareResult
			if cancelErr == nil {
				if !errors.Is(prepareErr, appfault.ErrConflict) {
					t.Fatalf("preparation won after cancellation: %v", prepareErr)
				}
				assertStatus(t, created.ID, "cancelled", "cancelled")
			} else {
				var denied *reservations.CancellationDenied
				if prepareErr != nil || !errors.As(cancelErr, &denied) || denied.Reason != "cancellation_preorder_locked" {
					t.Fatalf("race: %v / %v", cancelErr, prepareErr)
				}
				assertStatus(t, created.ID, "confirmed", "preparing")
			}
		}
	})
	t.Run("blocked preorder and paid preorder leave all data intact", func(t *testing.T) {
		for _, status := range []string{"preparing", "ready", "served", "completed"} {
			created, _, orderID := book(t, true)
			for _, next := range []string{"accepted", "preparing", "ready", "served", "completed"} {
				_, err := vendor.UpdateStatus(ctx, venueID, orderID, next)
				must(t, err)
				if next == status {
					break
				}
			}
			assertDenied(t, created.ID, "cancellation_preorder_locked")
			assertStatus(t, created.ID, "confirmed", status)
		}
		for _, status := range []string{"pending", "processing", "paid", "refunded", "disputed"} {
			created, _, orderID := book(t, true)
			_, err := db.Exec(ctx, `INSERT INTO payments (venue_id, order_id, provider, provider_payment_id, status, amount_minor, currency)
				VALUES ($1, $2, 'test', gen_random_uuid()::text, $3, 2400, 'EUR')`, venueID, orderID, status)
			must(t, err)
			assertDenied(t, created.ID, "cancellation_payment_started")
			assertStatus(t, created.ID, "confirmed", "new")
		}
	})
	t.Run("deadline, venue policy and reservation status", func(t *testing.T) {
		created, _, _ := book(t, false)
		_, err := db.Exec(ctx, `UPDATE reservations SET starts_at = now() - interval '1 hour', ends_at = now() + interval '1 hour' WHERE id = $1`, created.ID)
		must(t, err)
		assertDenied(t, created.ID, "cancellation_deadline_passed")
		created, _, _ = book(t, false)
		_, err = db.Exec(ctx, `UPDATE venues SET settings = jsonb_set(settings, '{guest_cancellation}', '{"cutoff_minutes":43200,"preorder_until":"preparing"}') WHERE id = $1`, venueID)
		must(t, err)
		assertDenied(t, created.ID, "cancellation_deadline_passed")
		_, err = db.Exec(ctx, `UPDATE venues SET settings = settings - 'guest_cancellation' WHERE id = $1`, venueID)
		must(t, err)
		_, err = db.Exec(ctx, `UPDATE reservations SET status = 'seated' WHERE id = $1`, created.ID)
		must(t, err)
		assertDenied(t, created.ID, "cancellation_status_blocked")
	})
	t.Run("failure rolls reservation, order and items back together", func(t *testing.T) {
		created, _, _ := book(t, true)
		_, err := db.Exec(ctx, `CREATE FUNCTION reject_cancel_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
			IF NEW.action = 'reservation.cancelled_by_customer' THEN RAISE EXCEPTION 'test audit failure'; END IF; RETURN NEW; END $$;
			CREATE TRIGGER reject_cancel_audit BEFORE INSERT ON audit_logs FOR EACH ROW EXECUTE FUNCTION reject_cancel_audit()`)
		must(t, err)
		if _, err := service.CancelCustomer(ctx, customerID, created.ID); err == nil {
			t.Fatal("forced failure was ignored")
		}
		assertStatus(t, created.ID, "confirmed", "new")
		var status string
		must(t, db.QueryRow(ctx, `SELECT status FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE reservation_id = $1)`, created.ID).Scan(&status))
		if status != "new" {
			t.Fatal("partial item cancellation")
		}
		_, err = db.Exec(ctx, `DROP TRIGGER reject_cancel_audit ON audit_logs; DROP FUNCTION reject_cancel_audit()`)
		must(t, err)
		_, err = service.CancelCustomer(ctx, customerID, created.ID)
		must(t, err)
	})
}
