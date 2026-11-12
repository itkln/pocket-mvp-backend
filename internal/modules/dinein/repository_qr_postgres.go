package dinein

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/tablesync"
)

func (r *PostgresRepository) IssueQR(ctx context.Context, venueID, identifier string, rotate bool, factory TokenFactory) (QRCode, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return QRCode{}, err
	}
	defer tx.Rollback(ctx)
	if err = tablesync.Table(ctx, tx, venueID, identifier); err != nil {
		return QRCode{}, err
	}
	var qr QRCode
	err = tx.QueryRow(ctx, `
 SELECT t.id::text, t.venue_id::text, t.identifier, t.qr_version, v.currency, v.name,
   COALESCE(t.qr_token_encrypted, ''), COALESCE(v.settings->>'dine_in', 'true') = 'true'
 FROM venue_tables t JOIN venues v ON v.id = t.venue_id
 WHERE t.venue_id = $1 AND t.identifier = $2 AND t.deleted_at IS NULL AND t.status <> 'disabled'
 FOR UPDATE OF t`, venueID, identifier).Scan(
		&qr.ID, &qr.VenueID, &qr.Identifier, &qr.Version, &qr.Currency,
		&qr.VenueName, &qr.Encrypted, &qr.OrderingEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return QRCode{}, appfault.ErrNotFound
	}
	if err != nil {
		return QRCode{}, err
	}
	if qr.Encrypted == "" || rotate {
		encrypted, hash, err := factory()
		if err != nil {
			return QRCode{}, err
		}
		if rotate {
			qr.Version++
		}
		qr.Encrypted = encrypted
		_, err = tx.Exec(ctx, `
  UPDATE venue_tables SET qr_token_encrypted = $2, qr_token_hash = $3, qr_version = $4
  WHERE id = $1`, qr.ID, encrypted, hash, qr.Version)
		if err != nil {
			return QRCode{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return QRCode{}, err
	}
	return qr, nil
}

const tableQuery = `
 SELECT t.id::text, t.venue_id::text, t.identifier, t.qr_version, v.currency, v.name,
   COALESCE(v.settings->>'dine_in', 'true') = 'true'
 FROM venue_tables t JOIN venues v ON v.id = t.venue_id
 WHERE t.qr_token_hash = $1 AND t.qr_token_encrypted IS NOT NULL
   AND t.deleted_at IS NULL AND t.status <> 'disabled'
   AND v.status = 'active' AND v.deleted_at IS NULL
   AND (NOT (v.settings ? 'floor_plan') OR EXISTS (
     SELECT 1 FROM jsonb_array_elements(v.settings->'floor_plan') f,
       jsonb_array_elements(COALESCE(f->'tables', '[]'::jsonb)) ft
     WHERE ft->>'id' = t.identifier
   ))`

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func resolve(ctx context.Context, q queryer, hash, suffix string) (Table, error) {
	var table Table
	err := q.QueryRow(ctx, tableQuery+suffix, hash).Scan(
		&table.ID, &table.VenueID, &table.Identifier, &table.Version,
		&table.Currency, &table.VenueName, &table.OrderingEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return Table{}, ErrQRInvalid
	}
	return table, err
}

func (r *PostgresRepository) Resolve(ctx context.Context, hash string) (Table, error) {
	return resolve(ctx, r.db, hash, "")
}
