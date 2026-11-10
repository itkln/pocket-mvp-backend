package dinein

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"
)

func (r *PostgresRepository) History(ctx context.Context, query HistoryQuery) (HistoryPage, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return HistoryPage{}, err
	}
	defer tx.Rollback(ctx)
	// The existing random idempotency keys are capabilities held only by the guest.
	// Claim only unowned legacy orders, in this venue, and never another account/device.
	if len(query.LegacyKeys) > 0 {
		_, err = tx.Exec(ctx, `
        UPDATE orders SET
            customer_user_id = COALESCE(customer_user_id, NULLIF($2, '')::uuid),
            guest_history_hash = CASE WHEN $2 = '' THEN $3 ELSE guest_history_hash END
        WHERE venue_id = $1 AND channel = 'dine_in' AND idempotency_key = ANY($4::text[])
            AND customer_user_id IS NULL AND guest_history_hash IS NULL`, query.VenueID, query.CustomerUserID, query.GuestHash, query.LegacyKeys)
		if err != nil {
			return HistoryPage{}, err
		}
	}
	var beforeTime *time.Time
	beforeID := "00000000-0000-4000-8000-000000000000"
	if query.Before != nil {
		beforeTime = &query.Before.CreatedAt
		beforeID = query.Before.ID
	}
	rows, err := tx.Query(ctx, `SELECT `+orderProjection+`
    FROM orders o LEFT JOIN venue_tables t ON t.id = o.venue_table_id
    WHERE o.venue_id = $1 AND o.channel = 'dine_in'
        AND (
            ($2 <> '' AND o.customer_user_id = NULLIF($2, '')::uuid)
            OR ($2 = '' AND o.customer_user_id IS NULL AND o.guest_history_hash = $3)
        )
        AND ($4::timestamptz IS NULL OR (o.created_at, o.id) < ($4::timestamptz, $5::uuid))
    ORDER BY o.created_at DESC, o.id DESC LIMIT 31`, query.VenueID, query.CustomerUserID, query.GuestHash, beforeTime, beforeID)
	if err != nil {
		return HistoryPage{}, err
	}
	page := HistoryPage{Orders: []Order{}}
	for rows.Next() {
		order, err := scanOrder(rows, query.VenueID)
		if err != nil {
			rows.Close()
			return HistoryPage{}, err
		}
		page.Orders = append(page.Orders, order)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return HistoryPage{}, err
	}
	if len(page.Orders) > 30 {
		page.Orders = page.Orders[:30]
		last := page.Orders[29]
		raw, _ := json.Marshal(historyCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	if err = tx.Commit(ctx); err != nil {
		return HistoryPage{}, err
	}
	return page, nil
}
