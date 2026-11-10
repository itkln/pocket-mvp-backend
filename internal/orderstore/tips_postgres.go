package orderstore

import (
	"context"
	"pocket-mvp-backend/internal/money"

	"github.com/jackc/pgx/v5"
)

// ApplyOrderTip is part of the order transaction. These are pending tips, not paid income.
// Payment initiation must use the persisted order total; only settlement may mark tips paid.
func ApplyOrderTip(ctx context.Context, tx pgx.Tx, venueID, orderID, currency string, amount money.Minor) error {
	if amount == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `
        UPDATE orders SET
            tip_minor = $2,
            total_minor = subtotal_minor + tax_minor + service_fee_minor - discount_minor + $2
        WHERE id = $1`, orderID, amount); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
        INSERT INTO tips (venue_id, order_id, amount_minor, currency, status, idempotency_key)
        VALUES ($1, $2, $3, $4, 'pending', $5)`, venueID, orderID, amount, currency, orderID)
	return err
}
