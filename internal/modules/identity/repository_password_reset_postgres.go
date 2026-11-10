package identity

// Password reset tokens: issuing, discarding and redeeming them.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) CreatePasswordReset(
	ctx context.Context,
	userID string,
	tokenHash string,
	ip string,
	expiresAt time.Time,
	cooldown time.Duration,
) (bool, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, fmt.Errorf("begin password reset request: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var recentlyRequested bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM password_reset_tokens
			WHERE user_id = $1 AND used_at IS NULL AND expires_at > now()
			  AND created_at > now() - make_interval(secs => $2)
		)`, userID, int(cooldown.Seconds())).Scan(&recentlyRequested)
	if err != nil {
		return false, fmt.Errorf("check password reset cooldown: %w", err)
	}
	if recentlyRequested {
		return false, tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE password_reset_tokens
		SET used_at = now()
		WHERE user_id = $1 AND used_at IS NULL`, userID); err != nil {
		return false, fmt.Errorf("invalidate previous password reset tokens: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, requested_ip, expires_at)
		VALUES ($1, $2, $3, $4)`,
		userID, tokenHash, ip, expiresAt); err != nil {
		return false, fmt.Errorf("create password reset token: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit password reset request: %w", err)
	}
	return true, nil
}

func (r *PostgresRepository) DeletePasswordReset(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM password_reset_tokens WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *PostgresRepository) ResetPassword(ctx context.Context, tokenHash, passwordHash string) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin password reset: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID string
	err = tx.QueryRow(ctx, `
		SELECT user_id::text
		FROM password_reset_tokens
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		FOR UPDATE`, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidResetToken
	}
	if err != nil {
		return fmt.Errorf("find password reset token: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`, passwordHash, userID); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE password_reset_tokens
		SET used_at = COALESCE(used_at, now())
		WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("consume password reset tokens: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE user_sessions
		SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
		return fmt.Errorf("revoke sessions after password reset: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password reset: %w", err)
	}
	return nil
}
