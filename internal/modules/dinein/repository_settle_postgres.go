package dinein

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/lifecycle"
	"pocket-mvp-backend/internal/money"
)

// staffProvider marks payments taken in person and confirmed in Vendor.
const staffProvider = "staff"

func (r *PostgresRepository) OpenBills(ctx context.Context, venueID string) ([]Bill, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	bills, err := readBills(ctx, tx, `
 WHERE b.venue_id=$1 AND b.venue_table_id IS NOT NULL AND b.status IN ('open','payment_pending')
 ORDER BY b.payment_requested_at IS NULL, b.payment_requested_at, b.opened_at, b.id`, venueID)
	if err != nil {
		return nil, err
	}
	return bills, tx.Commit(ctx)
}

// Settle locks the bill row first. Order creation locks the same row before adding to a bill, so
// an order either lands before the settlement (and changes the amount the staff checked) or after
// it (and opens a new visit): an order is never added to a bill that is being closed.
func (r *PostgresRepository) Settle(ctx context.Context, venueID, billID, userID string, input SettleInput) (Settlement, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Settlement{}, err
	}
	defer tx.Rollback(ctx)

	var status, identifier, currency string
	var closedAt *time.Time
	err = tx.QueryRow(ctx, `
 SELECT b.status, COALESCE(t.identifier,''), b.currency, b.closed_at
 FROM bills b LEFT JOIN venue_tables t ON t.id=b.venue_table_id
 WHERE b.id=$1 AND b.venue_id=$2 AND b.venue_table_id IS NOT NULL
 FOR UPDATE OF b`, billID, venueID).Scan(&status, &identifier, &currency, &closedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settlement{}, appfault.ErrNotFound
	}
	if err != nil {
		return Settlement{}, err
	}
	settled := Settlement{BillID: billID, TableIdentifier: identifier, Status: status, Currency: currency}

	switch status {
	case lifecycle.BillPaid, lifecycle.BillVoid:
		return replaySettlement(ctx, tx, settled, closedAt)
	case lifecycle.BillPaymentPending:
		return Settlement{}, ErrBillLocked
	}
	var onlinePayment bool
	if err = tx.QueryRow(ctx, `
 SELECT EXISTS (SELECT 1 FROM payments WHERE bill_id=$1 AND status NOT IN ('failed','cancelled'))`,
		billID).Scan(&onlinePayment); err != nil {
		return Settlement{}, err
	}
	if onlinePayment {
		return Settlement{}, ErrBillLocked
	}

	bill, err := readBill(ctx, tx, `WHERE b.id=$1`, billID)
	if err != nil {
		return Settlement{}, err
	}
	if bill == nil {
		return Settlement{}, appfault.ErrNotFound
	}
	if bill.TotalMinor != input.ExpectedTotalMinor {
		return Settlement{}, ErrBillChanged
	}

	// A visit whose every dish was cancelled has nothing to pay: it is closed as void.
	settled.Status, settled.AmountMinor = lifecycle.BillPaid, bill.TotalMinor
	if bill.TotalMinor == 0 {
		settled.Status = lifecycle.BillVoid
	} else {
		settled.Method, settled.SettledByUserID = input.Method, userID
		if _, err = tx.Exec(ctx, `
  INSERT INTO payments (venue_id, bill_id, provider, provider_payment_id, status, amount_minor, currency,
    method, recorded_by_user_id, paid_at)
  VALUES ($1, $2::uuid, $3, $8, 'paid', $4, $5, $6, $7, now())`,
			venueID, billID, staffProvider, bill.TotalMinor, currency, input.Method, userID, billID); err != nil {
			return Settlement{}, err
		}
	}
	// The cached totals are brought in line with what was actually paid.
	if err = tx.QueryRow(ctx, `
 UPDATE bills SET status=$2, closed_at=now(), subtotal_minor=$3, tip_minor=$4, total_minor=$5
 WHERE id=$1 RETURNING closed_at`, billID, settled.Status, bill.SubtotalMinor, bill.TipMinor, bill.TotalMinor,
	).Scan(&settled.SettledAt); err != nil {
		return Settlement{}, err
	}
	if _, err = tx.Exec(ctx, `
 INSERT INTO audit_logs (venue_id, actor_user_id, action, entity_type, entity_id, changes)
 VALUES ($1, $2, 'bill.settled', 'bill', $3, jsonb_build_object('amount_minor', $4::int, 'method', $5::text, 'currency', $6::text))`,
		venueID, userID, billID, settled.AmountMinor, settled.Method, currency); err != nil {
		return Settlement{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Settlement{}, err
	}
	return settled, nil
}

// replaySettlement answers a repeated confirmation with the settlement already recorded. A visit
// that was paid online has no in-person settlement to repeat.
func replaySettlement(ctx context.Context, tx pgx.Tx, settled Settlement, closedAt *time.Time) (Settlement, error) {
	if closedAt != nil {
		settled.SettledAt = *closedAt
	}
	if settled.Status == lifecycle.BillVoid {
		var settledVoid bool
		if err := tx.QueryRow(ctx, `
  SELECT EXISTS (SELECT 1 FROM audit_logs WHERE entity_id=$1 AND action='bill.settled')`, settled.BillID).Scan(&settledVoid); err != nil {
			return Settlement{}, err
		}
		if !settledVoid {
			return Settlement{}, ErrBillClosed
		}
		return settled, tx.Commit(ctx)
	}
	var amount money.Minor
	var method, recordedBy string
	var paidAt time.Time
	err := tx.QueryRow(ctx, `
 SELECT amount_minor, COALESCE(method,''), COALESCE(recorded_by_user_id::text,''), COALESCE(paid_at, created_at)
 FROM payments WHERE bill_id=$1 AND provider=$2 AND status='paid'`, settled.BillID, staffProvider).Scan(
		&amount, &method, &recordedBy, &paidAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settlement{}, ErrBillClosed
	}
	if err != nil {
		return Settlement{}, err
	}
	settled.AmountMinor, settled.Method, settled.SettledByUserID, settled.SettledAt = amount, method, recordedBy, paidAt
	return settled, tx.Commit(ctx)
}
