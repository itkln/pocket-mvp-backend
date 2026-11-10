package reservations

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/gratuity"
)

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

type rowScanner interface{ Scan(...any) error }

func scanByKey(row rowScanner) (Result, string, bool, error) {
	var result Result
	var hash string
	err := row.Scan(&result.ID, &result.Code, &result.StartsAt, &result.GuestCount, &result.PreorderTotalMinor, &result.PreorderSubtotalMinor, &result.PreorderTipMinor, &result.PreorderOrderNumber, &result.Currency, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, "", false, nil
	}
	return result, hash, err == nil, err
}

const byKeyQuery = `
 SELECT r.id::text, r.reservation_code, r.starts_at, r.guest_count,
   COALESCE(preorder.total_minor, 0),
   COALESCE((SELECT sum(unit_price_minor * quantity) FROM reservation_preorder_items WHERE reservation_id = r.id), 0),
   COALESCE(preorder.tip_minor, 0), COALESCE(preorder.order_number, 0),
   v.currency, r.request_hash
 FROM reservations r
 JOIN venues v ON v.id = r.venue_id
 LEFT JOIN LATERAL (
   SELECT total_minor, tip_minor, order_number FROM orders
   WHERE reservation_id = r.id AND channel = 'preorder'
   ORDER BY created_at, id LIMIT 1
 ) preorder ON true
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
	snapshots, total, err := pricePreorder(ctx, tx, venueID, currency, input)
	if err != nil {
		return Result{}, err
	}
	tipMinor, err := gratuity.Calculate(total, input.Tip)
	if err != nil {
		return Result{}, err
	}
	if input.ExpectedTotalMinor != nil && *input.ExpectedTotalMinor != total+tipMinor {
		return Result{}, ErrPriceChanged
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
	result.PreorderSubtotalMinor, result.PreorderTipMinor, result.PreorderTotalMinor, result.Currency = total, tipMinor, total+tipMinor, currency
	if err = insertPreorder(ctx, tx, venueID, slot.TableID, input, &result, snapshots); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Result{}, appfault.MapWriteError(err)
	}
	return result, nil
}
