package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"pocket-mvp-backend/internal/money"

	"pocket-mvp-backend/internal/modules/reservations"
	"pocket-mvp-backend/internal/orderstore"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func orderstoreFixture(t *testing.T) (*pgxpool.Pool, string, string, string, []reservations.PreorderItem) {
	t.Helper()
	db := resetTestDatabase(t)
	ctx := context.Background()
	var category, layout, table string
	owner := seedUser(t, db, "store@example.test", "")
	venue := seedVenue(t, db, owner, venueSeed{Slug: "store-test", Name: "Store", Settings: `{"preorder_enabled":true}`})
	must(t, db.QueryRow(ctx, `INSERT INTO menu_categories(venue_id,name) VALUES($1,'Food') RETURNING id::text`, venue).Scan(&category))
	must(t, db.QueryRow(ctx, `INSERT INTO venue_layouts(venue_id,name) VALUES($1,'Main') RETURNING id::text`, venue).Scan(&layout))
	must(t, db.QueryRow(ctx, `INSERT INTO venue_tables(venue_id,layout_id,identifier,seats,qr_token_hash) VALUES($1,$2,'01',4,'store-qr') RETURNING id::text`, venue, layout).Scan(&table))
	items := make([]reservations.PreorderItem, 10)
	for i := range items {
		var id string
		price := money.Minor(100 + i)
		must(t, db.QueryRow(ctx, `INSERT INTO menu_items(venue_id,category_id,name,price_minor) VALUES($1,$2,$3,$4) RETURNING id::text`, venue, category, fmt.Sprintf("Dish %d", i), price).Scan(&id))
		items[i] = reservations.PreorderItem{MenuItemID: id, Quantity: i%3 + 1, ExpectedPriceMinor: price}
	}
	return db, venue, category, table, items
}

func TestMenuSnapshotLocksIntegration(t *testing.T) {
	db, venue, category, _, items := orderstoreFixture(t)
	ctx := context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.MenuItemID
	}
	menu, err := orderstore.LockMenuItems(ctx, tx, venue, ids, false)
	if err != nil || len(menu) != len(items) {
		t.Fatalf("lock menu: %d, %v", len(menu), err)
	}
	for _, candidate := range []struct{ name, sql, id string }{
		{"price", `UPDATE menu_items SET price_minor=price_minor+1 WHERE id=$1`, items[0].MenuItemID},
		{"category", `UPDATE menu_categories SET is_active=false WHERE id=$1`, category},
	} {
		t.Run(candidate.name, func(t *testing.T) {
			other, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Rollback(ctx)
			if _, err = other.Exec(ctx, `SET LOCAL lock_timeout='50ms'`); err != nil {
				t.Fatal(err)
			}
			_, err = other.Exec(ctx, candidate.sql, candidate.id)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
				t.Fatalf("snapshot did not hold row lock: %v", err)
			}
		})
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE menu_items SET price_minor=price_minor+1 WHERE id=$1`, items[0].MenuItemID); err != nil {
		t.Fatal(err)
	}
}

func TestPreorderBatchSnapshotsIntegration(t *testing.T) {
	db, venue, _, table, items := orderstoreFixture(t)
	ctx := context.Background()
	repo := reservations.NewPostgresRepository(db)
	subtotal := money.Minor(0)
	for _, item := range items {
		subtotal += item.ExpectedPriceMinor.Times(item.Quantity)
	}
	for _, held := range []bool{false, true} {
		t.Run(fmt.Sprintf("held_%t", held), func(t *testing.T) {
			start := time.Now().Truncate(time.Second).Add(24 * time.Hour)
			if held {
				start = start.Add(24 * time.Hour)
			}
			input := reservations.CreateInput{StartsAt: start, Guests: 2, GuestName: "Batch Guest", GuestPhone: "+421900123456", Items: items, IdempotencyKey: fmt.Sprintf("preorder-batch-key-%t", held), ExpectedTotalMinor: &subtotal}
			slot := reservations.Slot{TableID: table, StartsAt: start, EndsAt: start.Add(time.Hour)}
			var result reservations.Result
			var err error
			if held {
				hold, e := repo.Hold(ctx, venue, input, "hold-hash", slot)
				if e != nil {
					t.Fatal(e)
				}
				result, err = repo.Confirm(ctx, venue, hold.ID, input, "hold-hash", "confirm-hash")
			} else {
				result, err = repo.Create(ctx, venue, input, "create-hash", slot)
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.PreorderTotalMinor != subtotal || result.PreorderSubtotalMinor != subtotal {
				t.Fatalf("wrong total: %+v", result)
			}
			var count int
			err = db.QueryRow(ctx, `SELECT count(*) FROM reservation_preorder_items rp
 JOIN orders o ON o.reservation_id=rp.reservation_id AND o.channel='preorder'
 JOIN order_items oi ON oi.order_id=o.id AND oi.menu_item_id=rp.menu_item_id
 JOIN menu_items mi ON mi.id=rp.menu_item_id
 WHERE rp.reservation_id=$1 AND rp.item_name=mi.name AND oi.item_name=rp.item_name
 AND oi.quantity=rp.quantity AND oi.unit_price_minor=rp.unit_price_minor
 AND oi.total_minor=rp.quantity*rp.unit_price_minor`, result.ID).Scan(&count)
			if err != nil || count != len(items) {
				t.Fatalf("snapshot mismatch: %d, %v", count, err)
			}
		})
	}
}
