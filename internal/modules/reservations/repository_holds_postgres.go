package reservations

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"pocket-mvp-backend/internal/appfault"
)

const holdTTL = 10 * time.Minute

func (r *PostgresRepository) Hold(ctx context.Context, venueID string, input CreateInput, hash string, slot Slot) (HoldResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return HoldResult{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, venueID+":"+input.IdempotencyKey); err != nil {
		return HoldResult{}, err
	}
	var existing HoldResult
	var oldHash, status string
	err = tx.QueryRow(ctx, `SELECT id::text, request_hash, status, COALESCE(hold_expires_at, 'epoch'::timestamptz) FROM reservations
		WHERE venue_id = $1 AND idempotency_key = $2`, venueID, input.IdempotencyKey).
		Scan(&existing.ID, &oldHash, &status, &existing.ExpiresAt)
	if err == nil {
		if oldHash != hash {
			return HoldResult{}, appfault.ErrConflict
		}
		if status == "confirmed" {
			return existing, nil
		}
		if status != "pending" {
			return HoldResult{}, ErrHoldExpired
		}
		if !existing.ExpiresAt.After(time.Now()) {
			if _, err = tx.Exec(ctx, `UPDATE reservations SET status = 'cancelled' WHERE id = $1`, existing.ID); err != nil {
				return HoldResult{}, err
			}
			if err = tx.Commit(ctx); err != nil {
				return HoldResult{}, err
			}
			return HoldResult{}, ErrHoldExpired
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return HoldResult{}, err
	}
	if slot.TableID == "" {
		return HoldResult{}, appfault.ErrConflict
	}
	// Expired pending rows must be cancelled before the exclusion constraint can release their slots.
	if _, err = tx.Exec(ctx, `UPDATE reservations SET status = 'cancelled'
		WHERE venue_id = $1 AND status = 'pending' AND hold_expires_at <= now()`, venueID); err != nil {
		return HoldResult{}, err
	}
	err = tx.QueryRow(ctx, `WITH new_id AS (SELECT gen_random_uuid() AS id)
		INSERT INTO reservations (id, venue_id, venue_table_id, customer_user_id, reservation_code,
			guest_name, guest_email, guest_phone, guest_count, starts_at, ends_at, status,
			notes, idempotency_key, request_hash, hold_expires_at)
		SELECT id, $1, $2, NULLIF($3, '')::uuid, upper(substr(replace(id::text, '-', ''), 1, 10)),
			$4, NULLIF($5, ''), NULLIF($6, ''), $7, $8, $9, 'pending', NULLIF($10, ''), $11, $12,
			now() + $13 * interval '1 second' FROM new_id
		RETURNING id::text, hold_expires_at`, venueID, slot.TableID, input.CustomerUserID,
		input.GuestName, input.GuestEmail, input.GuestPhone, input.Guests, slot.StartsAt,
		slot.EndsAt, input.Notes, input.IdempotencyKey, hash, int(holdTTL.Seconds())).
		Scan(&existing.ID, &existing.ExpiresAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
			return HoldResult{}, appfault.ErrConflict
		}
		return HoldResult{}, appfault.MapWriteError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return HoldResult{}, appfault.MapWriteError(err)
	}
	return existing, nil
}

