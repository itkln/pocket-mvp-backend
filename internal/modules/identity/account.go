package identity

import (
	"context"
	"fmt"

	"pocket-mvp-backend/internal/security"
)

const maxAvatarBytes = 2 << 20

func (s *Service) ChangeEmail(ctx context.Context, input ChangeEmailInput) (User, error) {
	input.NewEmail = security.NormalizeEmail(input.NewEmail)
	if input.UserID == "" || !validEmail(input.NewEmail) || len(input.CurrentPassword) == 0 || len(input.CurrentPassword) > 128 {
		return User{}, ErrInvalidInput
	}

	currentHash, err := s.repository.PasswordHash(ctx, input.UserID)
	if err != nil {
		return User{}, err
	}
	valid, verifyErr := security.VerifyPassword(input.CurrentPassword, currentHash)
	if verifyErr != nil || !valid {
		return User{}, ErrInvalidCurrentPassword
	}

	encryptedEmail, err := s.protector.Encrypt(input.NewEmail, "users.email")
	if err != nil {
		return User{}, fmt.Errorf("encrypt email: %w", err)
	}
	if err := s.repository.ChangeEmail(
		ctx,
		input.UserID,
		encryptedEmail,
		s.protector.Lookup(input.NewEmail),
		currentHash,
	); err != nil {
		return User{}, err
	}
	return s.loadUser(ctx, input.UserID)
}

func (s *Service) UpdateAvatar(ctx context.Context, userID, contentType string, data []byte) (User, error) {
	if userID == "" || len(data) == 0 || len(data) > maxAvatarBytes || !allowedAvatarType(contentType) {
		return User{}, ErrInvalidInput
	}
	encryptedData, err := s.protector.EncryptBytes(data, "users.avatar_data")
	if err != nil {
		return User{}, fmt.Errorf("encrypt avatar: %w", err)
	}
	if err := s.repository.UpdateAvatar(ctx, userID, contentType, encryptedData); err != nil {
		return User{}, err
	}
	return s.loadUser(ctx, userID)
}

func (s *Service) Avatar(ctx context.Context, userID string) (Avatar, error) {
	if userID == "" {
		return Avatar{}, ErrUnauthorized
	}
	avatar, err := s.repository.Avatar(ctx, userID)
	if err != nil {
		return Avatar{}, err
	}
	avatar.Data, err = s.protector.DecryptBytes(avatar.Data, "users.avatar_data")
	if err != nil {
		return Avatar{}, fmt.Errorf("decrypt avatar: %w", err)
	}
	return avatar, nil
}

func (s *Service) loadUser(ctx context.Context, userID string) (User, error) {
	record, err := s.repository.LoadUser(ctx, userID)
	if err != nil {
		return User{}, err
	}
	user, err := s.decryptUser(record)
	if err != nil {
		return User{}, err
	}
	user.Capabilities, err = s.capabilities.ListCapabilities(ctx, user.ID)
	return user, err
}

func allowedAvatarType(contentType string) bool {
	return contentType == "image/jpeg" || contentType == "image/png" || contentType == "image/webp"
}
