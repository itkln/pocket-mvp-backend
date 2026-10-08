package ordering

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/appfault"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) List(ctx context.Context, venueID string) ([]Order, error) {
	rows, err := r.db.Query(ctx, `
		SELECT o.id::text, o.order_number, o.channel,
		       CASE
		         WHEN t.identifier IS NOT NULL THEN 'Стол ' || t.identifier
		         WHEN o.channel = 'pickup' THEN 'Самовывоз'
		         ELSE 'Онлайн'
		       END,
		       COALESCE(o.guest_name, ''), o.total_minor, o.currency, o.status,
		       COALESCE(o.notes, ''), o.created_at,
		       COALESCE(r.reservation_code, ''), r.starts_at, COALESCE(r.guest_phone, ''), COALESCE(r.guest_count, 0)
		FROM orders o
		LEFT JOIN venue_tables t ON t.id = o.venue_table_id
		LEFT JOIN reservations r ON r.id = o.reservation_id
		WHERE o.venue_id = $1
		ORDER BY o.created_at DESC
		LIMIT 200`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Order{}
	for rows.Next() {
		order, scanErr := scanOrder(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, order)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	byID := make(map[string]int, len(result))
	for index := range result {
		byID[result[index].ID] = index
	}
	items, err := r.db.Query(ctx, `SELECT oi.order_id::text, oi.item_name, oi.quantity, oi.unit_price_minor
		FROM order_items oi JOIN orders o ON o.id = oi.order_id
		WHERE o.venue_id = $1 AND o.id IN (SELECT id FROM orders WHERE venue_id = $1 ORDER BY created_at DESC LIMIT 200)
		ORDER BY oi.created_at, oi.id`, venueID)
	if err != nil {
		return nil, err
	}
	defer items.Close()
	for items.Next() {
		var id string
		var item OrderItem
		if err := items.Scan(&id, &item.Name, &item.Quantity, &item.UnitPriceMinor); err != nil {
			return nil, err
		}
		if index, ok := byID[id]; ok {
			result[index].Items = append(result[index].Items, item)
		}
	}
	return result, items.Err()
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, venueID, orderID, status string) (Order, error) {
	order, err := scanOrder(r.db.QueryRow(ctx, `
		WITH updated AS (
			UPDATE orders SET status = $3,
				completed_at = CASE WHEN $3 IN ('completed', 'cancelled') THEN now() ELSE completed_at END
			WHERE id = $2 AND venue_id = $1 RETURNING *
		)
		SELECT o.id::text, o.order_number, o.channel,
			CASE WHEN t.identifier IS NOT NULL THEN 'Стол ' || t.identifier
				WHEN o.channel = 'pickup' THEN 'Самовывоз' ELSE 'Онлайн' END,
			COALESCE(o.guest_name, ''), o.total_minor, o.currency, o.status,
			COALESCE(o.notes, ''), o.created_at,
			COALESCE(r.reservation_code, ''), r.starts_at, COALESCE(r.guest_phone, ''), COALESCE(r.guest_count, 0)
		FROM updated o LEFT JOIN venue_tables t ON t.id = o.venue_table_id
		LEFT JOIN reservations r ON r.id = o.reservation_id`, venueID, orderID, status))
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, appfault.ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	items, err := r.db.Query(ctx, `SELECT item_name, quantity, unit_price_minor FROM order_items
		WHERE order_id = $1 ORDER BY created_at, id`, order.ID)
	if err != nil {
		return Order{}, err
	}
	defer items.Close()
	for items.Next() {
		var item OrderItem
		if err := items.Scan(&item.Name, &item.Quantity, &item.UnitPriceMinor); err != nil {
			return Order{}, err
		}
		order.Items = append(order.Items, item)
	}
	return order, items.Err()
}

type orderScanner interface {
	Scan(...any) error
}

func scanOrder(row orderScanner) (Order, error) {
	var order Order
	err := row.Scan(
		&order.ID, &order.Number, &order.Channel, &order.Source, &order.GuestName,
		&order.TotalMinor, &order.Currency, &order.Status, &order.Notes, &order.CreatedAt,
		&order.ReservationCode, &order.ReservationStartsAt, &order.GuestPhone, &order.GuestCount,
	)
	return order, err
}
