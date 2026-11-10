package dinein

import (
	"context"
	"errors"
	"fmt"

	"pocket-mvp-backend/internal/lifecycle"
	"pocket-mvp-backend/internal/money"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/gratuity"
	"pocket-mvp-backend/internal/orderstore"
)

// Create owns the entire transaction. Price snapshots, bill totals, pending tips
// and the audit record either commit together or are all rolled back.
func (r *PostgresRepository) Create(ctx context.Context, hash, requestHash string, input CreateInput) (Order, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)

	table, err := resolve(ctx, tx, hash, " FOR UPDATE OF t")
	if err != nil {
		return Order{}, err
	}
	replay, err := findReplay(ctx, tx, table, requestHash, input)
	if err != nil {
		return Order{}, err
	}
	if replay != nil {
		if err = tx.Commit(ctx); err != nil {
			return Order{}, err
		}
		return *replay, nil
	}
	if !table.OrderingEnabled {
		return Order{}, ErrBillLocked
	}

	items, subtotal, tip, err := priceItems(ctx, tx, table, input)
	if err != nil {
		return Order{}, err
	}
	billID, err := lockOpenBill(ctx, tx, table)
	if err != nil {
		return Order{}, err
	}
	orderID, err := insertOrder(ctx, tx, table, billID, requestHash, subtotal, input)
	if err != nil {
		return Order{}, err
	}
	if err = orderstore.InsertItems(ctx, tx, orderID, items); err != nil {
		return Order{}, err
	}
	if err = orderstore.ApplyOrderTip(ctx, tx, table.VenueID, orderID, table.Currency, tip); err != nil {
		return Order{}, err
	}
	if err = appendToBill(ctx, tx, billID, subtotal, tip); err != nil {
		return Order{}, err
	}
	if _, err = tx.Exec(ctx, `
 INSERT INTO audit_logs (venue_id, actor_user_id, action, entity_type, entity_id)
 VALUES ($1, NULLIF($2, '')::uuid, 'order.placed_from_table', 'order', $3)`, table.VenueID, input.CustomerUserID, orderID); err != nil {
		return Order{}, err
	}

	order, err := loadOrder(ctx, tx, table, input.IdempotencyKey)
	if err != nil {
		return Order{}, fmt.Errorf("read created order: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return order, nil
}

// A matching retry is returned before checking current menu prices or bill
// state. It must not create another order or charge the bill a second time.
func findReplay(ctx context.Context, tx pgx.Tx, table Table, requestHash string, input CreateInput) (*Order, error) {
	var oldHash, oldTable, oldGuestHash string
	err := tx.QueryRow(ctx, `
 SELECT COALESCE(request_hash, ''), COALESCE(venue_table_id::text, ''), COALESCE(guest_history_hash, '')
 FROM orders
 WHERE venue_id = $1 AND idempotency_key = $2
 FOR UPDATE`, table.VenueID, input.IdempotencyKey).Scan(&oldHash, &oldTable, &oldGuestHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	legacy := oldGuestHash == "" && oldHash == input.LegacyRequestHash
	if (oldHash != requestHash && !legacy) || oldTable != table.ID {
		return nil, appfault.ErrConflict
	}
	if legacy && input.GuestHash != "" {
		if _, err = tx.Exec(ctx, `
  UPDATE orders SET guest_history_hash = $2
  WHERE venue_id = $1 AND idempotency_key = $3 AND customer_user_id IS NULL`, table.VenueID, input.GuestHash, input.IdempotencyKey); err != nil {
			return nil, err
		}
	}
	order, err := loadOrder(ctx, tx, table, input.IdempotencyKey)
	return &order, err
}

func priceItems(ctx context.Context, tx pgx.Tx, table Table, input CreateInput) ([]orderstore.ItemSnapshot, money.Minor, money.Minor, error) {
	ids := make([]string, len(input.Items))
	for i, item := range input.Items {
		ids[i] = item.MenuItemID
	}
	menu, err := orderstore.LockMenuItems(ctx, tx, table.VenueID, ids, false)
	if err != nil {
		return nil, 0, 0, err
	}
	items := make([]orderstore.ItemSnapshot, 0, len(input.Items))
	subtotal := money.Minor(0)
	// Validate in request order, even though the rows were locked in UUID order.
	for _, item := range input.Items {
		saved, ok := orderstore.MenuItemByID(menu, item.MenuItemID)
		if !ok || saved.Currency != table.Currency {
			return nil, 0, 0, ErrItemUnavailable
		}
		if saved.PriceMinor != item.ExpectedPriceMinor {
			return nil, 0, 0, ErrPriceChanged
		}
		subtotal += saved.PriceMinor.Times(item.Quantity)
		items = append(items, orderstore.ItemSnapshot{MenuItemID: item.MenuItemID, Name: saved.Name, PriceMinor: saved.PriceMinor, Quantity: item.Quantity})
	}
	if subtotal > gratuity.MaxTotal {
		return nil, 0, 0, appfault.ErrInvalidInput
	}
	tip, err := gratuity.Calculate(subtotal, input.Tip)
	if err != nil {
		return nil, 0, 0, err
	}
	if input.ExpectedTotalMinor != nil && *input.ExpectedTotalMinor != subtotal+tip {
		return nil, 0, 0, ErrPriceChanged
	}
	return items, subtotal, tip, nil
}

func lockOpenBill(ctx context.Context, tx pgx.Tx, table Table) (string, error) {
	var billID, status string
	err := tx.QueryRow(ctx, `
 SELECT id::text, status FROM bills
 WHERE venue_id = $1 AND venue_table_id = $2 AND status IN ('open', 'payment_pending')
 FOR UPDATE`, table.VenueID, table.ID).Scan(&billID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
  INSERT INTO bills (venue_id, venue_table_id, currency)
  VALUES ($1, $2, $3) RETURNING id::text`, table.VenueID, table.ID, table.Currency).Scan(&billID)
		return billID, err
	}
	if err != nil {
		return "", err
	}
	var payment bool
	err = tx.QueryRow(ctx, `
 SELECT EXISTS (
   SELECT 1 FROM payments WHERE bill_id = $1 AND status NOT IN ('failed', 'cancelled')
 )`, billID).Scan(&payment)
	if err != nil {
		return "", err
	}
	if status != lifecycle.BillOpen || payment {
		return "", ErrBillLocked
	}
	return billID, nil
}

func insertOrder(ctx context.Context, tx pgx.Tx, table Table, billID, requestHash string, subtotal money.Minor, input CreateInput) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
 INSERT INTO orders (
   venue_id, bill_id, venue_table_id, customer_user_id, channel, status,
   currency, subtotal_minor, total_minor, notes, idempotency_key, request_hash,
   placed_at, guest_history_hash, table_identifier_snapshot, table_qr_version
 ) VALUES (
   $1, $2, $3, NULLIF($4, '')::uuid, 'dine_in', 'new', $5, $6, $6,
   $7, $8, $9, now(), NULLIF($10, ''), $11, $12
 )
 ON CONFLICT (venue_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
 RETURNING id::text`, table.VenueID, billID, table.ID, input.CustomerUserID, table.Currency, subtotal,
		input.Notes, input.IdempotencyKey, requestHash, input.GuestHash, table.Identifier, table.Version).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", appfault.ErrConflict
	}
	return id, err
}

func appendToBill(ctx context.Context, tx pgx.Tx, billID string, subtotal, tip money.Minor) error {
	tag, err := tx.Exec(ctx, `
 UPDATE bills SET
   subtotal_minor = subtotal_minor + $2,
   tip_minor = tip_minor + $3,
   total_minor = total_minor + $2 + $3
 WHERE id = $1 AND total_minor::bigint + $2::bigint + $3::bigint <= $4`, billID, subtotal, tip, gratuity.MaxTotal)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return appfault.ErrInvalidInput
	}
	return nil
}
