package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/floorplan"
	"pocket-mvp-backend/internal/modules/seating"
)

// The live floor: a booked party and a walk-in are seated, tables report who sits at them and
// what is booked next, and the staff free a table when the party leaves.
func TestSeatingIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	owner := seedUser(t, db, "owner@seating.test", "customer")
	waiter := seedUser(t, db, "waiter@seating.test", "customer")
	venue := seedVenue(t, db, owner, venueSeed{Slug: "seating"})
	other := seedVenue(t, db, owner, venueSeed{Slug: "seating-other"})
	seedStaff(t, db, venue, waiter, "waiter@seating.test", "waiter", owner)
	authorizer := access.NewVenueAuthorizer(db)
	plan := floorplan.NewService(floorplan.NewPostgresRepository(db), authorizer)
	floor := seating.NewService(seating.NewPostgresRepository(db), authorizer)
	_, err := plan.Save(ctx, owner, venue, json.RawMessage(`[{"id":"floor-1","name":"Main","tables":[
		{"id":"01","seats":2,"x":10,"y":20},{"id":"02","seats":4,"x":40,"y":20},{"id":"03","seats":4,"x":70,"y":20}],"fixtures":[]}]`))
	must(t, err)

	state := func(snapshot seating.Floor, identifier string) seating.Table {
		t.Helper()
		for _, table := range snapshot.Tables {
			if table.Identifier == identifier {
				return table
			}
		}
		t.Fatalf("table %s is not on the floor: %+v", identifier, snapshot.Tables)
		return seating.Table{}
	}

	// Saving the plan creates its tables: the fresh floor lists them all, free.
	snapshot, err := floor.Floor(ctx, owner, venue)
	must(t, err)
	if len(snapshot.Tables) != 3 || len(snapshot.Bookings) != 0 || snapshot.Timezone != "Europe/Bratislava" {
		t.Fatalf("fresh floor: %+v", snapshot)
	}
	for _, table := range snapshot.Tables {
		if table.State != seating.StateFree {
			t.Fatalf("fresh table is not free: %+v", table)
		}
	}

	// A walk-in takes a free table.
	snapshot, err = floor.SeatWalkIn(ctx, owner, venue, seating.WalkIn{TableIdentifier: "01", Guests: 2, GuestName: " Mia "})
	must(t, err)
	table := state(snapshot, "01")
	if table.State != seating.StateOccupied || table.Party == nil || !table.Party.WalkIn ||
		table.Party.GuestName != "Mia" || table.Party.GuestCount != 2 || table.Party.SeatedAt.IsZero() {
		t.Fatalf("walk-in table: %+v party %+v", table, table.Party)
	}
	if _, err = floor.SeatWalkIn(ctx, owner, venue, seating.WalkIn{TableIdentifier: "01", Guests: 2}); !errors.Is(err, seating.ErrTableOccupied) {
		t.Fatalf("second party at a taken table: %v", err)
	}
	for _, invalid := range []seating.WalkIn{{TableIdentifier: "02"}, {Guests: 2}, {TableIdentifier: "02", Guests: 21}} {
		if _, err = floor.SeatWalkIn(ctx, owner, venue, invalid); !errors.Is(err, appfault.ErrInvalidInput) {
			t.Fatalf("invalid walk-in %+v: %v", invalid, err)
		}
	}
	if _, err = floor.SeatWalkIn(ctx, owner, venue, seating.WalkIn{TableIdentifier: "99", Guests: 2}); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("walk-in at a table outside the plan: %v", err)
	}

	// A booking for later today: its table is reserved, and the booking is expected.
	_, err = floor.SeatWalkIn(ctx, owner, venue, seating.WalkIn{TableIdentifier: "02", Guests: 1})
	must(t, err)
	_, err = floor.Release(ctx, owner, venue, "02")
	must(t, err)
	var tableTwo string
	must(t, db.QueryRow(ctx, `SELECT id::text FROM venue_tables WHERE venue_id = $1 AND identifier = '02'`, venue).Scan(&tableTwo))
	booking := seedBooking(t, db, venue, tableTwo, "Anna", "30 minutes")
	snapshot, err = floor.Floor(ctx, owner, venue)
	must(t, err)
	if table = state(snapshot, "02"); table.State != seating.StateReserved || table.NextReservationID != booking {
		t.Fatalf("booked table: %+v", table)
	}
	if len(snapshot.Bookings) != 1 || snapshot.Bookings[0].ID != booking || snapshot.Bookings[0].TableIdentifier != "02" ||
		snapshot.Bookings[0].GuestName != "Anna" || snapshot.Bookings[0].GuestCount != 2 {
		t.Fatalf("expected bookings: %+v", snapshot.Bookings)
	}
	// A walk-in cannot take a table whose booking starts within half an hour.
	if _, err = floor.SeatWalkIn(ctx, owner, venue, seating.WalkIn{TableIdentifier: "02", Guests: 2}); !errors.Is(err, seating.ErrTableReserved) {
		t.Fatalf("walk-in at a table about to be taken: %v", err)
	}

	// The booked party cannot join the walk-in's table, but may move to a free one.
	if _, err = floor.SeatReservation(ctx, owner, venue, booking, "01"); !errors.Is(err, seating.ErrTableOccupied) {
		t.Fatalf("booking seated at a taken table: %v", err)
	}
	snapshot, err = floor.SeatReservation(ctx, owner, venue, booking, "03")
	must(t, err)
	if table = state(snapshot, "03"); table.State != seating.StateOccupied || table.Party == nil ||
		table.Party.ReservationID != booking || table.Party.WalkIn {
		t.Fatalf("moved booking: %+v", table)
	}
	if table = state(snapshot, "02"); table.State != seating.StateFree || len(snapshot.Bookings) != 0 {
		t.Fatalf("the booked table was not freed: %+v, bookings %+v", table, snapshot.Bookings)
	}
	// Seating the same booking again is a retry, not an error.
	_, err = floor.SeatReservation(ctx, owner, venue, booking, "03")
	must(t, err)

	// A booking seated at its own table needs no table in the request.
	own := seedBooking(t, db, venue, tableTwo, "Boris", "10 minutes")
	snapshot, err = floor.SeatReservation(ctx, owner, venue, own, "")
	must(t, err)
	if table = state(snapshot, "02"); table.Party == nil || table.Party.ReservationID != own {
		t.Fatalf("booking at its own table: %+v", table)
	}

	// An open bill keeps a table occupied even after its party was released.
	_, err = db.Exec(ctx, `INSERT INTO bills (venue_id, venue_table_id) VALUES ($1, $2)`, venue, tableTwo)
	must(t, err)
	snapshot, err = floor.Release(ctx, owner, venue, "02")
	must(t, err)
	if table = state(snapshot, "02"); table.State != seating.StateOccupied || table.Party != nil || table.Bill == nil {
		t.Fatalf("released table with an open bill: %+v", table)
	}
	snapshot, err = floor.Release(ctx, owner, venue, "01")
	must(t, err)
	if table = state(snapshot, "01"); table.State != seating.StateFree || table.Party != nil {
		t.Fatalf("released table: %+v", table)
	}
	var status string
	must(t, db.QueryRow(ctx, `SELECT status FROM reservations WHERE id = $1`, own).Scan(&status))
	if status != "completed" {
		t.Fatalf("released booking is %s", status)
	}
	// A finished booking cannot be seated again.
	if _, err = floor.SeatReservation(ctx, owner, venue, own, ""); !errors.Is(err, seating.ErrNotSeatable) {
		t.Fatalf("finished booking seated again: %v", err)
	}

	// Waiters seat guests and free tables on the live floor and read the plan, but cannot edit
	// the plan or issue QR codes (V-SRV-02). Strangers and other venues get 404.
	if _, err = floor.Floor(ctx, waiter, venue); err != nil {
		t.Fatalf("waiter read the floor: %v", err)
	}
	if _, err = plan.Get(ctx, waiter, venue); err != nil {
		t.Fatalf("waiter read the plan: %v", err)
	}
	if _, err = floor.Release(ctx, waiter, venue, "03"); err != nil {
		t.Fatalf("waiter released a table: %v", err)
	}
	walkIn, err := floor.SeatWalkIn(ctx, waiter, venue, seating.WalkIn{TableIdentifier: "03", Guests: 2})
	if err != nil || state(walkIn, "03").State != "occupied" {
		t.Fatalf("waiter seated a walk-in: %v", err)
	}
	if _, err = plan.Save(ctx, waiter, venue, json.RawMessage(`[]`)); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("waiter edited the plan: %v", err)
	}
	stranger := seedUser(t, db, "stranger@seating.test", "customer")
	if _, err = floor.Floor(ctx, stranger, venue); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("stranger read the floor: %v", err)
	}
	third := seedBooking(t, db, venue, tableTwo, "Clara", "3 hours")
	if _, err = floor.SeatReservation(ctx, owner, other, third, ""); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("booking seated through another venue: %v", err)
	}
}

// seedBooking inserts a confirmed two-guest, two-hour booking that starts after the given interval.
func seedBooking(t *testing.T, db *pgxpool.Pool, venueID, tableID, guest, startsIn string) string {
	t.Helper()
	var id string
	must(t, db.QueryRow(context.Background(), `
		INSERT INTO reservations (venue_id, venue_table_id, reservation_code, guest_name, guest_phone,
			guest_count, starts_at, ends_at, status)
		VALUES ($1, $2, upper(substr(md5(random()::text), 1, 10)), $3, '+421900000000', 2,
			now() + $4::interval, now() + $4::interval + interval '2 hours', 'confirmed')
		RETURNING id::text`, venueID, tableID, guest, startsIn).Scan(&id))
	return id
}
