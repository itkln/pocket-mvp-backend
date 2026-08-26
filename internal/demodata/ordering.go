package demodata

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func seedOrders(ctx context.Context, tx pgx.Tx, venueID string) error {
	for index, order := range demoOrders(venueID) {
		_, err := tx.Exec(ctx, `
			INSERT INTO orders (
				id, venue_id, venue_table_id, assigned_waiter_staff_id, channel,
				status, guest_name, currency, subtotal_minor, total_minor, notes,
				placed_at, completed_at, created_at, updated_at
			)
			VALUES (
				$1, $2, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5,
				$6, $7, 'EUR', $8, $8, NULLIF($9, ''), $10::timestamptz,
				CASE WHEN $6 IN ('completed', 'cancelled') THEN $10::timestamptz + interval '35 minutes' ELSE NULL END,
				$10::timestamptz, $10::timestamptz
			)`,
			order.ID, venueID, order.TableID, order.StaffID, order.Channel, order.Status,
			order.GuestName, order.TotalMinor, order.Notes, order.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("seed order %s: %w", order.ID, err)
		}
		itemID, itemName := orderMenuItem(venueID, index)
		if _, err = tx.Exec(ctx, `
			INSERT INTO order_items (
				order_id, menu_item_id, item_name, unit_price_minor, quantity,
				total_minor, status, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, 1, $4, $5, $6, $6)`,
			order.ID, itemID, itemName, order.TotalMinor, orderItemStatus(order.Status), order.CreatedAt,
		); err != nil {
			return fmt.Errorf("seed items for order %s: %w", order.ID, err)
		}
	}
	return nil
}

func demoOrders(venueID string) []orderSeed {
	now := time.Now().UTC()
	if venueID == secondaryVenueID {
		return []orderSeed{
			{ID: "70000000-0000-4000-8000-000000000101", TableID: "61000000-0000-4000-8000-000000000102", StaffID: "50000000-0000-4000-8000-000000000102", Channel: "dine_in", Status: "preparing", GuestName: "Мария", TotalMinor: 810, CreatedAt: now.Add(-12 * time.Minute)},
			{ID: "70000000-0000-4000-8000-000000000102", Channel: "pickup", Status: "completed", GuestName: "Томаш", TotalMinor: 1210, CreatedAt: now.Add(-95 * time.Minute)},
			{ID: "70000000-0000-4000-8000-000000000103", Channel: "online", Status: "new", GuestName: "Елена", TotalMinor: 740, Notes: "Без сахара", CreatedAt: now.Add(-4 * time.Minute)},
		}
	}
	return []orderSeed{
		{ID: "70000000-0000-4000-8000-000000000001", TableID: "61000000-0000-4000-8000-000000000002", StaffID: "50000000-0000-4000-8000-000000000001", Channel: "dine_in", Status: "new", GuestName: "Анна К.", TotalMinor: 4250, Notes: "Аллергия на орехи", CreatedAt: now.Add(-3 * time.Minute)},
		{ID: "70000000-0000-4000-8000-000000000002", Channel: "online", Status: "preparing", GuestName: "Марк С.", TotalMinor: 6890, CreatedAt: now.Add(-11 * time.Minute)},
		{ID: "70000000-0000-4000-8000-000000000003", TableID: "61000000-0000-4000-8000-000000000003", StaffID: "50000000-0000-4000-8000-000000000001", Channel: "dine_in", Status: "ready", GuestName: "Олег", TotalMinor: 3240, CreatedAt: now.Add(-18 * time.Minute)},
		{ID: "70000000-0000-4000-8000-000000000004", TableID: "61000000-0000-4000-8000-000000000004", StaffID: "50000000-0000-4000-8000-000000000001", Channel: "dine_in", Status: "served", GuestName: "Наталья", TotalMinor: 7820, CreatedAt: now.Add(-42 * time.Minute)},
		{ID: "70000000-0000-4000-8000-000000000005", Channel: "pickup", Status: "completed", GuestName: "Виктор", TotalMinor: 5370, CreatedAt: now.Add(-75 * time.Minute)},
		{ID: "70000000-0000-4000-8000-000000000006", TableID: "61000000-0000-4000-8000-000000000012", StaffID: "50000000-0000-4000-8000-000000000004", Channel: "preorder", Status: "completed", GuestName: "Ева", TotalMinor: 9550, Notes: "День рождения", CreatedAt: now.Add(-130 * time.Minute)},
		{ID: "70000000-0000-4000-8000-000000000007", Channel: "online", Status: "accepted", GuestName: "Даниэль", TotalMinor: 2860, CreatedAt: now.Add(-7 * time.Minute)},
		{ID: "70000000-0000-4000-8000-000000000008", TableID: "61000000-0000-4000-8000-000000000023", StaffID: "50000000-0000-4000-8000-000000000001", Channel: "dine_in", Status: "completed", GuestName: "Сара", TotalMinor: 12140, CreatedAt: now.Add(-190 * time.Minute)},
		{ID: "70000000-0000-4000-8000-000000000009", Channel: "pickup", Status: "cancelled", GuestName: "Иван", TotalMinor: 1790, Notes: "Отменено гостем", CreatedAt: now.Add(-24 * time.Hour)},
	}
}

func orderMenuItem(venueID string, index int) (string, string) {
	if venueID == secondaryVenueID {
		items := [][2]string{
			{"40000000-0000-4000-8000-000000000101", "Фильтр-кофе"},
			{"40000000-0000-4000-8000-000000000104", "Авокадо-тост"},
			{"40000000-0000-4000-8000-000000000102", "Капучино"},
		}
		return items[index%len(items)][0], items[index%len(items)][1]
	}
	items := [][2]string{
		{"40000000-0000-4000-8000-000000000005", "Тальятелле с грибами"},
		{"40000000-0000-4000-8000-000000000006", "Лосось на гриле"},
		{"40000000-0000-4000-8000-000000000003", "Буррата с томатами"},
		{"40000000-0000-4000-8000-000000000001", "Шакшука"},
		{"40000000-0000-4000-8000-000000000008", "Баскский чизкейк"},
	}
	return items[index%len(items)][0], items[index%len(items)][1]
}

func orderItemStatus(orderStatus string) string {
	switch orderStatus {
	case "new", "preparing", "ready":
		return orderStatus
	case "cancelled":
		return "cancelled"
	default:
		return "served"
	}
}
