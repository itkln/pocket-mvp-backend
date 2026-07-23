package workforce

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

func (r *PostgresRepository) List(ctx context.Context, venueID string) ([]StaffMember, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, COALESCE(display_name, ''), invited_email, role, status, invited_at, accepted_at
		FROM venue_staff
		WHERE venue_id = $1 AND status <> 'removed'
		ORDER BY created_at`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []StaffMember{}
	for rows.Next() {
		var member StaffMember
		if err := rows.Scan(&member.ID, &member.DisplayName, &member.Email, &member.Role, &member.Status, &member.InvitedAt, &member.AcceptedAt); err != nil {
			return nil, err
		}
		result = append(result, member)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) Create(ctx context.Context, ownerID, venueID string, input StaffInput) (StaffMember, error) {
	var member StaffMember
	err := r.db.QueryRow(ctx, `
		INSERT INTO venue_staff (venue_id, display_name, invited_email, role, status, invited_by_user_id)
		VALUES ($1, $2, $3, $4, 'invited', $5)
		RETURNING id::text, display_name, invited_email, role, status, invited_at, accepted_at`,
		venueID, input.DisplayName, input.Email, input.Role, ownerID,
	).Scan(&member.ID, &member.DisplayName, &member.Email, &member.Role, &member.Status, &member.InvitedAt, &member.AcceptedAt)
	return member, appfault.MapWriteError(err)
}

func (r *PostgresRepository) Update(ctx context.Context, venueID, staffID string, input StaffInput) (StaffMember, error) {
	var member StaffMember
	err := r.db.QueryRow(ctx, `
		UPDATE venue_staff
		SET role = $3, status = $4
		WHERE id = $2 AND venue_id = $1 AND status <> 'removed'
		RETURNING id::text, COALESCE(display_name, ''), invited_email, role, status, invited_at, accepted_at`,
		venueID, staffID, input.Role, input.Status,
	).Scan(&member.ID, &member.DisplayName, &member.Email, &member.Role, &member.Status, &member.InvitedAt, &member.AcceptedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return StaffMember{}, appfault.ErrNotFound
	}
	return member, appfault.MapWriteError(err)
}

func (r *PostgresRepository) Delete(ctx context.Context, venueID, staffID string) error {
	result, err := r.db.Exec(ctx, `
		UPDATE venue_staff
		SET status = 'removed', removed_at = now()
		WHERE id = $2 AND venue_id = $1 AND status <> 'removed'`, venueID, staffID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return appfault.ErrNotFound
	}
	return nil
}
