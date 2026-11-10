package dinein

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
)

// Both receipt lookup and venue history use this projection and scanOrder.
// The caller joins venue_tables as t, including for removed/renamed tables;
// the immutable identifier snapshot takes precedence over the live identifier.
const orderProjection = `
 o.id::text, o.order_number, o.status, o.total_minor, o.subtotal_minor,
 o.tip_minor, o.currency, COALESCE(o.notes, ''), o.created_at,
 COALESCE(o.table_identifier_snapshot, t.identifier, ''), o.table_qr_version,
 COALESCE((
   SELECT jsonb_agg(jsonb_build_object(
     'menu_item_id', COALESCE(oi.menu_item_id::text, ''),
     'modifiers', oi.modifiers, 'name', oi.item_name,
     'quantity', oi.quantity, 'unit_price_minor', oi.unit_price_minor
   ) ORDER BY oi.created_at, oi.id)
   FROM order_items oi WHERE oi.order_id = o.id
 ), '[]'::jsonb)`

func (r *PostgresRepository) Lookup(ctx context.Context, hash, key, customerID, guestHash string) (Order, error) {
	table, err := r.Resolve(ctx, hash)
	if err != nil {
		return Order{}, err
	}
	// Ownership is part of the receipt query so a concurrent legacy claim cannot
	// change access between a separate authorization query and the read.
	row := r.db.QueryRow(ctx, `SELECT `+orderProjection+`
 FROM orders o LEFT JOIN venue_tables t ON t.id = o.venue_table_id
 WHERE o.venue_id = $1 AND o.venue_table_id = $2
   AND o.channel = 'dine_in' AND o.idempotency_key = $3
   AND (
     (o.customer_user_id IS NULL AND o.guest_history_hash IS NULL)
     OR ($4 <> '' AND o.customer_user_id = NULLIF($4, '')::uuid)
     OR ($4 = '' AND o.customer_user_id IS NULL AND o.guest_history_hash = NULLIF($5, ''))
   )`, table.VenueID, table.ID, key, customerID, guestHash)
	return scanOrder(row, table.VenueID)
}

func loadOrder(ctx context.Context, q queryer, table Table, key string) (Order, error) {
	row := q.QueryRow(ctx, `SELECT `+orderProjection+`
 FROM orders o LEFT JOIN venue_tables t ON t.id = o.venue_table_id
 WHERE o.venue_id = $1 AND o.venue_table_id = $2
   AND o.channel = 'dine_in' AND o.idempotency_key = $3`, table.VenueID, table.ID, key)
	return scanOrder(row, table.VenueID)
}

type orderScanner interface{ Scan(...any) error }

func scanOrder(row orderScanner, venueID string) (Order, error) {
	var order Order
	var raw []byte
	err := row.Scan(
		&order.ID, &order.Number, &order.Status, &order.TotalMinor, &order.SubtotalMinor,
		&order.TipMinor, &order.Currency, &order.Notes, &order.CreatedAt,
		&order.TableIdentifier, &order.QRVersion, &raw,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, appfault.ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	order.VenueID = venueID
	err = json.Unmarshal(raw, &order.Items)
	return order, err
}
