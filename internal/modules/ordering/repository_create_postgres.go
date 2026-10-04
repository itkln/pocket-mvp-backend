package ordering

import (
	"context"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
)

func (r *PostgresRepository) Quote(ctx context.Context, venueID string, input CreateInput) (Quote, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Quote{}, err
	}
	defer tx.Rollback(ctx)
	quote, err := quoteInTransaction(ctx, tx, venueID, input)
	if err != nil {
		return Quote{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Quote{}, err
	}
	return quote, nil
}

func quoteInTransaction(ctx context.Context, tx pgx.Tx, venueID string, input CreateInput) (Quote, error) {
	var quote Quote
	err := tx.QueryRow(ctx, `SELECT currency FROM venues WHERE id = $1 AND status = 'active' AND deleted_at IS NULL FOR SHARE`, venueID).Scan(&quote.Currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return Quote{}, appfault.ErrNotFound
	}
	if err != nil {
		return Quote{}, err
	}
	quote.Items = make([]OrderItem, 0, len(input.Items))
	for _, selected := range input.Items {
		var item OrderItem
		var currency string
		err := tx.QueryRow(ctx, `SELECT m.id::text, m.name, m.price_minor, m.currency
			FROM menu_items m JOIN menu_categories c ON c.id = m.category_id
			WHERE m.id = $1 AND m.venue_id = $2 AND c.venue_id = $2 AND c.is_active
				AND m.is_available AND m.deleted_at IS NULL FOR SHARE OF m, c`, selected.MenuItemID, venueID).
			Scan(&item.MenuItemID, &item.Name, &item.UnitPriceMinor, &currency)
		if errors.Is(err, pgx.ErrNoRows) {
			return Quote{}, ErrItemUnavailable
		}
		if err != nil {
			return Quote{}, err
		}
		if currency != quote.Currency {
			return Quote{}, appfault.ErrInvalidInput
		}
		if item.UnitPriceMinor != selected.ExpectedPriceMinor {
			return Quote{}, ErrPriceChanged
		}
		lineTotal := int64(item.UnitPriceMinor) * int64(selected.Quantity)
		if lineTotal > math.MaxInt32 || int64(quote.TotalMinor)+lineTotal > math.MaxInt32 {
			return Quote{}, appfault.ErrInvalidInput
		}
		item.Quantity = selected.Quantity
		item.TotalMinor = int(lineTotal)
		quote.TotalMinor += item.TotalMinor
		quote.Items = append(quote.Items, item)
	}
	return quote, nil
}

func (r *PostgresRepository) Submit(ctx context.Context, venueID string, input CreateInput, requestHash string) (Order, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)
	// One operation key is serialized before reading or writing its order.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, venueID+":"+input.IdempotencyKey); err != nil {
		return Order{}, err
	}
	var existingID, existingHash string
	err = tx.QueryRow(ctx, `SELECT id::text, COALESCE(request_hash, '') FROM orders WHERE venue_id = $1 AND idempotency_key = $2`, venueID, input.IdempotencyKey).Scan(&existingID, &existingHash)
	if err == nil {
		if existingHash != requestHash {
			return Order{}, appfault.ErrConflict
		}
		return readOrder(ctx, tx, existingID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Order{}, err
	}
	quote, err := quoteInTransaction(ctx, tx, venueID, input)
	if err != nil {
		return Order{}, err
	}
	var orderID string
	err = tx.QueryRow(ctx, `INSERT INTO orders (venue_id, customer_user_id, channel, status, guest_name, guest_email,
		currency, subtotal_minor, total_minor, notes, idempotency_key, request_hash, placed_at)
		VALUES ($1, NULLIF($2, '')::uuid, 'online', 'new', $3, $4, $5, $6, $6, NULLIF($7, ''), $8, $9, now())
		RETURNING id::text`, venueID, input.CustomerUserID, input.GuestName, input.GuestEmail,
		quote.Currency, quote.TotalMinor, input.Notes, input.IdempotencyKey, requestHash).Scan(&orderID)
	if err != nil {
		return Order{}, appfault.MapWriteError(err)
	}
	for _, item := range quote.Items {
		_, err = tx.Exec(ctx, `INSERT INTO order_items (order_id, menu_item_id, item_name, unit_price_minor, quantity, total_minor)
			VALUES ($1, $2, $3, $4, $5, $6)`, orderID, item.MenuItemID, item.Name, item.UnitPriceMinor, item.Quantity, item.TotalMinor)
		if err != nil {
			return Order{}, appfault.MapWriteError(err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (venue_id, actor_user_id, action, entity_type, entity_id, changes)
		VALUES ($1, NULLIF($2, '')::uuid, 'order.created', 'order', $3, jsonb_build_object('total_minor', $4::integer, 'currency', $5::text))`,
		venueID, input.CustomerUserID, orderID, quote.TotalMinor, quote.Currency); err != nil {
		return Order{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (topic, aggregate_type, aggregate_id, payload)
		VALUES ('order.created', 'order', $1, jsonb_build_object('order_id', $1::uuid::text, 'venue_id', $2::uuid::text))`, orderID, venueID); err != nil {
		return Order{}, err
	}
	order, err := readOrder(ctx, tx, orderID)
	if err != nil {
		return Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, appfault.MapWriteError(err)
	}
	return order, nil
}

type orderReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func readOrder(ctx context.Context, db orderReader, id string) (Order, error) {
	var order Order
	err := db.QueryRow(ctx, `SELECT o.id::text, o.order_number, o.channel, o.status, COALESCE(o.guest_name, ''),
		o.total_minor, o.currency, COALESCE(o.notes, ''), o.created_at, v.name,
		COALESCE(r.reservation_code, ''), r.starts_at
		FROM orders o JOIN venues v ON v.id = o.venue_id
		LEFT JOIN reservations r ON r.id = o.reservation_id WHERE o.id = $1`, id).
		Scan(&order.ID, &order.Number, &order.Channel, &order.Status, &order.GuestName,
			&order.TotalMinor, &order.Currency, &order.Notes, &order.CreatedAt, &order.VenueName,
			&order.ReservationCode, &order.ReservationStartsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, appfault.ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	rows, err := db.Query(ctx, `SELECT COALESCE(menu_item_id::text, ''), item_name, quantity, unit_price_minor, total_minor
		FROM order_items WHERE order_id = $1 ORDER BY created_at, id`, id)
	if err != nil {
		return Order{}, err
	}
	defer rows.Close()
	order.Items = []OrderItem{}
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(&item.MenuItemID, &item.Name, &item.Quantity, &item.UnitPriceMinor, &item.TotalMinor); err != nil {
			return Order{}, err
		}
		order.Items = append(order.Items, item)
	}
	return order, rows.Err()
}

func (r *PostgresRepository) ListCustomer(ctx context.Context, customerID string) ([]Order, error) {
	rows, err := r.db.Query(ctx, `SELECT id::text FROM orders WHERE customer_user_id = $1 AND status <> 'draft' ORDER BY created_at DESC LIMIT 100`, customerID)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	orders := make([]Order, 0, len(ids))
	for _, id := range ids {
		order, err := readOrder(ctx, r.db, id)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

func (r *PostgresRepository) GetPublic(ctx context.Context, venueID, orderID, key string) (Order, error) {
	var id string
	err := r.db.QueryRow(ctx, `SELECT id::text FROM orders WHERE id = $1 AND venue_id = $2 AND idempotency_key = $3 AND status <> 'draft'`, orderID, venueID, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, appfault.ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	return readOrder(ctx, r.db, id)
}
