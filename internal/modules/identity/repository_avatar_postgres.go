package identity

// Profile photos, stored encrypted with the user row.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) UpdateAvatar(ctx context.Context, userID, contentType string, encryptedData []byte) error {
	result, err := r.db.Exec(ctx, `
		UPDATE users
		SET avatar_data = $1, avatar_mime_type = $2, avatar_updated_at = now(), updated_at = now()
		WHERE id = $3 AND status = 'active' AND deleted_at IS NULL`,
		encryptedData, contentType, userID,
	)
	if err != nil {
		return fmt.Errorf("update avatar: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUnauthorized
	}
	return nil
}

func (r *PostgresRepository) Avatar(ctx context.Context, userID string) (Avatar, error) {
	var avatar Avatar
	err := r.db.QueryRow(ctx, `
		SELECT avatar_data, avatar_mime_type
		FROM users
		WHERE id = $1 AND status = 'active' AND deleted_at IS NULL
		  AND avatar_data IS NOT NULL AND avatar_mime_type IS NOT NULL`,
		userID,
	).Scan(&avatar.Data, &avatar.ContentType)
	if errors.Is(err, pgx.ErrNoRows) {
		return Avatar{}, ErrAvatarNotFound
	}
	if err != nil {
		return Avatar{}, fmt.Errorf("load avatar: %w", err)
	}
	return avatar, nil
}
