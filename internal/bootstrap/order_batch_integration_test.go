package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"pocket-mvp-backend/internal/money"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/gratuity"
	"pocket-mvp-backend/internal/modules/dinein"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type orderQueryCounter struct{ count atomic.Int64 }

func (c *orderQueryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.count.Add(1)
	return ctx
}
func (*orderQueryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// This is also a reproducible query-count profile. It calls the real service
// and transaction; setup, receipt verification and retries are outside timing.
func TestTableOrderBatchIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	var categoryID string
	ownerID := seedUser(t, db, "batch@example.test", "venue_owner")
	venueID := seedVenue(t, db, ownerID, venueSeed{Slug: "batch-test", Name: "Batch", Settings: `{"floor_plan":[{"name":"Main","tables":[{"id":"01","seats":4,"x":0,"y":0}]}]}`})
	must(t, db.QueryRow(ctx, `INSERT INTO menu_categories(venue_id,name) VALUES($1,'Food') RETURNING id::text`, venueID).Scan(&categoryID))
	inputs := make([]dinein.ItemInput, 100)
	names := make(map[string]string, 100)
	for i := range inputs {
		var id string
		price := money.Minor(100 + i)
		name := fmt.Sprintf("Dish %d", i)
		must(t, db.QueryRow(ctx, `INSERT INTO menu_items(venue_id,category_id,name,price_minor) VALUES($1,$2,$3,$4) RETURNING id::text`, venueID, categoryID, name, price).Scan(&id))
		inputs[i] = dinein.ItemInput{MenuItemID: strings.ToUpper(id), Quantity: i%3 + 1, ExpectedPriceMinor: price}
		names[id] = name
	}
	counter := &orderQueryCounter{}
	cfg := db.Config()
	cfg.ConnConfig.Tracer = counter
	measured, err := pgxpool.NewWithConfig(ctx, cfg)
	must(t, err)
	defer measured.Close()
	protector := testProtector(t)
	service := dinein.NewService(dinein.NewPostgresRepository(measured), access.NewVenueAuthorizer(measured), protector, "https://example.test")
	qr, err := service.OwnerQRCode(ctx, ownerID, venueID, "01", false)
	must(t, err)
	token := qr.URL[strings.LastIndex(qr.URL, "/")+1:]
	for _, size := range []int{1, 10, 100} {
		t.Run(fmt.Sprintf("items_%d", size), func(t *testing.T) {
			input := dinein.CreateInput{Token: token, IdempotencyKey: fmt.Sprintf("batch-order-key-%d", size), Items: inputs[:size], Tip: &gratuity.Selection{Mode: "percent", Percent: 5}}
			subtotal := money.Minor(0)
			expected := map[string]dinein.ItemInput{}
			for _, item := range input.Items {
				subtotal += item.ExpectedPriceMinor.Times(item.Quantity)
				expected[strings.ToLower(item.MenuItemID)] = item
			}
			tip, err := gratuity.Calculate(subtotal, input.Tip)
			must(t, err)
			total := subtotal + tip
			input.ExpectedTotalMinor = &total
			counter.count.Store(0)
			started := time.Now()
			order, err := service.Create(ctx, input)
			elapsed := time.Since(started)
			queries := counter.count.Load()
			must(t, err)
			t.Logf("items=%d statements=%d elapsed=%s", size, queries, elapsed)
			if queries > 20 {
				t.Fatalf("query budget exceeded for %d items: %d; check for per-item queries", size, queries)
			}
			if order.TotalMinor != total || order.SubtotalMinor != subtotal || order.TipMinor != tip || len(order.Items) != size {
				t.Fatalf("incorrect receipt totals: %+v", order)
			}
			for _, saved := range order.Items {
				original, ok := expected[saved.MenuItemID]
				if !ok || saved.Name != names[saved.MenuItemID] || saved.Quantity != original.Quantity || saved.UnitPriceMinor != original.ExpectedPriceMinor {
					t.Fatalf("mismatched snapshot: %+v", saved)
				}
				delete(expected, saved.MenuItemID)
			}
			if len(expected) != 0 {
				t.Fatal("missing receipt items")
			}
			// A changed live price cannot invalidate a matching retry.
			_, err = db.Exec(ctx, `UPDATE menu_items SET price_minor=price_minor+1 WHERE id=$1`, input.Items[0].MenuItemID)
			must(t, err)
			replay, err := service.Create(ctx, input)
			must(t, err)
			if replay.ID != order.ID || replay.TotalMinor != total {
				t.Fatal("retry changed persisted order")
			}
			_, err = db.Exec(ctx, `UPDATE menu_items SET price_minor=price_minor-1 WHERE id=$1`, input.Items[0].MenuItemID)
			must(t, err)
		})
	}
	// Failure after all item/tip writes must roll back the entire transaction.
	_, err = db.Exec(ctx, `UPDATE bills SET subtotal_minor=2000000000,tip_minor=0,total_minor=2000000000 WHERE venue_id=$1`, venueID)
	must(t, err)
	input := dinein.CreateInput{Token: token, IdempotencyKey: "batch-rollback-order-key", Items: inputs[:10], Tip: &gratuity.Selection{Mode: "percent", Percent: 10}}
	var ordersBefore, itemsBefore, tipsBefore int
	must(t, db.QueryRow(ctx, `SELECT (SELECT count(*) FROM orders),(SELECT count(*) FROM order_items),(SELECT count(*) FROM tips)`).Scan(&ordersBefore, &itemsBefore, &tipsBefore))
	if _, err = service.Create(ctx, input); !errors.Is(err, appfault.ErrInvalidInput) {
		t.Fatalf("overflow: %v", err)
	}
	var ordersAfter, itemsAfter, tipsAfter, total int
	must(t, db.QueryRow(ctx, `SELECT (SELECT count(*) FROM orders),(SELECT count(*) FROM order_items),(SELECT count(*) FROM tips),total_minor FROM bills WHERE venue_id=$1`, venueID).Scan(&ordersAfter, &itemsAfter, &tipsAfter, &total))
	if ordersBefore != ordersAfter || itemsBefore != itemsAfter || tipsBefore != tipsAfter || total != 2000000000 {
		t.Fatal("partial order committed after bill overflow")
	}
	// A missing item at the end of a large basket must not accept the valid prefix.
	input.IdempotencyKey = "batch-unavailable-order-key"
	input.Items = append([]dinein.ItemInput(nil), inputs...)
	input.Items[99].MenuItemID = "10000000-0000-4000-8000-000000000099"
	if _, err = service.Create(ctx, input); !errors.Is(err, dinein.ErrItemUnavailable) {
		t.Fatalf("partial batch accepted: %v", err)
	}
}