func (r *PostgresRepository) Confirm(ctx context.Context, venueID, holdID string, input CreateInput, holdHash, confirmationHash string) (Result, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)
	var status, oldHoldHash, oldConfirmationHash, tableID string
	var expiresAt time.Time
	var result Result
	err = tx.QueryRow(ctx, `SELECT status, request_hash, COALESCE(confirmation_hash, ''), COALESCE(hold_expires_at, 'epoch'::timestamptz),
		venue_table_id::text, reservation_code, starts_at, guest_count
		FROM reservations WHERE venue_id = $1 AND id = $2 AND idempotency_key = $3 FOR UPDATE`,
		venueID, holdID, input.IdempotencyKey).
		Scan(&status, &oldHoldHash, &oldConfirmationHash, &expiresAt,
			&tableID, &result.Code, &result.StartsAt, &result.GuestCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, appfault.ErrNotFound
	}
	if err != nil {
		return Result{}, err
	}
	if oldHoldHash != holdHash {
		return Result{}, appfault.ErrConflict
	}
	if status == "confirmed" {
		if oldConfirmationHash != confirmationHash {
			return Result{}, appfault.ErrConflict
		}
		found, _, ok, err := scanByKey(tx.QueryRow(ctx, byKeyQuery, venueID, input.IdempotencyKey))
		if err != nil {
			return Result{}, err
		}
		if !ok {
			return Result{}, appfault.ErrNotFound
		}
		return found, nil
	}
	if status != "pending" {
		return Result{}, ErrHoldExpired
	}
	if !expiresAt.After(time.Now()) {
		if _, err = tx.Exec(ctx, `UPDATE reservations SET status = 'cancelled' WHERE id = $1`, holdID); err != nil {
			return Result{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return Result{}, err
		}
		return Result{}, ErrHoldExpired
	}
	if !result.StartsAt.Equal(input.StartsAt) || result.GuestCount != input.Guests {
		return Result{}, appfault.ErrConflict
	}
	var preorderEnabled bool
	err = tx.QueryRow(ctx, `SELECT currency, COALESCE((settings ->> 'preorder_enabled')::boolean, false)
		FROM venues WHERE id = $1 AND status = 'active' AND deleted_at IS NULL
		AND COALESCE(settings ->> 'public_category', 'restaurants') = 'restaurants'`, venueID).
		Scan(&result.Currency, &preorderEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, appfault.ErrNotFound
	}
	if err != nil {
		return Result{}, err
	}
	if len(input.Items) > 0 && !preorderEnabled {
		return Result{}, ErrItemUnavailable
	}
	type snapshot struct {
		ID, Name        string
		Price, Quantity int
	}
	snapshots := make([]snapshot, 0, len(input.Items))
	for _, item := range input.Items {
		var name, currency string
		var price int
		err = tx.QueryRow(ctx, `SELECT i.name, i.price_minor, i.currency FROM menu_items i
			JOIN menu_categories c ON c.id = i.category_id AND c.venue_id = i.venue_id
			WHERE i.id = $1 AND i.venue_id = $2 AND i.is_available AND i.deleted_at IS NULL AND c.is_active AND c.preorder_enabled
			FOR SHARE OF i, c`, item.MenuItemID, venueID).Scan(&name, &price, &currency)
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, ErrItemUnavailable
		}
		if err != nil {
			return Result{}, err
		}
		if currency != result.Currency {
			return Result{}, appfault.ErrInvalidInput
		}
		if price != item.ExpectedPriceMinor {
			return Result{}, ErrPriceChanged
		}
		if price > 100000000 || result.PreorderTotalMinor > 100000000-price*item.Quantity {
			return Result{}, appfault.ErrInvalidInput
		}
		result.PreorderTotalMinor += price * item.Quantity
		snapshots = append(snapshots, snapshot{item.MenuItemID, name, price, item.Quantity})
	}
	if _, err = tx.Exec(ctx, `UPDATE reservations SET status = 'confirmed', hold_expires_at = NULL,
		confirmation_hash = $2, guest_name = $3, guest_phone = NULLIF($4, ''),
		guest_email = NULLIF($5, ''), notes = NULLIF($6, '') WHERE id = $1`, holdID, confirmationHash,
		input.GuestName, input.GuestPhone, input.GuestEmail, input.Notes); err != nil {
		return Result{}, appfault.MapWriteError(err)
	}
	result.ID = holdID
	if len(snapshots) > 0 {
		var orderID string
		err = tx.QueryRow(ctx, `INSERT INTO orders (venue_id, reservation_id, venue_table_id, customer_user_id, channel, status, guest_name, guest_email,
			currency, subtotal_minor, total_minor, notes, idempotency_key, placed_at)
			VALUES ($1, $2, $3, NULLIF($4, '')::uuid, 'preorder', 'new', $5, NULLIF($6, ''), $7, $8, $8, NULLIF($9, ''), $10, now()) RETURNING id::text, order_number`,
			venueID, holdID, tableID, input.CustomerUserID, input.GuestName, input.GuestEmail, result.Currency,
			result.PreorderTotalMinor, input.Notes, input.IdempotencyKey).Scan(&orderID, &result.PreorderOrderNumber)
		if err != nil {
			return Result{}, appfault.MapWriteError(err)
		}
		for _, item := range snapshots {
			if _, err = tx.Exec(ctx, `INSERT INTO reservation_preorder_items (reservation_id, menu_item_id, item_name, unit_price_minor, quantity)
				VALUES ($1, $2, $3, $4, $5)`, holdID, item.ID, item.Name, item.Price, item.Quantity); err != nil {
				return Result{}, err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO order_items (order_id, menu_item_id, item_name, unit_price_minor, quantity, total_minor)
				VALUES ($1, $2, $3, $4, $5, $6)`, orderID, item.ID, item.Name, item.Price, item.Quantity,
				item.Price*item.Quantity); err != nil {
				return Result{}, err
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Result{}, appfault.MapWriteError(err)
	}
	return result, nil
}

func (r *PostgresRepository) GetConfirmed(ctx context.Context, venueID, reservationID, key string) (Result, error) {
	result, _, found, err := scanByKey(r.db.QueryRow(ctx, byKeyQuery+` AND r.id = $3`, venueID, key, reservationID))
	if err != nil {
		return Result{}, err
	}
	if !found {
		return Result{}, appfault.ErrNotFound
	}
	return result, nil
}
