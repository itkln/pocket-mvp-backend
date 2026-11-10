package bootstrap

import (
	"context"
	"os"
	"testing"
)

// Migration 000025 removes kitchen-era data: orders in accepted/ready or on pickup are deleted
// with their contribution taken out of open bills, kitchen and viewer staff become inactive waiters.
func TestOrdersWithoutKitchenMigrationIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	apply := func(file string) {
		t.Helper()
		sql, err := os.ReadFile("../../migrations/" + file)
		must(err)
		_, err = db.Exec(ctx, string(sql))
		must(err)
	}
	apply("000025_orders_without_kitchen.down.sql")

	var owner, venue, bill string
	must(db.QueryRow(ctx, `INSERT INTO users(email,password_hash,first_name,last_name,account_role) VALUES('owner@kitchen-migration.test','x','Owner','Test','venue_owner') RETURNING id::text`).Scan(&owner))
	must(db.QueryRow(ctx, `INSERT INTO venues(owner_user_id,slug,name,address_line1,city,country_code,timezone,currency,status) VALUES($1,'kitchen-migration','Venue','Main','Bratislava','SK','Europe/Bratislava','EUR','active') RETURNING id::text`, owner).Scan(&venue))
	must(db.QueryRow(ctx, `INSERT INTO bills(venue_id,subtotal_minor,tip_minor,total_minor) VALUES($1,6000,300,6300) RETURNING id::text`, venue).Scan(&bill))
	order := func(channel, status string, subtotal, tip int) string {
		var id string
		must(db.QueryRow(ctx, `INSERT INTO orders(venue_id,bill_id,channel,status,subtotal_minor,tip_minor,total_minor) VALUES($1,$2,$3,$4,$5::int,$6::int,$7::int) RETURNING id::text`,
			venue, bill, channel, status, subtotal, tip, subtotal+tip).Scan(&id))
		itemStatus := "new"
		if status == "ready" {
			itemStatus = "ready"
		}
		_, err := db.Exec(ctx, `INSERT INTO order_items(order_id,item_name,quantity,unit_price_minor,total_minor,status) VALUES($1,'Soup',1,$2,$2,$3)`, id, subtotal, itemStatus)
		must(err)
		return id
	}
	kept := order("dine_in", "preparing", 1000, 100)
	order("dine_in", "accepted", 2000, 100)
	order("dine_in", "ready", 1500, 100)
	order("pickup", "cancelled", 1500, 0) // cancelled orders were already out of the bill
	var readyItemOrder string
	must(db.QueryRow(ctx, `INSERT INTO orders(venue_id,channel,status) VALUES($1,'dine_in','served') RETURNING id::text`, venue).Scan(&readyItemOrder))
	_, err := db.Exec(ctx, `INSERT INTO order_items(order_id,item_name,quantity,unit_price_minor,total_minor,status) VALUES($1,'Tea',1,300,300,'ready')`, readyItemOrder)
	must(err)
	removedID := order("dine_in", "accepted", 0, 0)
	_, err = db.Exec(ctx, `INSERT INTO order_status_events(order_id,actor_name,previous_status,new_status) VALUES($1,'Owner','new','accepted')`, removedID)
	must(err)
	for _, role := range []string{"kitchen", "viewer", "waiter"} {
		_, err := db.Exec(ctx, `INSERT INTO venue_staff(venue_id,invited_email,role,status,invited_by_user_id,invite_token_hash) VALUES($1,$2,$3,'active',$4,$2)`,
			venue, role+"@kitchen-migration.test", role, owner)
		must(err)
	}

	apply("000025_orders_without_kitchen.up.sql")

	var orders, keptCount int
	must(db.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE id=$2) FROM orders WHERE venue_id=$1`, venue, kept).Scan(&orders, &keptCount))
	if orders != 2 || keptCount != 1 {
		t.Fatalf("orders after migration: %d, kept %d", orders, keptCount)
	}
	var subtotal, tip, total int
	must(db.QueryRow(ctx, `SELECT subtotal_minor,tip_minor,total_minor FROM bills WHERE id=$1`, bill).Scan(&subtotal, &tip, &total))
	if subtotal != 2500 || tip != 100 || total != 2600 {
		t.Fatalf("bill after migration: %d %d %d", subtotal, tip, total)
	}
	var itemStatus string
	must(db.QueryRow(ctx, `SELECT status FROM order_items WHERE order_id=$1`, readyItemOrder).Scan(&itemStatus))
	if itemStatus != "preparing" {
		t.Fatalf("ready order line became %s", itemStatus)
	}
	rows, err := db.Query(ctx, `SELECT invited_email, role, status, invite_token_hash IS NULL FROM venue_staff WHERE venue_id=$1 ORDER BY invited_email`, venue)
	must(err)
	defer rows.Close()
	want := map[string][3]any{
		"kitchen@kitchen-migration.test": {"waiter", "inactive", true},
		"viewer@kitchen-migration.test":  {"waiter", "inactive", true},
		"waiter@kitchen-migration.test":  {"waiter", "active", false},
	}
	for rows.Next() {
		var email, role, status string
		var tokenCleared bool
		must(rows.Scan(&email, &role, &status, &tokenCleared))
		if got := [3]any{role, status, tokenCleared}; got != want[email] {
			t.Fatalf("%s: %v, want %v", email, got, want[email])
		}
	}
	must(rows.Err())
	for _, sql := range []string{
		`INSERT INTO orders(venue_id,channel,status) VALUES('` + venue + `','dine_in','accepted')`,
		`INSERT INTO orders(venue_id,channel,status) VALUES('` + venue + `','pickup','new')`,
		`INSERT INTO venue_staff(venue_id,invited_email,role,invited_by_user_id) VALUES('` + venue + `','late@kitchen-migration.test','kitchen','` + owner + `')`,
	} {
		if _, err := db.Exec(ctx, sql); err == nil {
			t.Fatalf("removed value still accepted: %s", sql)
		}
	}
}
