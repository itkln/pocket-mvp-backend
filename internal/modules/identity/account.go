package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"pocket-mvp-backend/internal/security"
)

const maxAvatarBytes = 2 << 20

func (s *Service) ChangeEmail(ctx context.Context, input ChangeEmailInput) (User, error) {
	input.NewEmail = security.NormalizeEmail(input.NewEmail)
	if input.UserID == "" || !validEmail(input.NewEmail) || len(input.CurrentPassword) == 0 || len(input.CurrentPassword) > 128 {
		return User{}, ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, fmt.Errorf("begin email change: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentHash string
	err = tx.QueryRow(ctx, `
		SELECT password_hash
		FROM users
		WHERE id = $1 AND status = 'active' AND deleted_at IS NULL
		FOR UPDATE`,
		input.UserID,
	).Scan(&currentHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, fmt.Errorf("load password for email change: %w", err)
	}
	valid, verifyErr := security.VerifyPassword(input.CurrentPassword, currentHash)
	if verifyErr != nil || !valid {
		return User{}, ErrInvalidCurrentPassword
	}

	encryptedEmail, err := s.protector.Encrypt(input.NewEmail, "users.email")
	if err != nil {
		return User{}, fmt.Errorf("encrypt email: %w", err)
	}
	_, err = tx.Exec(ctx, `
		UPDATE users
		SET email = $1, email_lookup = $2, email_verified_at = NULL, updated_at = now()
		WHERE id = $3 AND status = 'active' AND deleted_at IS NULL`,
		encryptedEmail, s.protector.Lookup(input.NewEmail), input.UserID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrEmailAlreadyExists
		}
		return User{}, fmt.Errorf("update email: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit email change: %w", err)
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
	command, err := s.db.Exec(ctx, `
		UPDATE users
		SET avatar_data = $1, avatar_mime_type = $2, avatar_updated_at = now(), updated_at = now()
		WHERE id = $3 AND status = 'active' AND deleted_at IS NULL`,
		encryptedData, contentType, userID,
	)
	if err != nil {
		return User{}, fmt.Errorf("update avatar: %w", err)
	}
	if command.RowsAffected() == 0 {
		return User{}, ErrUnauthorized
	}
	return s.loadUser(ctx, userID)
}

func (s *Service) Avatar(ctx context.Context, userID string) (Avatar, error) {
	if userID == "" {
		return Avatar{}, ErrUnauthorized
	}
	var avatar Avatar
	err := s.db.QueryRow(ctx, `
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
	avatar.Data, err = s.protector.DecryptBytes(avatar.Data, "users.avatar_data")
	if err != nil {
		return Avatar{}, fmt.Errorf("decrypt avatar: %w", err)
	}
	return avatar, nil
}

func (s *Service) loadUser(ctx context.Context, userID string) (User, error) {
	var record encryptedUser
	err := s.db.QueryRow(ctx, `
		SELECT id::text, email, first_name, last_name, phone, account_role, avatar_updated_at
		FROM users
		WHERE id = $1 AND status = 'active' AND deleted_at IS NULL`,
		userID,
	).Scan(
		&record.id, &record.email, &record.firstName, &record.lastName,
		&record.phone, &record.role, &record.avatarUpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, fmt.Errorf("load user: %w", err)
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
