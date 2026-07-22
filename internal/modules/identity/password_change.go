package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/security"
)

func (s *Service) ChangePassword(ctx context.Context, input ChangePasswordInput) error {
	if input.UserID == "" || input.SessionToken == "" || len(input.CurrentPassword) == 0 || len(input.CurrentPassword) > 128 || !validPassword(input.NewPassword) {
		return ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin password change: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentHash string
	err = tx.QueryRow(ctx, `
		SELECT password_hash
		FROM users
		WHERE id = $1 AND status = 'active' AND deleted_at IS NULL
		FOR UPDATE`, input.UserID).Scan(&currentHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUnauthorized
	}
	if err != nil {
		return fmt.Errorf("load password: %w", err)
	}

	valid, verifyErr := security.VerifyPassword(input.CurrentPassword, currentHash)
	if verifyErr != nil || !valid {
		return ErrInvalidCurrentPassword
	}
	newHash, err := security.HashPassword(input.NewPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, newHash, input.UserID); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE user_sessions
		SET revoked_at = now()
		WHERE user_id = $1 AND refresh_token_hash <> $2 AND revoked_at IS NULL`,
		input.UserID, security.HashSessionToken(input.SessionToken)); err != nil {
		return fmt.Errorf("revoke other sessions: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE password_reset_tokens
		SET used_at = COALESCE(used_at, now())
		WHERE user_id = $1`, input.UserID); err != nil {
		return fmt.Errorf("consume password reset tokens: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password change: %w", err)
	}
	return nil
}
