package reservations

import (
	"context"

	"github.com/jackc/pgx/v5"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/orderstore"
)

const maxPreorderSubtotal = 100_000_000

// Both direct reservations and held-slot confirmation use the same locked menu
// snapshot. Preorders additionally require category.preorder_enabled.
func pricePreorder(ctx context.Context, tx pgx.Tx, venueID, currency string, input CreateInput) ([]orderstore.ItemSnapshot, int, error) {
	if len(input.Items) == 0 {
		return nil, 0, nil
	}
	ids := make([]string, len(input.Items))
	for i, item := range input.Items {
		ids[i] = item.MenuItemID
	}
	menu, err := orderstore.LockMenuItems(ctx, tx, venueID, ids, true)
	if err != nil {
		return nil, 0, err
	}
	snapshots := make([]orderstore.ItemSnapshot, 0, len(input.Items))
	total := int64(0)
	for _, item := range input.Items {
		saved, ok := orderstore.MenuItemByID(menu, item.MenuItemID)
		if !ok {
			return nil, 0, ErrItemUnavailable
		}
		if saved.Currency != currency {
			return nil, 0, appfault.ErrInvalidInput
		}
		if saved.PriceMinor != item.ExpectedPriceMinor {
			return nil, 0, ErrPriceChanged
		}
		total += int64(saved.PriceMinor) * int64(item.Quantity)
		if saved.PriceMinor > maxPreorderSubtotal || total > maxPreorderSubtotal {
			return nil, 0, appfault.ErrInvalidInput
		}
		snapshots = append(snapshots, orderstore.ItemSnapshot{MenuItemID: item.MenuItemID, Name: saved.Name, PriceMinor: saved.PriceMinor, Quantity: item.Quantity})
	}
	return snapshots, int(total), nil
}

func insertPreorder(ctx context.Context, tx pgx.Tx, venueID, tableID string, input CreateInput, result *Result, items []orderstore.ItemSnapshot) error {
	if len(items) == 0 {
		return nil
	}
	var orderID string
	err := tx.QueryRow(ctx, `
 INSERT INTO orders (
   venue_id, reservation_id, venue_table_id, customer_user_id, channel, status,
   guest_name, guest_email, currency, subtotal_minor, total_minor, notes, idempotency_key, placed_at
 ) VALUES (
   $1, $2, $3, NULLIF($4, '')::uuid, 'preorder', 'new', $5, NULLIF($6, ''),
   $7, $8, $8, NULLIF($9, ''), $10, now()
 ) RETURNING id::text, order_number`, venueID, result.ID, tableID, input.CustomerUserID,
		input.GuestName, input.GuestEmail, result.Currency, result.PreorderSubtotalMinor,
		input.Notes, input.IdempotencyKey).Scan(&orderID, &result.PreorderOrderNumber)
	if err != nil {
		return appfault.MapWriteError(err)
	}
	if err = orderstore.ApplyOrderTip(ctx, tx, venueID, orderID, result.Currency, result.PreorderTipMinor); err != nil {
		return err
	}
	if err = orderstore.InsertPreorderItems(ctx, tx, result.ID, items); err != nil {
		return err
	}
	return orderstore.InsertItems(ctx, tx, orderID, items)
}
