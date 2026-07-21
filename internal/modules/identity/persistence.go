package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/security"
)

type encryptedRegistration struct {
	email     string
	firstName string
	lastName  string
	phone     *string
}

type encryptedUser struct {
	id        string
	email     string
	firstName string
	lastName  string
	phone     *string
	role      string
}

type credentialRecord struct {
	user         encryptedUser
	passwordHash string
}

func (s *Service) encryptRegistration(input RegisterInput) (encryptedRegistration, error) {
	email, err := s.protector.Encrypt(input.Email, "users.email")
	if err != nil {
		return encryptedRegistration{}, err
	}
	firstName, err := s.protector.Encrypt(input.FirstName, "users.first_name")
	if err != nil {
		return encryptedRegistration{}, err
	}
	lastName, err := s.protector.Encrypt(input.LastName, "users.last_name")
	if err != nil {
		return encryptedRegistration{}, err
	}
	var phone *string
	if input.Phone != "" {
		value, encryptErr := s.protector.Encrypt(input.Phone, "users.phone")
		if encryptErr != nil {
			return encryptedRegistration{}, encryptErr
		}
		phone = &value
	}
	return encryptedRegistration{email: email, firstName: firstName, lastName: lastName, phone: phone}, nil
}

func (s *Service) decryptUser(record encryptedUser) (User, error) {
	email, err := s.protector.Decrypt(record.email, "users.email")
	if err != nil {
		return User{}, err
	}
	firstName, err := s.protector.Decrypt(record.firstName, "users.first_name")
	if err != nil {
		return User{}, err
	}
	lastName, err := s.protector.Decrypt(record.lastName, "users.last_name")
	if err != nil {
		return User{}, err
	}
	phone := ""
	if record.phone != nil {
		phone, err = s.protector.Decrypt(*record.phone, "users.phone")
		if err != nil {
			return User{}, err
		}
	}
	return User{
		ID: record.id, Email: email, FirstName: firstName, LastName: lastName,
		Phone: phone, Role: record.role,
	}, nil
}

func (s *Service) findUserByLookup(ctx context.Context, lookup []byte) (credentialRecord, bool, error) {
	var record credentialRecord
	err := s.db.QueryRow(ctx, `
		SELECT id::text, email, first_name, last_name, phone, password_hash, account_role
		FROM users
		WHERE email_lookup = $1 AND deleted_at IS NULL AND status = 'active'`, lookup,
	).Scan(
		&record.user.id, &record.user.email, &record.user.firstName, &record.user.lastName,
		&record.user.phone, &record.passwordHash, &record.user.role,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return credentialRecord{}, false, nil
	}
	if err != nil {
		return credentialRecord{}, false, fmt.Errorf("find user: %w", err)
	}
	return record, true, nil
}

func (s *Service) loginBlocked(ctx context.Context, lookup []byte, ip string) (bool, error) {
	var blocked bool
	err := s.db.QueryRow(ctx, `
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

func (s *Service) createSession(ctx context.Context, tx pgx.Tx, userID, userAgent, ip string) (Session, error) {
	token, err := security.NewSessionToken()
	if err != nil {
		return Session{}, err
	}
	expiresAt := time.Now().UTC().Add(s.ttl)
	_, err = tx.Exec(ctx, `
		INSERT INTO user_sessions (user_id, refresh_token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		userID, security.HashSessionToken(token), truncate(userAgent, 512), normalizedIP(ip), expiresAt,
	)
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	return Session{Token: token, ExpiresAt: expiresAt}, nil
}

func (s *Service) recordFailure(ctx context.Context, lookup []byte, ip string) error {
	_, err := s.db.Exec(ctx, `
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
