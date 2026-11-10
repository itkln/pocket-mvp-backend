package feedback

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

func (r *PostgresRepository) List(ctx context.Context, venueID string) ([]Review, error) {
	rows, err := r.db.Query(ctx, `
		SELECT r.id::text, COALESCE(NULLIF(o.guest_name, ''), 'Гость'), r.rating,
		       COALESCE(t.identifier, ''), COALESCE('#' || o.order_number::text, ''),
		       COALESCE(r.body, ''), COALESCE(r.owner_reply, ''), r.replied_at, r.created_at
		FROM reviews r
		LEFT JOIN orders o ON o.id = r.order_id
		LEFT JOIN venue_tables t ON t.id = r.venue_table_id
		WHERE r.venue_id = $1 AND r.status = 'published'
		ORDER BY r.created_at DESC`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Review{}
	for rows.Next() {
		review, scanErr := scanReview(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, review)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) Reply(ctx context.Context, ownerID, venueID, reviewID, reply string) (Review, error) {
	var review Review
	err := r.db.QueryRow(ctx, `
		UPDATE reviews r
		SET owner_reply = $4, replied_by_user_id = $1, replied_at = now()
		WHERE r.id = $3 AND r.venue_id = $2
		RETURNING r.id::text,
		          COALESCE(NULLIF((SELECT guest_name FROM orders WHERE id = r.order_id), ''), 'Гость'),
		          r.rating, COALESCE((SELECT identifier FROM venue_tables WHERE id = r.venue_table_id), ''),
		          COALESCE('#' || (SELECT order_number::text FROM orders WHERE id = r.order_id), ''),
		          COALESCE(r.body, ''), COALESCE(r.owner_reply, ''), r.replied_at, r.created_at`,
		ownerID, venueID, reviewID, reply,
	).Scan(
		&review.ID, &review.GuestName, &review.Rating, &review.Table, &review.Order,
		&review.Body, &review.OwnerReply, &review.RepliedAt, &review.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, appfault.ErrNotFound
	}
	return review, err
}

type reviewScanner interface {
	Scan(...any) error
}

func scanReview(row reviewScanner) (Review, error) {
	var review Review
	err := row.Scan(
		&review.ID, &review.GuestName, &review.Rating, &review.Table, &review.Order,
		&review.Body, &review.OwnerReply, &review.RepliedAt, &review.CreatedAt,
	)
	return review, err
}
