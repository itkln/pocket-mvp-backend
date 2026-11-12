package dinein

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
)

// billSelect computes a bill from its orders, never from the cached bills totals: cancelled and
// draft orders, cancelled items and the tips of fully cancelled orders do not count. The guest's
// bill, the staff list and the settlement all use it, so they always agree on the amount.
const billSelect = `
 SELECT b.id::text,b.venue_id::text,b.venue_table_id::text,COALESCE(t.identifier,''),b.status,b.currency,
 b.opened_at,b.payment_requested_at,
 COALESCE((SELECT sum(oi.total_minor) FROM order_items oi JOIN orders o ON o.id=oi.order_id
 WHERE o.bill_id=b.id AND o.venue_id=b.venue_id AND o.venue_table_id=b.venue_table_id
 AND o.status NOT IN ('draft','cancelled') AND oi.status<>'cancelled'),0),
 COALESCE((SELECT sum(o.tip_minor) FROM orders o WHERE o.bill_id=b.id
 AND o.venue_id=b.venue_id AND o.venue_table_id=b.venue_table_id AND o.status NOT IN ('draft','cancelled')
 AND EXISTS (SELECT 1 FROM order_items oi WHERE oi.order_id=o.id AND oi.status<>'cancelled')),0),
 b.tax_minor,b.service_fee_minor,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('order_id',o.id,'order_number',o.order_number,
 'name',oi.item_name,'quantity',oi.quantity,'unit_price_minor',oi.unit_price_minor,'total_minor',oi.total_minor)
 ORDER BY o.created_at,o.id,oi.created_at,oi.id)
 FROM orders o JOIN order_items oi ON oi.order_id=o.id
 WHERE o.bill_id=b.id AND o.venue_id=b.venue_id AND o.venue_table_id=b.venue_table_id
 AND o.status NOT IN ('draft','cancelled') AND oi.status<>'cancelled'),'[]'::jsonb)
 FROM bills b LEFT JOIN venue_tables t ON t.id=b.venue_table_id `

type rowsQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

// readBills runs billSelect with the given WHERE/ORDER clause.
func readBills(ctx context.Context, q rowsQueryer, clause string, args ...any) ([]Bill, error) {
	rows, err := q.Query(ctx, billSelect+clause, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bills := []Bill{}
	for rows.Next() {
		var b Bill
		var items []byte
		if err = rows.Scan(&b.ID, &b.VenueID, &b.TableID, &b.TableIdentifier, &b.Status, &b.Currency, &b.OpenedAt,
			&b.PaymentRequestedAt, &b.SubtotalMinor, &b.TipMinor, &b.TaxMinor, &b.ServiceFeeMinor, &items); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(items, &b.Items); err != nil {
			return nil, err
		}
		b.TotalMinor = b.SubtotalMinor + b.TipMinor + b.TaxMinor + b.ServiceFeeMinor
		bills = append(bills, b)
	}
	return bills, rows.Err()
}

func readBill(ctx context.Context, q rowsQueryer, clause string, args ...any) (*Bill, error) {
	bills, err := readBills(ctx, q, clause, args...)
	if err != nil || len(bills) == 0 {
		return nil, err
	}
	return &bills[0], nil
}

const activeTableBill = `WHERE b.venue_id=$1 AND b.venue_table_id=$2 AND b.status IN ('open','payment_pending')`

func (r *PostgresRepository) CurrentBill(ctx context.Context, hash string) (*Bill, error) {
	// Resolve and bill rows share a snapshot, including concurrent QR rotation,
	// cancellation and settlement. Reading never creates or reopens a visit.
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	table, err := resolve(ctx, tx, hash, "")
	if err != nil {
		return nil, err
	}
	bill, err := readBill(ctx, tx, activeTableBill, table.VenueID, table.ID)
	if err != nil {
		return nil, err
	}
	return bill, tx.Commit(ctx)
}

func (r *PostgresRepository) CallStaff(ctx context.Context, hash string) (*Bill, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	table, err := resolve(ctx, tx, hash, "")
	if err != nil {
		return nil, err
	}
	var billID string
	var first bool
	err = tx.QueryRow(ctx, `
 UPDATE bills SET payment_requested_at = COALESCE(payment_requested_at, now())
 WHERE venue_id=$1 AND venue_table_id=$2 AND status IN ('open','payment_pending')
 RETURNING id::text, payment_requested_at = now()`, table.VenueID, table.ID).Scan(&billID, &first)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, appfault.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if first {
		if _, err = tx.Exec(ctx, `
  INSERT INTO audit_logs (venue_id, action, entity_type, entity_id)
  VALUES ($1, 'bill.payment_requested', 'bill', $2)`, table.VenueID, billID); err != nil {
			return nil, err
		}
	}
	bill, err := readBill(ctx, tx, `WHERE b.id=$1`, billID)
	if err != nil {
		return nil, err
	}
	return bill, tx.Commit(ctx)
}
