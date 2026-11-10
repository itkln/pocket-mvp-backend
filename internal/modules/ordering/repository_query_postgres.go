package ordering

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"pocket-mvp-backend/internal/appfault"
	"strings"
)

// One query for the page, one batched query for its item snapshots, independent of page size.
const operationalSelect = `SELECT o.id::text,o.order_number,o.channel,
 CASE WHEN t.identifier IS NOT NULL THEN 'Стол '||t.identifier ELSE 'Онлайн' END,
 COALESCE(o.guest_name,''),o.total_minor,o.tip_minor,o.currency,o.status,COALESCE(o.notes,''),o.created_at,
 COALESCE(r.reservation_code,''),r.starts_at,COALESCE(r.guest_phone,''),COALESCE(r.guest_count,0),
 o.updated_at,o.placed_at,o.completed_at,COALESCE(o.venue_table_id::text,''),COALESCE(t.identifier,''),
 COALESCE(o.reservation_id::text,''),COALESCE(o.bill_id::text,''),COALESCE(r.status,''),
 (b.id IS NULL OR b.status='open')
 AND NOT EXISTS(SELECT 1 FROM payments p WHERE p.order_id=o.id AND p.status NOT IN ('failed','cancelled'))
 AND NOT EXISTS(SELECT 1 FROM payments p WHERE p.bill_id=o.bill_id AND p.status NOT IN ('failed','cancelled'))
 AND NOT EXISTS(SELECT 1 FROM tips tip WHERE tip.order_id=o.id AND tip.status='paid')
 FROM orders o LEFT JOIN venue_tables t ON t.id=o.venue_table_id
 LEFT JOIN reservations r ON r.id=o.reservation_id LEFT JOIN bills b ON b.id=o.bill_id `

func scanOperationalOrder(row orderScanner) (Order, error) {
	var o Order
	err := row.Scan(&o.ID, &o.Number, &o.Channel, &o.Source, &o.GuestName, &o.TotalMinor, &o.TipMinor, &o.Currency, &o.Status, &o.Notes, &o.CreatedAt,
		&o.ReservationCode, &o.ReservationStartsAt, &o.GuestPhone, &o.GuestCount, &o.UpdatedAt, &o.PlacedAt, &o.CompletedAt, &o.TableID, &o.TableLabel, &o.ReservationID, &o.BillID, &o.ReservationStatus, &o.cancelAllowed)
	o.Items = []OrderItem{}
	return o, err
}
func (r *PostgresRepository) ListPage(ctx context.Context, venueID string, query Query) (Page, error) {
	if err := query.normalize(venueID); err != nil {
		return Page{}, err
	}
	predicate := `o.status IN (` + sqlStatuses(activeStatuses) + `)`
	direction, comparison := "ASC", ">"
	if query.Scope == ScopeHistory {
		predicate = `o.status IN (` + sqlStatuses(finalStatuses) + `)`
		direction, comparison = "DESC", "<"
	}
	args := []any{venueID, query.Statuses, query.Channel, query.TableID, query.Limit + 1, query.Search, query.Table, query.DateFrom, query.DateTo}
	sql := operationalSelect + ` WHERE o.venue_id=$1 AND ` + predicate + `
 AND ($2::text[] IS NULL OR o.status=ANY($2))
 AND ($3::text='' OR o.channel=$3) AND ($4::text='' OR o.venue_table_id=NULLIF($4,'')::uuid)
 AND ($6::text='' OR o.order_number::text=$6)
 AND ($7::text='' OR lower(t.identifier)=lower($7))
 AND ($8::text='' OR o.created_at >= (NULLIF($8,'')::date::timestamp AT TIME ZONE (SELECT timezone FROM venues WHERE id=$1)))
 AND ($9::text='' OR o.created_at < ((NULLIF($9,'')::date + 1)::timestamp AT TIME ZONE (SELECT timezone FROM venues WHERE id=$1)))`
	if query.after != nil {
		sql += ` AND (o.created_at,o.id) ` + comparison + ` ($10::timestamptz,$11::uuid)`
		args = append(args, query.after.CreatedAt, query.after.ID)
	}
	sql += ` ORDER BY o.created_at ` + direction + `,o.id ` + direction + ` LIMIT $5`
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	page := Page{Orders: []Order{}}
	for rows.Next() {
		o, err := scanOperationalOrder(rows)
		if err != nil {
			return Page{}, err
		}
		page.Orders = append(page.Orders, o)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	rows.Close()
	if len(page.Orders) > query.Limit {
		page.HasMore = true
		page.Orders = page.Orders[:query.Limit]
		page.NextCursor = query.cursorFor(venueID, page.Orders[len(page.Orders)-1])
	}
	if err := r.loadPageItems(ctx, page.Orders); err != nil {
		return Page{}, err
	}
	return page, nil
}
func (r *PostgresRepository) Get(ctx context.Context, venueID, orderID string) (Order, error) {
	o, err := scanOperationalOrder(r.db.QueryRow(ctx, operationalSelect+` WHERE o.venue_id=$1 AND o.id=$2 AND o.status<>'draft'`, venueID, orderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, appfault.ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	orders := []Order{o}
	if err := r.loadPageItems(ctx, orders); err != nil {
		return Order{}, err
	}
	events, err := r.db.Query(ctx, `SELECT e.id::text,COALESCE(e.actor_id::text,''),e.actor_name,e.previous_status,e.new_status,e.reason,e.comment,e.created_at FROM order_status_events e JOIN orders o ON o.id=e.order_id WHERE o.id=$1 AND o.venue_id=$2 ORDER BY e.created_at,e.id`, orderID, venueID)
	if err != nil {
		return Order{}, err
	}
	defer events.Close()
	for events.Next() {
		var e StatusEvent
		if err = events.Scan(&e.ID, &e.ActorID, &e.ActorName, &e.PreviousStatus, &e.NewStatus, &e.Reason, &e.Comment, &e.CreatedAt); err != nil {
			return Order{}, err
		}
		if r.protector != nil && e.ActorName != "System" {
			parts := strings.SplitN(e.ActorName, " ", 2)
			if len(parts) != 2 {
				return Order{}, appfault.ErrConflict
			}
			first, err := r.protector.Decrypt(parts[0], "users.first_name")
			if err != nil {
				return Order{}, err
			}
			last, err := r.protector.Decrypt(parts[1], "users.last_name")
			if err != nil {
				return Order{}, err
			}
			e.ActorName = strings.TrimSpace(first + " " + last)
		}
		orders[0].StatusEvents = append(orders[0].StatusEvents, e)
	}
	if err = events.Err(); err != nil {
		return Order{}, err
	}
	return orders[0], nil
}
func (r *PostgresRepository) loadPageItems(ctx context.Context, orders []Order) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]string, len(orders))
	byID := map[string]int{}
	for i := range orders {
		ids[i] = orders[i].ID
		byID[ids[i]] = i
	}
	rows, err := r.db.Query(ctx, `SELECT order_id::text,id::text,item_name,quantity,unit_price_minor,total_minor,COALESCE(notes,''),status
 FROM order_items WHERE order_id=ANY($1::uuid[]) ORDER BY created_at,id`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var item OrderItem
		if err := rows.Scan(&id, &item.ID, &item.Name, &item.Quantity, &item.UnitPriceMinor, &item.TotalMinor, &item.Notes, &item.Status); err != nil {
			return err
		}
		if i, ok := byID[id]; ok {
			orders[i].Items = append(orders[i].Items, item)
		}
	}
	return rows.Err()
}
