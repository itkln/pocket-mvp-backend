package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Register(ctx context.Context, user newUserRecord, session storedSession) (string, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", fmt.Errorf("begin registration: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, email_lookup, password_hash, first_name, last_name, phone, account_role)
		VALUES ($1, $2, $3, $4, $5, $6, 'customer')
		RETURNING id::text`,
		user.email, user.emailLookup, user.passwordHash, user.firstName, user.lastName, user.phone,
	).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", ErrEmailAlreadyExists
		}
		return "", fmt.Errorf("insert user: %w", err)
	}
	session.userID = userID
	if err := insertSession(ctx, tx, session); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit registration: %w", err)
	}
	return userID, nil
}

func (r *PostgresRepository) LoginBlocked(ctx context.Context, lookup []byte, ip string) (bool, error) {
	var blocked bool
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(blocked_until > now(), false)
		FROM auth_login_attempts
		WHERE email_lookup = $1 AND ip_address = $2`, lookup, ip).Scan(&blocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check login attempts: %w", err)
	}
	return blocked, nil
}

func (r *PostgresRepository) FindCredentials(ctx context.Context, lookup []byte) (credentialRecord, bool, error) {
	var record credentialRecord
	err := r.db.QueryRow(ctx, `
		SELECT id::text, email, first_name, last_name, phone, password_hash, account_role, avatar_updated_at
		FROM users
		WHERE email_lookup = $1 AND deleted_at IS NULL AND status = 'active'`, lookup,
	).Scan(
		&record.user.id, &record.user.email, &record.user.firstName, &record.user.lastName,
		&record.user.phone, &record.passwordHash, &record.user.role, &record.user.avatarUpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return credentialRecord{}, false, nil
	}
	if err != nil {
		return credentialRecord{}, false, fmt.Errorf("find user: %w", err)
	}
	return record, true, nil
}

func (r *PostgresRepository) RecordLoginFailure(ctx context.Context, lookup []byte, ip string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth_login_attempts (email_lookup, ip_address, failure_count, window_started_at, blocked_until)
		VALUES ($1, $2, 1, now(), NULL)
		ON CONFLICT (email_lookup, ip_address) DO UPDATE SET
		  failure_count = CASE
		    WHEN auth_login_attempts.window_started_at < now() - interval '15 minutes' THEN 1
		    ELSE auth_login_attempts.failure_count + 1
		  END,
		  window_started_at = CASE
		    WHEN auth_login_attempts.window_started_at < now() - interval '15 minutes' THEN now()
		    ELSE auth_login_attempts.window_started_at
		  END,
		  blocked_until = CASE
		    WHEN auth_login_attempts.window_started_at >= now() - interval '15 minutes'
		      AND auth_login_attempts.failure_count + 1 >= 5
		    THEN now() + interval '15 minutes'
		    ELSE NULL
		  END`, lookup, ip)
	if err != nil {
		return fmt.Errorf("record login failure: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CompleteLogin(ctx context.Context, lookup []byte, ip string, session storedSession) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin login: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM auth_login_attempts WHERE email_lookup = $1 AND ip_address = $2`, lookup, ip); err != nil {
		return fmt.Errorf("clear login attempts: %w", err)
	}
	if err := insertSession(ctx, tx, session); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit login: %w", err)
	}
	return nil
}

func (r *PostgresRepository) FindUserBySession(ctx context.Context, tokenHash string) (encryptedUser, error) {
	var record encryptedUser
	err := r.db.QueryRow(ctx, `
		SELECT u.id::text, u.email, u.first_name, u.last_name, u.phone, u.account_role, u.avatar_updated_at
		FROM user_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.refresh_token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now()
		  AND u.status = 'active' AND u.deleted_at IS NULL`, tokenHash,
	).Scan(
		&record.id, &record.email, &record.firstName, &record.lastName,
		&record.phone, &record.role, &record.avatarUpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return encryptedUser{}, ErrUnauthorized
	}
	if err != nil {
		return encryptedUser{}, fmt.Errorf("authenticate session: %w", err)
	}
	return record, nil
}

func (r *PostgresRepository) UpdateProfile(ctx context.Context, userID string, profile encryptedProfile) (encryptedUser, error) {
	var record encryptedUser
	err := r.db.QueryRow(ctx, `
		UPDATE users
		SET first_name = $1, last_name = $2, phone = $3, updated_at = now()
		WHERE id = $4 AND status = 'active' AND deleted_at IS NULL
		RETURNING id::text, email, first_name, last_name, phone, account_role, avatar_updated_at`,
		profile.firstName, profile.lastName, profile.phone, userID,
	).Scan(
		&record.id, &record.email, &record.firstName, &record.lastName,
		&record.phone, &record.role, &record.avatarUpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return encryptedUser{}, ErrUnauthorized
	}
	if err != nil {
		return encryptedUser{}, fmt.Errorf("update profile: %w", err)
	}
	return record, nil
}

func (r *PostgresRepository) RevokeSession(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE user_sessions
		SET revoked_at = now()
		WHERE refresh_token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	return err
}

func (r *PostgresRepository) LoadUser(ctx context.Context, userID string) (encryptedUser, error) {
	var record encryptedUser
	err := r.db.QueryRow(ctx, `
		SELECT id::text, email, first_name, last_name, phone, account_role, avatar_updated_at
		FROM users
		WHERE id = $1 AND status = 'active' AND deleted_at IS NULL`,
		userID,
	).Scan(
		&record.id, &record.email, &record.firstName, &record.lastName,
		&record.phone, &record.role, &record.avatarUpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return encryptedUser{}, ErrUnauthorized
	}
	if err != nil {
		return encryptedUser{}, fmt.Errorf("load user: %w", err)
	}
	return record, nil
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

func insertSession(ctx context.Context, tx pgx.Tx, session storedSession) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO user_sessions (user_id, refresh_token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		session.userID, session.tokenHash, session.userAgent, session.ipAddress, session.expiresAt,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}
