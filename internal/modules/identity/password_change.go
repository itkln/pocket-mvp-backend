package identity

import (
	"context"
	"fmt"

	"pocket-mvp-backend/internal/security"
)

func (s *Service) ChangePassword(ctx context.Context, input ChangePasswordInput) error {
	if input.UserID == "" || input.SessionToken == "" || len(input.CurrentPassword) == 0 || len(input.CurrentPassword) > 128 || !validPassword(input.NewPassword) {
		return ErrInvalidInput
	}

	currentHash, err := s.repository.PasswordHash(ctx, input.UserID)
	if err != nil {
		return err
	}

	valid, verifyErr := security.VerifyPassword(input.CurrentPassword, currentHash)
	if verifyErr != nil || !valid {
		return ErrInvalidCurrentPassword
	}
	newHash, err := security.HashPassword(input.NewPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	return s.repository.ChangePassword(
		ctx,
		input.UserID,
		currentHash,
		newHash,
		security.HashSessionToken(input.SessionToken),
	)
}
