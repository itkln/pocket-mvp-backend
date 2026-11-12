package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/availability"
	"pocket-mvp-backend/internal/modules/floorplan"
	"pocket-mvp-backend/internal/modules/reporting"
)

// V-10: the plan saved in Vendor is the venue's tables. Spots offers them by the same ID, removing
// a table that guests still need is refused, and the dashboard counts the real tables.
func TestFloorPlanSyncIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	owner := seedUser(t, db, "owner@floorsync.test", "customer")
	venue := seedVenue(t, db, owner, venueSeed{Slug: "floorsync", Settings: `{"schedule_enabled":true,"same_day_booking":true,"public_category":"restaurants"}`})
	authorizer := access.NewVenueAuthorizer(db)
	plans := floorplan.NewService(floorplan.NewPostgresRepository(db), authorizer)
	spots := availability.NewService(availability.NewPostgresRepository(db))
	dashboard := reporting.NewService(reporting.NewPostgresRepository(db), authorizer)

	type table struct {
		ID      string `json:"id"`
		TableID string `json:"table_id"`
		Seats   int    `json:"seats"`
	}
	save := func(plan string) (map[string]table, error) {
		t.Helper()
		saved, err := plans.Save(ctx, owner, venue, json.RawMessage(plan))
		if err != nil {
			return nil, err
		}
		var floors []struct{ Tables []table }
		must(t, json.Unmarshal(saved, &floors))
		tables := map[string]table{}
		for _, floor := range floors {
			for _, item := range floor.Tables {
				tables[item.ID] = item
			}
		}
		return tables, nil
	}
	tables, err := save(`[
		{"id":"f1","name":"Hall","tables":[{"id":"01","seats":2,"x":10,"y":10},{"id":"02","seats":4,"x":40,"y":10,"shape":"round"}],"fixtures":[]},
		{"id":"f2","name":"Terrace","tables":[{"id":"T1","seats":6,"x":20,"y":50}],"fixtures":[]}]`)
	must(t, err)
	if len(tables) != 3 || tables["01"].TableID == "" || tables["T1"].TableID == "" {
		t.Fatalf("saved plan without table IDs: %+v", tables)
	}
	stored, err := plans.Get(ctx, owner, venue)
	must(t, err)
	var reread []struct{ Tables []table }
	must(t, json.Unmarshal(stored, &reread))
	if reread[0].Tables[0].TableID != tables["01"].TableID {
		t.Fatalf("Get returned another ID: %s", stored)
	}

	// Spots offers every table of every floor by the ID the editor shows.
	zone, err := time.LoadLocation("Europe/Bratislava")
	must(t, err)
	date := time.Now().In(zone).AddDate(0, 0, 1).Format("2006-01-02")
	result, err := spots.Get(ctx, venue, date, 2)
	must(t, err)
	offered := map[string]bool{}
	for _, slot := range result.Slots {
		for _, item := range slot.Tables {
			offered[item.ID] = true
		}
	}
	for _, id := range []string{"01", "02", "T1"} {
		if !offered[tables[id].TableID] {
			t.Fatalf("table %s (%s) is not offered in Spots: %+v", id, tables[id].TableID, offered)
		}
	}

	// Seats and floor changes keep the row; a table_id sent back by the editor is ignored.
	tables, err = save(`[
		{"id":"f1","name":"Hall","tables":[{"id":"01","table_id":"forged","seats":3,"x":10,"y":10},{"id":"02","seats":4,"x":40,"y":10}],"fixtures":[]},
		{"id":"f2","name":"Terrace","tables":[{"id":"T1","seats":6,"x":20,"y":50}],"fixtures":[]}]`)
	must(t, err)
	var seats int
	must(t, db.QueryRow(ctx, `SELECT seats FROM venue_tables WHERE id = $1`, tables["01"].TableID).Scan(&seats))
	if seats != 3 || tables["01"].TableID == "forged" {
		t.Fatalf("table 01 after edit: seats %d, %+v", seats, tables["01"])
	}
	var storedForged bool
	must(t, db.QueryRow(ctx, `SELECT settings::text LIKE '%forged%' FROM venues WHERE id = $1`, venue).Scan(&storedForged))
	if storedForged {
		t.Fatal("the plan stored a client-sent table_id")
	}

	// A table with an upcoming booking cannot be removed; nothing of that save is applied.
	booking := seedBooking(t, db, venue, tables["02"].TableID, "Anna", "1 day")
	if _, err = save(`[{"id":"f1","name":"Hall","tables":[{"id":"01","seats":2,"x":10,"y":10}],"fixtures":[]}]`); !errors.Is(err, floorplan.ErrTableInUse) {
		t.Fatalf("removing a booked table: %v", err)
	}
	var live int
	must(t, db.QueryRow(ctx, `SELECT count(*) FROM venue_tables WHERE venue_id = $1 AND deleted_at IS NULL`, venue).Scan(&live))
	if live != 3 {
		t.Fatalf("a refused save changed the tables: %d live", live)
	}

	// Once the booking is cancelled the table goes, and so does the empty floor.
	_, err = db.Exec(ctx, `UPDATE reservations SET status = 'cancelled' WHERE id = $1`, booking)
	must(t, err)
	_, err = save(`[{"id":"f1","name":"Hall","tables":[{"id":"01","seats":2,"x":10,"y":10}],"fixtures":[]}]`)
	must(t, err)
	stats, err := dashboard.Dashboard(ctx, owner, venue)
	must(t, err)
	if stats.TotalTables != 1 || stats.ActiveTables != 0 {
		t.Fatalf("dashboard after removal: %+v", stats)
	}
	var terraceActive bool
	must(t, db.QueryRow(ctx, `SELECT is_active FROM venue_layouts WHERE venue_id = $1 AND name = 'Terrace'`, venue).Scan(&terraceActive))
	if terraceActive {
		t.Fatal("a floor removed from the plan stays active")
	}

	// An open bill also keeps a table; seated guests count as an occupied table.
	_, err = db.Exec(ctx, `INSERT INTO bills (venue_id, venue_table_id) VALUES ($1, $2)`, venue, tables["01"].TableID)
	must(t, err)
	if _, err = save(`[{"id":"f1","name":"Hall","tables":[],"fixtures":[]}]`); !errors.Is(err, floorplan.ErrTableInUse) {
		t.Fatalf("removing a table with an open bill: %v", err)
	}
	stats, err = dashboard.Dashboard(ctx, owner, venue)
	must(t, err)
	if stats.ActiveTables != 1 {
		t.Fatalf("table with an open bill is not occupied: %+v", stats)
	}

	// A table that comes back keeps its ID, so old bookings and QR codes still point at it.
	restored, err := save(`[{"id":"f1","name":"Hall","tables":[{"id":"01","seats":2,"x":10,"y":10},{"id":"02","seats":4,"x":40,"y":10}],"fixtures":[]}]`)
	must(t, err)
	if restored["02"].TableID != tables["02"].TableID {
		t.Fatalf("restored table got a new ID: %s, was %s", restored["02"].TableID, tables["02"].TableID)
	}

	// Plans the tables cannot follow are refused.
	for _, invalid := range []string{
		`{}`, `null`,
		`[{"name":"Hall","tables":[{"id":"01","seats":2},{"id":"01","seats":2}]}]`,
		`[{"name":"Hall","tables":[{"id":"01","seats":0}]}]`,
		`[{"name":"Hall","tables":[{"id":" ","seats":2}]}]`,
	} {
		if _, err = save(invalid); !errors.Is(err, appfault.ErrInvalidInput) {
			t.Fatalf("invalid plan %s: %v", invalid, err)
		}
	}
}
