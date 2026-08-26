package demodata

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type reviewSeed struct {
	ID         string
	OrderID    string
	TableID    string
	Rating     int
	Body       string
	OwnerReply string
	CreatedAt  time.Time
}

func seedReviews(ctx context.Context, tx pgx.Tx, ownerID, venueID string) error {
	for _, review := range demoReviews(venueID) {
		_, err := tx.Exec(ctx, `
			INSERT INTO reviews (
				id, venue_id, order_id, venue_table_id, rating, body, status,
				owner_reply, replied_by_user_id, replied_at, created_at, updated_at
			)
			VALUES (
				$1, $2, $3, NULLIF($4, '')::uuid, $5, $6, 'published', NULLIF($7, ''),
				CASE WHEN $7 = '' THEN NULL ELSE $8::uuid END,
				CASE WHEN $7 = '' THEN NULL ELSE $9::timestamptz + interval '20 minutes' END,
				$9::timestamptz, $9::timestamptz
			)`,
			review.ID, venueID, review.OrderID, review.TableID, review.Rating,
			review.Body, review.OwnerReply, ownerID, review.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("seed review %s: %w", review.ID, err)
		}
	}
	return nil
}

func demoReviews(venueID string) []reviewSeed {
	now := time.Now().UTC()
	if venueID == secondaryVenueID {
		return []reviewSeed{
			{ID: "90000000-0000-4000-8000-000000000101", OrderID: "70000000-0000-4000-8000-000000000102", Rating: 5, Body: "Отличный фильтр и очень внимательная команда.", OwnerReply: "Спасибо! Будем рады видеть вас снова.", CreatedAt: now.Add(-45 * time.Minute)},
		}
	}
	return []reviewSeed{
		{ID: "90000000-0000-4000-8000-000000000001", OrderID: "70000000-0000-4000-8000-000000000005", Rating: 5, Body: "Очень быстро приготовили заказ, паста была отличной.", OwnerReply: "Спасибо за отзыв! Передадим комплимент кухне.", CreatedAt: now.Add(-55 * time.Minute)},
		{ID: "90000000-0000-4000-8000-000000000002", OrderID: "70000000-0000-4000-8000-000000000006", TableID: "61000000-0000-4000-8000-000000000012", Rating: 4, Body: "Красивый зал и вкусные десерты. Немного долго ждали напитки.", CreatedAt: now.Add(-110 * time.Minute)},
		{ID: "90000000-0000-4000-8000-000000000003", OrderID: "70000000-0000-4000-8000-000000000008", TableID: "61000000-0000-4000-8000-000000000023", Rating: 5, Body: "Терраса супер, официант помог подобрать вино.", OwnerReply: "Спасибо! София будет рада вашим словам.", CreatedAt: now.Add(-170 * time.Minute)},
		{ID: "90000000-0000-4000-8000-000000000004", OrderID: "70000000-0000-4000-8000-000000000009", Rating: 3, Body: "Пришлось отменить заказ из-за долгого ожидания подтверждения.", CreatedAt: now.Add(-23 * time.Hour)},
	}
}
