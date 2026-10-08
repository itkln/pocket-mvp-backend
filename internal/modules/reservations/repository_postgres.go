package reservations

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"pocket-mvp-backend/internal/appfault"
)

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

type rowScanner interface{ Scan(...any) error }

func scanByKey(row rowScanner) (Result, string, bool, error) {
	var result Result
	var hash string
	err := row.Scan(&result.ID, &result.Code, &result.StartsAt, &result.GuestCount, &result.PreorderTotalMinor, &result.PreorderOrderNumber, &result.Currency, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, "", false, nil
	}
	return result, hash, err == nil, err
}

const byKeyQuery = `SELECT r.id::text, r.reservation_code, r.starts_at, r.guest_count,
	COALESCE((SELECT sum(unit_price_minor * quantity) FROM reservation_preorder_items WHERE reservation_id = r.id), 0),
	COALESCE((SELECT order_number FROM orders WHERE reservation_id = r.id AND channel = 'preorder' LIMIT 1), 0),
	v.currency, r.request_hash
	FROM reservations r JOIN venues v ON v.id = r.venue_id
	WHERE r.venue_id = $1 AND r.idempotency_key = $2 AND r.status = 'confirmed'`

func (r *PostgresRepository) FindByKey(ctx context.Context, venueID, key string) (Result, string, bool, error) {
	return scanByKey(r.db.QueryRow(ctx, byKeyQuery, venueID, key))
}

func (r *PostgresRepository) Create(ctx context.Context, venueID string, input CreateInput, requestHash string, slot Slot) (Result, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, venueID+":"+input.IdempotencyKey); err != nil {
		return Result{}, err
	}
	if existing, oldHash, found, err := scanByKey(tx.QueryRow(ctx, byKeyQuery, venueID, input.IdempotencyKey)); err != nil {
		return Result{}, err
	} else if found {
		if oldHash != requestHash {
			return Result{}, appfault.ErrConflict
		}
		return existing, nil
	}
	var currency string
	var preorderEnabled bool
	err = tx.QueryRow(ctx, `SELECT currency, COALESCE((settings ->> 'preorder_enabled')::boolean, false)
		FROM venues WHERE id = $1 AND status = 'active' AND deleted_at IS NULL
		AND COALESCE(settings ->> 'public_category', 'restaurants') = 'restaurants'`, venueID).Scan(&currency, &preorderEnabled)
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
	total := 0
	for _, item := range input.Items {
		var name, itemCurrency string
		var price int
		err = tx.QueryRow(ctx, `SELECT i.name, i.price_minor, i.currency FROM menu_items i
			JOIN menu_categories c ON c.id = i.category_id AND c.venue_id = i.venue_id
			WHERE i.id = $1 AND i.venue_id = $2 AND i.is_available AND i.deleted_at IS NULL AND c.is_active AND c.preorder_enabled
			FOR SHARE OF i, c`, item.MenuItemID, venueID).Scan(&name, &price, &itemCurrency)
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, ErrItemUnavailable
		}
		if err != nil {
			return Result{}, err
		}
		if itemCurrency != currency {
			return Result{}, appfault.ErrInvalidInput
		}
		if price != item.ExpectedPriceMinor {
			return Result{}, ErrPriceChanged
		}
		if price > 100000000 || total > 100000000-price*item.Quantity {
			return Result{}, appfault.ErrInvalidInput
		}
		total += price * item.Quantity
		snapshots = append(snapshots, snapshot{item.MenuItemID, name, price, item.Quantity})
	}
	var result Result
	err = tx.QueryRow(ctx, `WITH new_id AS (SELECT gen_random_uuid() AS id)
		INSERT INTO reservations (id, venue_id, venue_table_id, customer_user_id, reservation_code, guest_name, guest_email, guest_phone,
			guest_count, starts_at, ends_at, status, notes, idempotency_key, request_hash)
		SELECT id, $1, $2, NULLIF($3, '')::uuid, upper(substr(replace(id::text, '-', ''), 1, 10)), $4, NULLIF($5, ''), NULLIF($6, ''),
			$7, $8, $9, 'confirmed', NULLIF($10, ''), $11, $12 FROM new_id
		RETURNING id::text, reservation_code, starts_at, guest_count`,
		venueID, slot.TableID, input.CustomerUserID, input.GuestName, input.GuestEmail, input.GuestPhone,
		input.Guests, slot.StartsAt, slot.EndsAt, input.Notes, input.IdempotencyKey, requestHash).
		Scan(&result.ID, &result.Code, &result.StartsAt, &result.GuestCount)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
			return Result{}, appfault.ErrConflict
		}
		return Result{}, appfault.MapWriteError(err)
	}
	result.PreorderTotalMinor, result.Currency = total, currency
	if len(snapshots) > 0 {
		var orderID string
		err = tx.QueryRow(ctx, `INSERT INTO orders (venue_id, reservation_id, venue_table_id, customer_user_id, channel, status, guest_name, guest_email,
			currency, subtotal_minor, total_minor, notes, idempotency_key, placed_at)
			VALUES ($1, $2, $3, NULLIF($4, '')::uuid, 'preorder', 'new', $5, NULLIF($6, ''), $7, $8, $8, NULLIF($9, ''), $10, now()) RETURNING id::text, order_number`,
			venueID, result.ID, slot.TableID, input.CustomerUserID, input.GuestName, input.GuestEmail, currency, total, input.Notes, input.IdempotencyKey).
			Scan(&orderID, &result.PreorderOrderNumber)
		if err != nil {
			return Result{}, appfault.MapWriteError(err)
		}
		for _, item := range snapshots {
			_, err = tx.Exec(ctx, `INSERT INTO reservation_preorder_items (reservation_id, menu_item_id, item_name, unit_price_minor, quantity)
				VALUES ($1, $2, $3, $4, $5)`, result.ID, item.ID, item.Name, item.Price, item.Quantity)
			if err != nil {
				return Result{}, err
			}
			_, err = tx.Exec(ctx, `INSERT INTO order_items (order_id, menu_item_id, item_name, unit_price_minor, quantity, total_minor)
				VALUES ($1, $2, $3, $4, $5, $6)`, orderID, item.ID, item.Name, item.Price, item.Quantity, item.Price*item.Quantity)
			if err != nil {
				return Result{}, err
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Result{}, appfault.MapWriteError(err)
	}
	return result, nil
}
