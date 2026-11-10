package identity

// The signed-in user's account: profile, email and password.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *PostgresRepository) UpdateProfile(ctx context.Context, userID string, profile encryptedProfile) (encryptedUser, error) {
	return scanUser(r.db.QueryRow(ctx, `
		UPDATE users
		SET first_name = $1, last_name = $2, phone = $3, updated_at = now()
		WHERE id = $4 AND status = 'active' AND deleted_at IS NULL
		RETURNING `+userColumns,
		profile.firstName, profile.lastName, profile.phone, userID), "update profile")
}

func (r *PostgresRepository) LoadUser(ctx context.Context, userID string) (encryptedUser, error) {
	return scanUser(r.db.QueryRow(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE id = $1 AND status = 'active' AND deleted_at IS NULL`, userID), "load user")
}

func (r *PostgresRepository) PasswordHash(ctx context.Context, userID string) (string, error) {
	var passwordHash string
	err := r.db.QueryRow(ctx, `
		SELECT password_hash
		FROM users
		WHERE id = $1 AND status = 'active' AND deleted_at IS NULL`, userID,
	).Scan(&passwordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", fmt.Errorf("load password: %w", err)
	}
	return passwordHash, nil
}

func (r *PostgresRepository) ChangeEmail(ctx context.Context, userID, encryptedEmail string, lookup []byte, expectedPasswordHash string) error {
	result, err := r.db.Exec(ctx, `
		UPDATE users
		SET email = $1, email_lookup = $2, email_verified_at = NULL, updated_at = now()
		WHERE id = $3 AND password_hash = $4 AND status = 'active' AND deleted_at IS NULL`,
		encryptedEmail, lookup, userID, expectedPasswordHash,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEmailAlreadyExists
		}
		return fmt.Errorf("update email: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrInvalidCurrentPassword
	}
	return nil
}

func (r *PostgresRepository) ChangePassword(ctx context.Context, userID, expectedHash, newHash, currentSessionHash string) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin password change: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := tx.Exec(ctx, `
		UPDATE users
		SET password_hash = $1, updated_at = now()
		WHERE id = $2 AND password_hash = $3 AND status = 'active' AND deleted_at IS NULL`,
		newHash, userID, expectedHash,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrInvalidCurrentPassword
	}
	if _, err = tx.Exec(ctx, `
		UPDATE user_sessions
		SET revoked_at = now()
		WHERE user_id = $1 AND refresh_token_hash <> $2 AND revoked_at IS NULL`,
		userID, currentSessionHash); err != nil {
		return fmt.Errorf("revoke other sessions: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE password_reset_tokens
		SET used_at = COALESCE(used_at, now())
		WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("consume password reset tokens: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password change: %w", err)
	}
	return nil
}
