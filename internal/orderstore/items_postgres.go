// Package orderstore contains shared PostgreSQL operations used by order-producing
// adapters. The caller owns the transaction and all channel-specific decisions.
package orderstore

import (
	"context"
	"strings"

	"pocket-mvp-backend/internal/money"

	"github.com/jackc/pgx/v5"
)

type MenuItem struct {
	Name       string
	PriceMinor money.Minor
	Currency   string
}

// LockMenuItems reads available items in one round trip and holds their prices
// and category availability stable until the caller commits. UUID ordering
// gives concurrent baskets a consistent menu-row lock order.
func LockMenuItems(ctx context.Context, tx pgx.Tx, venueID string, ids []string, preorder bool) (map[string]MenuItem, error) {
	rows, err := tx.Query(ctx, `
 SELECT mi.id::text, mi.name, mi.price_minor, mi.currency
 FROM menu_items mi
 JOIN menu_categories c ON c.id = mi.category_id AND c.venue_id = mi.venue_id
 WHERE mi.venue_id = $1 AND mi.id = ANY($2::uuid[])
   AND mi.deleted_at IS NULL AND mi.is_available AND c.is_active
   AND (NOT $3::boolean OR c.preorder_enabled)
 ORDER BY mi.id
 FOR SHARE OF mi, c`, venueID, ids, preorder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make(map[string]MenuItem, len(ids))
	for rows.Next() {
		var id string
		var item MenuItem
		if err := rows.Scan(&id, &item.Name, &item.PriceMinor, &item.Currency); err != nil {
			return nil, err
		}
		items[id] = item
	}
	return items, rows.Err()
}

// MenuItemByID accepts the same case-insensitive UUID syntax as PostgreSQL.
func MenuItemByID(items map[string]MenuItem, id string) (MenuItem, bool) {
	item, ok := items[strings.ToLower(id)]
	return item, ok
}

type ItemSnapshot struct {
	MenuItemID string
	Name       string
	PriceMinor money.Minor
	Quantity   int
}

// InsertItems persists immutable receipt values in a single statement. Menu
// changes must never rewrite these snapshots. The input is already validated.
func InsertItems(ctx context.Context, tx pgx.Tx, orderID string, items []ItemSnapshot) error {
	if len(items) == 0 {
		return nil
	}
	ids, names, prices, quantities := snapshotArrays(items)
	_, err := tx.Exec(ctx, `
 INSERT INTO order_items (order_id, menu_item_id, item_name, unit_price_minor, quantity, total_minor)
 SELECT $1, item.id, item.name, item.price, item.quantity, item.price * item.quantity
 FROM unnest($2::uuid[], $3::text[], $4::integer[], $5::integer[])
   AS item(id, name, price, quantity)`, orderID, ids, names, prices, quantities)
	return err
}

func InsertPreorderItems(ctx context.Context, tx pgx.Tx, reservationID string, items []ItemSnapshot) error {
	if len(items) == 0 {
		return nil
	}
	ids, names, prices, quantities := snapshotArrays(items)
	_, err := tx.Exec(ctx, `
 INSERT INTO reservation_preorder_items (reservation_id, menu_item_id, item_name, unit_price_minor, quantity)
 SELECT $1, item.id, item.name, item.price, item.quantity
 FROM unnest($2::uuid[], $3::text[], $4::integer[], $5::integer[])
   AS item(id, name, price, quantity)`, reservationID, ids, names, prices, quantities)
	return err
}

func snapshotArrays(items []ItemSnapshot) ([]string, []string, []int64, []int) {
	ids := make([]string, len(items))
	names := make([]string, len(items))
	prices := make([]int64, len(items))
	quantities := make([]int, len(items))
	for i, item := range items {
		ids[i], names[i], prices[i], quantities[i] = item.MenuItemID, item.Name, int64(item.PriceMinor), item.Quantity
	}
	return ids, names, prices, quantities
}
