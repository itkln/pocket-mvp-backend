package reservations

import "context"

func (r *PostgresRepository) ListCustomer(ctx context.Context, customerID string) ([]CustomerReservation, error) {
	rows, err := r.db.Query(ctx, `SELECT r.id::text, r.reservation_code, v.id::text, v.name,
		v.address_line1, v.city, v.timezone, r.starts_at, r.ends_at, r.guest_count, r.status,
		r.guest_name, COALESCE(r.guest_phone, ''), COALESCE(r.guest_email, ''), COALESCE(r.notes, ''),
		COALESCE((SELECT sum(unit_price_minor * quantity) FROM reservation_preorder_items WHERE reservation_id = r.id), 0),
		COALESCE((SELECT order_number FROM orders WHERE reservation_id = r.id AND channel = 'preorder' ORDER BY created_at LIMIT 1), 0),
		COALESCE((SELECT currency FROM orders WHERE reservation_id = r.id AND channel = 'preorder' ORDER BY created_at LIMIT 1), v.currency)
		FROM reservations r JOIN venues v ON v.id = r.venue_id
		WHERE r.customer_user_id = $1 AND r.status <> 'pending' AND r.hold_expires_at IS NULL
		ORDER BY r.starts_at DESC, r.id`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []CustomerReservation{}
	ids := []string{}
	byID := make(map[string]int)
	for rows.Next() {
		var reservation CustomerReservation
		if err := rows.Scan(&reservation.ID, &reservation.Code, &reservation.VenueID, &reservation.VenueName,
			&reservation.VenueAddress, &reservation.VenueCity, &reservation.Timezone, &reservation.StartsAt,
			&reservation.EndsAt, &reservation.GuestCount, &reservation.Status, &reservation.GuestName,
			&reservation.GuestPhone, &reservation.GuestEmail, &reservation.Notes, &reservation.PreorderTotalMinor,
			&reservation.PreorderOrderNumber, &reservation.Currency); err != nil {
			return nil, err
		}
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
	items, err := r.db.Query(ctx, `SELECT reservation_id::text, item_name, quantity, unit_price_minor
		FROM reservation_preorder_items WHERE reservation_id::text = ANY($1::text[])
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
