package identity

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"pocket-mvp-backend/internal/security"
)

const passwordResetCooldown = time.Minute

func (s *Service) RequestPasswordReset(ctx context.Context, input PasswordResetRequest) error {
	email := security.NormalizeEmail(input.Email)
	if !validEmail(email) {
		return nil
	}

	record, found, err := s.repository.FindCredentials(ctx, s.protector.Lookup(email))
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	token, err := security.NewSessionToken()
	if err != nil {
		return err
	}
	tokenHash := security.HashSessionToken(token)
	created, err := s.repository.CreatePasswordReset(
		ctx,
		record.user.id,
		tokenHash,
		normalizedIP(input.IPAddress),
		time.Now().UTC().Add(s.resetTTL),
		passwordResetCooldown,
	)
	if err != nil {
		return err
	}
	if !created {
		return nil
	}

	locale := normalizeResetLocale(input.Locale)
	resetURL := fmt.Sprintf("%s/%s/reset-password?token=%s", s.resetBaseURL, locale, url.QueryEscape(token))
	if err = s.resetSender.SendPasswordReset(ctx, email, resetURL, locale); err != nil {
		_ = s.repository.DeletePasswordReset(ctx, tokenHash)
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

	return s.repository.ResetPassword(ctx, security.HashSessionToken(input.Token), passwordHash)
}

func normalizeResetLocale(locale string) string {
	switch strings.ToLower(strings.TrimSpace(locale)) {
	case "ru", "ua", "uk", "sk":
		return strings.ToLower(strings.TrimSpace(locale))
	default:
		return "en"
	}
}
