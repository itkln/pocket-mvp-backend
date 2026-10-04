package ordering

import "context"

func (r *PostgresRepository) ListCustomerPreorders(ctx context.Context, customerID string) ([]Order, error) {
	rows, err := r.db.Query(ctx, `WITH recent AS (
		SELECT id, order_number, venue_id, reservation_id, guest_name, total_minor, currency, status, created_at
		FROM orders
		WHERE customer_user_id = $1 AND channel = 'preorder' AND reservation_id IS NOT NULL
		ORDER BY created_at DESC
		LIMIT 100
	)
	SELECT o.id::text, o.order_number, v.name, COALESCE(o.guest_name, ''), o.total_minor,
		o.currency, o.status, o.created_at, r.reservation_code, r.starts_at,
		COALESCE(oi.item_name, ''), COALESCE(oi.quantity, 0), COALESCE(oi.unit_price_minor, 0)
	FROM recent o
	JOIN reservations r ON r.id = o.reservation_id AND r.status = 'confirmed'
	JOIN venues v ON v.id = o.venue_id
	LEFT JOIN order_items oi ON oi.order_id = o.id
	ORDER BY o.created_at DESC, oi.created_at, oi.id`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := []Order{}
	byID := make(map[string]int)
	for rows.Next() {
		var order Order
		var item OrderItem
		if err := rows.Scan(&order.ID, &order.Number, &order.VenueName, &order.GuestName,
			&order.TotalMinor, &order.Currency, &order.Status, &order.CreatedAt,
			&order.ReservationCode, &order.ReservationStartsAt,
			&item.Name, &item.Quantity, &item.UnitPriceMinor); err != nil {
			return nil, err
		}
		if index, found := byID[order.ID]; found {
			if item.Name != "" {
				orders[index].Items = append(orders[index].Items, item)
			}
			continue
		}
		order.Channel = "preorder"
		if item.Name != "" {
			order.Items = []OrderItem{item}
		}
		byID[order.ID] = len(orders)
		orders = append(orders, order)
	}
	return orders, rows.Err()
}
