package reservations

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
)

func (r *PostgresRepository) CancelCustomer(ctx context.Context, customerID, reservationID string) (CustomerReservation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return CustomerReservation{}, err
	}
	defer tx.Rollback(ctx)
	var status string
	// Ownership is part of the lock query. Foreign IDs and unfinished holds are indistinguishable from missing IDs.
	err = tx.QueryRow(ctx, `SELECT status FROM reservations WHERE id = $1 AND customer_user_id = $2
		AND status <> 'pending' AND hold_expires_at IS NULL FOR UPDATE`, reservationID, customerID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerReservation{}, appfault.ErrNotFound
	}
	if err != nil {
		return CustomerReservation{}, err
	}
	if status != "cancelled" {
		// Lock orders before checking eligibility. Vendor status changes serialize on these rows.
		// FOR UPDATE also prevents new payment/item foreign keys being attached during cancellation.
		orders, err := tx.Query(ctx, `SELECT id FROM orders WHERE reservation_id = $1 AND channel = 'preorder' ORDER BY id FOR UPDATE`, reservationID)
		if err != nil {
			return CustomerReservation{}, err
		}
		for orders.Next() {
		}
		err = orders.Err()
		orders.Close()
		if err != nil {
			return CustomerReservation{}, err
		}
		for _, query := range []string{
			`SELECT b.id FROM bills b WHERE b.id IN (SELECT bill_id FROM orders WHERE reservation_id = $1 AND channel = 'preorder') ORDER BY b.id FOR UPDATE`,
			`SELECT p.id FROM payments p WHERE p.order_id IN (SELECT id FROM orders WHERE reservation_id = $1 AND channel = 'preorder')
				OR p.bill_id IN (SELECT bill_id FROM orders WHERE reservation_id = $1 AND channel = 'preorder') ORDER BY p.id FOR UPDATE`,
		} {
			rows, err := tx.Query(ctx, query, reservationID)
			if err != nil {
				return CustomerReservation{}, err
			}
			for rows.Next() {
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return CustomerReservation{}, err
			}
		}
		current, err := listCustomer(ctx, tx, customerID, reservationID)
		if err != nil {
			return CustomerReservation{}, err
		}
		if !current[0].Cancellation.Allowed {
			return CustomerReservation{}, &CancellationDenied{Reason: current[0].Cancellation.Reason}
		}
		if _, err := tx.Exec(ctx, `UPDATE order_items SET status = 'cancelled' WHERE order_id IN
			(SELECT id FROM orders WHERE reservation_id = $1 AND channel = 'preorder') AND status <> 'cancelled'`, reservationID); err != nil {
			return CustomerReservation{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE orders SET status = 'cancelled', completed_at = now()
			WHERE reservation_id = $1 AND channel = 'preorder' AND status <> 'cancelled'`, reservationID); err != nil {
			return CustomerReservation{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE tips SET status='cancelled' WHERE status='pending' AND order_id IN (SELECT id FROM orders WHERE reservation_id=$1 AND channel='preorder')`, reservationID); err != nil {
			return CustomerReservation{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE reservations SET status = 'cancelled' WHERE id = $1`, reservationID); err != nil {
			return CustomerReservation{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (venue_id, actor_user_id, action, entity_type, entity_id, changes)
			SELECT venue_id, $2, 'reservation.cancelled_by_customer', 'reservation', id,
			jsonb_build_object('status', 'cancelled', 'previous_status', $3::text) FROM reservations WHERE id = $1`, reservationID, customerID, status); err != nil {
			return CustomerReservation{}, err
		}
	}
	result, err := listCustomer(ctx, tx, customerID, reservationID)
	if err != nil {
		return CustomerReservation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CustomerReservation{}, err
	}
	return result[0], nil
}
