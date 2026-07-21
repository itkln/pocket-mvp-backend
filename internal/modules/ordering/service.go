package ordering

import (
	"context"
	"errors"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/appfault"
)

var orderStatusPattern = regexp.MustCompile(`^(new|accepted|preparing|ready|served|completed|cancelled)$`)

type VenueAuthorizer interface {
	RequireOwner(context.Context, string, string) error
}

type Service struct {
	db         *pgxpool.Pool
	authorizer VenueAuthorizer
}

func NewService(db *pgxpool.Pool, authorizer VenueAuthorizer) *Service {
	return &Service{db: db, authorizer: authorizer}
}

func (s *Service) List(ctx context.Context, ownerID, venueID string) ([]Order, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT o.id::text, o.order_number, o.channel,
		       CASE
		         WHEN t.identifier IS NOT NULL THEN 'Стол ' || t.identifier
		         WHEN o.channel = 'pickup' THEN 'Самовывоз'
		         ELSE 'Онлайн'
		       END,
		       COALESCE(o.guest_name, ''), o.total_minor, o.currency, o.status,
		       COALESCE(o.notes, ''), o.created_at
		FROM orders o
		LEFT JOIN venue_tables t ON t.id = o.venue_table_id
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
	return result, rows.Err()
}

func (s *Service) UpdateStatus(ctx context.Context, ownerID, venueID, orderID, status string) (Order, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return Order{}, err
	}
	if !orderStatusPattern.MatchString(status) {
		return Order{}, appfault.ErrInvalidInput
	}

	var order Order
	err := s.db.QueryRow(ctx, `
		UPDATE orders o
		SET status = $3,
		    completed_at = CASE WHEN $3 IN ('completed', 'cancelled') THEN now() ELSE completed_at END
		WHERE o.id = $2 AND o.venue_id = $1
		RETURNING o.id::text, o.order_number, o.channel,
		          CASE
		            WHEN o.venue_table_id IS NOT NULL THEN 'Стол ' || COALESCE((SELECT identifier FROM venue_tables WHERE id = o.venue_table_id), '')
		            WHEN o.channel = 'pickup' THEN 'Самовывоз'
		            ELSE 'Онлайн'
		          END,
		          COALESCE(o.guest_name, ''), o.total_minor, o.currency, o.status,
		          COALESCE(o.notes, ''), o.created_at`, venueID, orderID, status,
	).Scan(
		&order.ID, &order.Number, &order.Channel, &order.Source, &order.GuestName,
		&order.TotalMinor, &order.Currency, &order.Status, &order.Notes, &order.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, appfault.ErrNotFound
	}
	return order, err
}

type orderScanner interface {
	Scan(...any) error
}

func scanOrder(row orderScanner) (Order, error) {
	var order Order
	err := row.Scan(
		&order.ID, &order.Number, &order.Channel, &order.Source, &order.GuestName,
		&order.TotalMinor, &order.Currency, &order.Status, &order.Notes, &order.CreatedAt,
	)
	return order, err
}
