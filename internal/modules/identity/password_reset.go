package identity

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/security"
)

const passwordResetCooldown = time.Minute

func (s *Service) RequestPasswordReset(ctx context.Context, input PasswordResetRequest) error {
	email := security.NormalizeEmail(input.Email)
	if !validEmail(email) {
		return nil
	}

	record, found, err := s.findUserByLookup(ctx, s.protector.Lookup(email))
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin password reset request: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var recentlyRequested bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM password_reset_tokens
			WHERE user_id = $1 AND used_at IS NULL AND expires_at > now()
			  AND created_at > now() - make_interval(secs => $2)
		)`, record.user.id, int(passwordResetCooldown.Seconds())).Scan(&recentlyRequested)
	if err != nil {
		return fmt.Errorf("check password reset cooldown: %w", err)
	}
	if recentlyRequested {
		return tx.Commit(ctx)
	}

	token, err := security.NewSessionToken()
	if err != nil {
		return err
	}
	tokenHash := security.HashSessionToken(token)
	if _, err = tx.Exec(ctx, `
		UPDATE password_reset_tokens
		SET used_at = now()
		WHERE user_id = $1 AND used_at IS NULL`, record.user.id); err != nil {
		return fmt.Errorf("invalidate previous password reset tokens: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, requested_ip, expires_at)
		VALUES ($1, $2, $3, $4)`,
		record.user.id, tokenHash, normalizedIP(input.IPAddress), time.Now().UTC().Add(s.resetTTL)); err != nil {
		return fmt.Errorf("create password reset token: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password reset request: %w", err)
	}

	locale := normalizeResetLocale(input.Locale)
	resetURL := fmt.Sprintf("%s/%s/reset-password?token=%s", s.resetBaseURL, locale, url.QueryEscape(token))
	if err = s.resetSender.SendPasswordReset(ctx, email, resetURL, locale); err != nil {
		_, _ = s.db.Exec(ctx, `DELETE FROM password_reset_tokens WHERE token_hash = $1`, tokenHash)
		return fmt.Errorf("send password reset: %w", err)
	}
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, input PasswordResetConfirmation) error {
	if len(input.Token) < 32 {
		return ErrInvalidResetToken
	}
	if !validPassword(input.Password) {
		return ErrInvalidInput
	}
	passwordHash, err := security.HashPassword(input.Password)
	if err != nil {
		return fmt.Errorf("hash reset password: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin password reset: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID string
	err = tx.QueryRow(ctx, `
		SELECT user_id::text
		FROM password_reset_tokens
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		FOR UPDATE`, security.HashSessionToken(input.Token)).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidResetToken
	}
	if err != nil {
		return fmt.Errorf("find password reset token: %w", err)
	}

	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, passwordHash, userID); err != nil {
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

func normalizeResetLocale(locale string) string {
	switch strings.ToLower(strings.TrimSpace(locale)) {
	case "ru", "ua", "uk", "sk":
		return strings.ToLower(strings.TrimSpace(locale))
	default:
		return "en"
	}
}
