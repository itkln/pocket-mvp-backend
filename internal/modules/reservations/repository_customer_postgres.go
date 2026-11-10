package reservations

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

type customerQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (r *PostgresRepository) ListCustomer(ctx context.Context, customerID string) ([]CustomerReservation, error) {
	return listCustomer(ctx, r.db, customerID, "")
}

func listCustomer(ctx context.Context, db customerQuerier, customerID, reservationID string) ([]CustomerReservation, error) {
	rows, err := db.Query(ctx, `
        SELECT r.id::text, r.reservation_code, v.id::text, v.name,
            v.address_line1, v.city, v.timezone, r.starts_at, r.ends_at, r.guest_count, r.status,
            r.guest_name, COALESCE(r.guest_phone, ''), COALESCE(r.guest_email, ''), COALESCE(r.notes, ''),
            COALESCE(preorder.total_minor, 0), COALESCE(preorder.tip_minor, 0),
            COALESCE(preorder.order_number, 0), COALESCE(preorder.currency, v.currency),
            COALESCE(preorder.status, ''),
            COALESCE(v.settings->'guest_cancellation', 'null'::jsonb),
            COALESCE((
                SELECT jsonb_agg(status) FROM orders
                WHERE reservation_id = r.id AND channel = 'preorder'
            ), '[]'::jsonb),
            EXISTS (
                SELECT 1 FROM orders o LEFT JOIN bills b ON b.id = o.bill_id
                WHERE o.reservation_id = r.id AND o.channel = 'preorder'
                    AND (
                        b.status IN ('paid', 'payment_pending') OR EXISTS (
                            SELECT 1 FROM payments p
                            WHERE (p.order_id = o.id OR p.bill_id = o.bill_id)
                                AND p.status NOT IN ('failed', 'cancelled')
                        )
                    )
            ),
            clock_timestamp()
        FROM reservations r
        JOIN venues v ON v.id = r.venue_id
        LEFT JOIN LATERAL (
            SELECT total_minor, tip_minor, order_number, currency, status
            FROM orders
            WHERE reservation_id = r.id AND channel = 'preorder'
            ORDER BY created_at, id LIMIT 1
        ) preorder ON true
        WHERE r.customer_user_id = $1 AND r.status <> 'pending' AND r.hold_expires_at IS NULL
            AND (NULLIF($2, '')::uuid IS NULL OR r.id = NULLIF($2, '')::uuid)
        ORDER BY r.starts_at DESC, r.id`, customerID, reservationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []CustomerReservation{}
	ids := []string{}
	byID := make(map[string]int)
	for rows.Next() {
		var reservation CustomerReservation
		var policy, statesJSON json.RawMessage
		var paymentStarted bool
		var now time.Time
		if err := rows.Scan(&reservation.ID, &reservation.Code, &reservation.VenueID, &reservation.VenueName,
			&reservation.VenueAddress, &reservation.VenueCity, &reservation.Timezone, &reservation.StartsAt,
			&reservation.EndsAt, &reservation.GuestCount, &reservation.Status, &reservation.GuestName,
			&reservation.GuestPhone, &reservation.GuestEmail, &reservation.Notes, &reservation.PreorderTotalMinor, &reservation.PreorderTipMinor,
			&reservation.PreorderOrderNumber, &reservation.Currency, &reservation.PreorderStatus,
			&policy, &statesJSON, &paymentStarted, &now); err != nil {
			return nil, err
		}
		var states []string
		if err := json.Unmarshal(statesJSON, &states); err != nil {
			return nil, err
		}
		reservation.Cancellation = cancellationEligibility(reservation.Status, reservation.StartsAt, now, policy, states, paymentStarted)
		reservation.PreorderItems = []PreorderSnapshot{}
		byID[reservation.ID] = len(result)
		ids = append(ids, reservation.ID)
		result = append(result, reservation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if len(ids) == 0 {
		return result, nil
	}
	items, err := db.Query(ctx, `SELECT reservation_id::text, item_name, quantity, unit_price_minor
		FROM reservation_preorder_items WHERE reservation_id = ANY($1::uuid[])
		ORDER BY created_at, id`, ids)
	if err != nil {
		return nil, err
	}
	defer items.Close()
	for items.Next() {
		var id string
		var item PreorderSnapshot
		if err := items.Scan(&id, &item.Name, &item.Quantity, &item.UnitPriceMinor); err != nil {
			return nil, err
		}
		index := byID[id]
		result[index].PreorderItems = append(result[index].PreorderItems, item)
	}
	return result, items.Err()
}
