package bootstrap

import (
	"context"
	"os"
	"testing"
)

// Migration 000025 removes kitchen-era data: orders in accepted/ready are deleted with their
// contribution taken out of open bills and with payments that only they referenced, pickup orders
// become online orders, kitchen and viewer staff become inactive waiters.
func TestOrdersWithoutKitchenMigrationIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	apply := func(file string) {
		t.Helper()
		sql, err := os.ReadFile("../../migrations/" + file)
		must(t, err)
		_, err = db.Exec(ctx, string(sql))
		must(t, err)
	}
	apply("000025_orders_without_kitchen.down.sql")

	var bill string
	owner := seedUser(t, db, "owner@kitchen-migration.test", "venue_owner")
	venue := seedVenue(t, db, owner, venueSeed{Slug: "kitchen-migration", Name: "Venue"})
	must(t, db.QueryRow(ctx, `INSERT INTO bills(venue_id,subtotal_minor,tip_minor,total_minor) VALUES($1,6000,300,6300) RETURNING id::text`, venue).Scan(&bill))
	order := func(channel, status string, subtotal, tip int) string {
		var id string
		must(t, db.QueryRow(ctx, `INSERT INTO orders(venue_id,bill_id,channel,status,subtotal_minor,tip_minor,total_minor) VALUES($1,$2,$3,$4,$5::int,$6::int,$7::int) RETURNING id::text`,
			venue, bill, channel, status, subtotal, tip, subtotal+tip).Scan(&id))
		itemStatus := "new"
		if status == "ready" {
			itemStatus = "ready"
		}
		_, err := db.Exec(ctx, `INSERT INTO order_items(order_id,item_name,quantity,unit_price_minor,total_minor,status) VALUES($1,'Soup',1,$2,$2,$3)`, id, subtotal, itemStatus)
		must(t, err)
		return id
	}
	kept := order("dine_in", "preparing", 1000, 100)
	order("dine_in", "accepted", 2000, 100)
	order("dine_in", "ready", 1500, 100)
	pickup := order("pickup", "cancelled", 1500, 0) // kept as an online order
	var pickupPayment, billPayment string
	must(t, db.QueryRow(ctx, `INSERT INTO payments(venue_id,order_id,provider,provider_payment_id,status,amount_minor,currency) VALUES($1,$2,'manual','pickup-payment','refunded',1500,'EUR') RETURNING id::text`, venue, pickup).Scan(&pickupPayment))
	var readyItemOrder string
	must(t, db.QueryRow(ctx, `INSERT INTO orders(venue_id,channel,status) VALUES($1,'dine_in','served') RETURNING id::text`, venue).Scan(&readyItemOrder))
	_, err := db.Exec(ctx, `INSERT INTO order_items(order_id,item_name,quantity,unit_price_minor,total_minor,status) VALUES($1,'Tea',1,300,300,'ready')`, readyItemOrder)
	must(t, err)
	removedID := order("dine_in", "accepted", 0, 0)
	_, err = db.Exec(ctx, `INSERT INTO order_status_events(order_id,actor_name,previous_status,new_status) VALUES($1,'Owner','new','accepted')`, removedID)
	must(t, err)
	// A payment of a removed order alone goes with it; one that also references a bill stays.
	removedReady := order("online", "ready", 900, 0)
	_, err = db.Exec(ctx, `INSERT INTO payments(venue_id,order_id,provider,provider_payment_id,status,amount_minor,currency) VALUES($1,$2,'manual','ready-payment','processing',900,'EUR')`, venue, removedReady)
	must(t, err)
	must(t, db.QueryRow(ctx, `INSERT INTO payments(venue_id,bill_id,order_id,provider,provider_payment_id,status,amount_minor,currency) VALUES($1,$2,$3,'manual','bill-payment','processing',2000,'EUR') RETURNING id::text`, venue, bill, removedID).Scan(&billPayment))
	for _, role := range []string{"kitchen", "viewer", "waiter"} {
		_, err := db.Exec(ctx, `INSERT INTO venue_staff(venue_id,invited_email,role,status,invited_by_user_id,invite_token_hash) VALUES($1,$2,$3,'active',$4,$2)`,
			venue, role+"@kitchen-migration.test", role, owner)
		must(t, err)
	}

	apply("000025_orders_without_kitchen.up.sql")

	var orders, keptCount int
	must(t, db.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE id=$2) FROM orders WHERE venue_id=$1`, venue, kept).Scan(&orders, &keptCount))
	if orders != 3 || keptCount != 1 {
		t.Fatalf("orders after migration: %d, kept %d", orders, keptCount)
	}
	var subtotal, tip, total int
	must(t, db.QueryRow(ctx, `SELECT subtotal_minor,tip_minor,total_minor FROM bills WHERE id=$1`, bill).Scan(&subtotal, &tip, &total))
	if subtotal != 1600 || tip != 100 || total != 1700 {
		t.Fatalf("bill after migration: %d %d %d", subtotal, tip, total)
	}
	var pickupChannel string
	must(t, db.QueryRow(ctx, `SELECT channel FROM orders WHERE id=$1`, pickup).Scan(&pickupChannel))
	if pickupChannel != "online" {
		t.Fatalf("pickup order became %s", pickupChannel)
	}
	var payments int
	var keptPickupPayment, keptBillPayment bool
	must(t, db.QueryRow(ctx, `SELECT count(*), bool_or(id=$2), bool_or(id=$3 AND order_id IS NULL) FROM payments WHERE venue_id=$1`, venue, pickupPayment, billPayment).Scan(&payments, &keptPickupPayment, &keptBillPayment))
	if payments != 2 || !keptPickupPayment || !keptBillPayment {
		t.Fatalf("payments after migration: %d, pickup kept %v, bill payment kept %v", payments, keptPickupPayment, keptBillPayment)
	}
	var itemStatus string
	must(t, db.QueryRow(ctx, `SELECT status FROM order_items WHERE order_id=$1`, readyItemOrder).Scan(&itemStatus))
	if itemStatus != "preparing" {
		t.Fatalf("ready order line became %s", itemStatus)
	}
	rows, err := db.Query(ctx, `SELECT invited_email, role, status, invite_token_hash IS NULL FROM venue_staff WHERE venue_id=$1 ORDER BY invited_email`, venue)
	must(t, err)
	defer rows.Close()
	want := map[string][3]any{
		"kitchen@kitchen-migration.test": {"waiter", "inactive", true},
		"viewer@kitchen-migration.test":  {"waiter", "inactive", true},
		"waiter@kitchen-migration.test":  {"waiter", "active", false},
	}
	for rows.Next() {
		var email, role, status string
		var tokenCleared bool
		must(t, rows.Scan(&email, &role, &status, &tokenCleared))
		if got := [3]any{role, status, tokenCleared}; got != want[email] {
			t.Fatalf("%s: %v, want %v", email, got, want[email])
		}
	}
	must(t, rows.Err())
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
