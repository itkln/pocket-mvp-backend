package ordering

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
)

type lockedOrder struct{ status, channel, billID, billStatus string }

func (r *PostgresRepository) UpdateStatus(ctx context.Context, venueID, orderID, status string, changes ...StatusChange) (Order, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)
	role := "owner"
	var change StatusChange
	if len(changes) > 0 {
		change = changes[0]
	}
	if change.ActorID != "" {
		role, err = access.ResolveOrderRole(ctx, tx, change.ActorID, venueID, true)
		if err != nil {
			return Order{}, err
		}
	}
	if change.ActorID != "" {
		if err := validateCancellation(status, change); err != nil {
			return Order{}, err
		}
	}
	state, err := lockStatusChange(ctx, tx, venueID, orderID, status, role, change.ExpectedStatus)
	if err != nil {
		return Order{}, err
	}
	if state.status == status {
		order, err := readCurrentOrder(ctx, tx, venueID, orderID)
		if err != nil {
			return Order{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return Order{}, err
		}
		return r.orderResponse(ctx, venueID, order.ID, role)
	}
	order, err := updateOrderStatus(ctx, tx, venueID, orderID, status)
	if err != nil {
		if errors.Is(err, appfault.ErrConflict) {
			return Order{}, &StatusConflict{state.status}
		}
		return Order{}, err
	}
	if status == "cancelled" && state.status != "cancelled" {
		if err = cancelOrderContribution(ctx, tx, orderID, state.billID); err != nil {
			return Order{}, err
		}
	}
	actorName := "System"
	if change.ActorID != "" {
		if err = tx.QueryRow(ctx, `SELECT trim(first_name || ' ' || last_name) FROM users WHERE id=$1`, change.ActorID).Scan(&actorName); err != nil {
			return Order{}, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO order_status_events(order_id,actor_id,actor_name,previous_status,new_status,reason,comment) VALUES($1,NULLIF($2,'')::uuid,$3,$4,$5,$6,$7)`, orderID, change.ActorID, actorName, state.status, status, change.Reason, change.Comment); err != nil {
		return Order{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return r.orderResponse(ctx, venueID, order.ID, role)
}

func lockStatusChange(ctx context.Context, tx pgx.Tx, venueID, orderID, status, role, expected string) (lockedOrder, error) {
	var state lockedOrder
	err := tx.QueryRow(ctx, `
        SELECT status, channel FROM orders
        WHERE venue_id = $1 AND id = $2 FOR UPDATE`, venueID, orderID).Scan(&state.status, &state.channel)
	if errors.Is(err, pgx.ErrNoRows) {
		return lockedOrder{}, appfault.ErrNotFound
	}
	if err != nil {
		return lockedOrder{}, err
	}
	if err = validateTransition(role, state.channel, state.status, status, expected); err != nil {
		return lockedOrder{}, err
	}
	// Keep the same order-then-bill lock order as reservation cancellation.

	err = tx.QueryRow(ctx, `
        SELECT b.id::text, b.status FROM bills b JOIN orders o ON o.bill_id = b.id
        WHERE o.venue_id = $1 AND o.id = $2 FOR UPDATE OF b`, venueID, orderID).Scan(&state.billID, &state.billStatus)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return lockedOrder{}, err
	}
	if status == "cancelled" && state.status != "cancelled" {
		var hasPayment bool
		if err = tx.QueryRow(ctx, `
            SELECT EXISTS (
                SELECT 1 FROM payments
                WHERE (order_id = $1 OR bill_id = NULLIF($2, '')::uuid)
                    AND status NOT IN ('failed', 'cancelled')
            ) OR EXISTS (
                SELECT 1 FROM tips WHERE order_id = $1 AND status = 'paid'
            )`, orderID, state.billID).Scan(&hasPayment); err != nil {
			return lockedOrder{}, err
		}
		if hasPayment || (state.billID != "" && state.billStatus != "open") {
			return lockedOrder{}, &StatusConflict{state.status}
		}
	}

	return state, nil
}

func updateOrderStatus(ctx context.Context, tx pgx.Tx, venueID, orderID, status string) (Order, error) {
	order, err := scanOrder(tx.QueryRow(ctx, `
		WITH updated AS (
			UPDATE orders SET status = $3,
				completed_at = CASE WHEN $3 IN ('completed', 'cancelled') THEN now() ELSE completed_at END
			WHERE id = $2 AND venue_id = $1
				AND (status <> 'cancelled' OR $3 = 'cancelled')
				AND ($3 = 'cancelled' OR NOT EXISTS (SELECT 1 FROM reservations r WHERE r.id = orders.reservation_id AND r.status = 'cancelled'))
			RETURNING *
		)
		SELECT o.id::text, o.order_number, o.channel,
			CASE WHEN t.identifier IS NOT NULL THEN 'Стол ' || t.identifier ELSE 'Онлайн' END,
			COALESCE(o.guest_name, ''), o.total_minor, o.tip_minor, o.currency, o.status,
			COALESCE(o.notes, ''), o.created_at,
			COALESCE(r.reservation_code, ''), r.starts_at, COALESCE(r.guest_phone, ''), COALESCE(r.guest_count, 0)
		FROM updated o LEFT JOIN venue_tables t ON t.id = o.venue_table_id
		LEFT JOIN reservations r ON r.id = o.reservation_id`, venueID, orderID, status))
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if queryErr := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM orders WHERE id = $1 AND venue_id = $2)`, orderID, venueID).Scan(&exists); queryErr != nil {
			return Order{}, queryErr
		}
		if exists {
			return Order{}, appfault.ErrConflict
		}
		return Order{}, appfault.ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	return order, nil
}

// Called only on the first successful cancellation, under the order/bill locks.
func cancelOrderContribution(ctx context.Context, tx pgx.Tx, orderID, billID string) error {
	if _, err := tx.Exec(ctx, `
        UPDATE tips SET status = 'cancelled'
        WHERE order_id = $1 AND status = 'pending'`, orderID); err != nil {
		return err
	}
	if billID != "" {
		_, err := tx.Exec(ctx, `
            UPDATE bills b SET
                subtotal_minor = b.subtotal_minor - o.subtotal_minor,
                tip_minor = b.tip_minor - o.tip_minor,
                total_minor = b.total_minor - o.total_minor
            FROM orders o WHERE b.id = $1 AND o.id = $2`, billID, orderID)
		return err
	}
	return nil
}

func (r *PostgresRepository) orderResponse(ctx context.Context, venueID, orderID, role string) (Order, error) {
	order, err := r.Get(ctx, venueID, orderID)
	if err != nil {
		return Order{}, err
	}
	order.applyRole(role)
	return order, nil
}

func readCurrentOrder(ctx context.Context, tx pgx.Tx, venueID, orderID string) (Order, error) {
	return scanOrder(tx.QueryRow(ctx, `SELECT o.id::text,o.order_number,o.channel,
 CASE WHEN t.identifier IS NOT NULL THEN 'Стол '||t.identifier ELSE 'Онлайн' END,
 COALESCE(o.guest_name,''),o.total_minor,o.tip_minor,o.currency,o.status,COALESCE(o.notes,''),o.created_at,
 COALESCE(r.reservation_code,''),r.starts_at,COALESCE(r.guest_phone,''),COALESCE(r.guest_count,0)
 FROM orders o LEFT JOIN venue_tables t ON t.id=o.venue_table_id LEFT JOIN reservations r ON r.id=o.reservation_id
 WHERE o.venue_id=$1 AND o.id=$2`, venueID, orderID))
}
