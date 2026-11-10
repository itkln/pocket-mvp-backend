package ordering

import (
	"context"

	"pocket-mvp-backend/internal/security"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db        *pgxpool.Pool
	protector *security.DataProtector
}

func NewPostgresRepository(db *pgxpool.Pool, protectors ...*security.DataProtector) *PostgresRepository {
	r := &PostgresRepository{db: db}
	if len(protectors) > 0 {
		r.protector = protectors[0]
	}
	return r
}

func (r *PostgresRepository) List(ctx context.Context, venueID string) ([]Order, error) {
	rows, err := r.db.Query(ctx, `
		SELECT o.id::text, o.order_number, o.channel,
		       CASE
		         WHEN t.identifier IS NOT NULL THEN 'Стол ' || t.identifier
		         ELSE 'Онлайн'
		       END,
		       COALESCE(o.guest_name, ''), o.total_minor, o.tip_minor, o.currency, o.status,
		       COALESCE(o.notes, ''), o.created_at,
		       COALESCE(r.reservation_code, ''), r.starts_at, COALESCE(r.guest_phone, ''), COALESCE(r.guest_count, 0)
		FROM orders o
		LEFT JOIN venue_tables t ON t.id = o.venue_table_id
		LEFT JOIN reservations r ON r.id = o.reservation_id
		WHERE o.venue_id = $1
		ORDER BY o.created_at DESC, o.id DESC
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
	if err := r.loadPageItems(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}

type orderScanner interface {
	Scan(...any) error
}

func scanOrder(row orderScanner) (Order, error) {
	var order Order
	err := row.Scan(
		&order.ID, &order.Number, &order.Channel, &order.Source, &order.GuestName,
		&order.TotalMinor, &order.TipMinor, &order.Currency, &order.Status, &order.Notes, &order.CreatedAt,
		&order.ReservationCode, &order.ReservationStartsAt, &order.GuestPhone, &order.GuestCount,
	)
	return order, err
}
