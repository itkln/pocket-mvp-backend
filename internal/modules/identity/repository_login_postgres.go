package identity

// Sign-up and sign-in: new accounts, credentials and the login rate limit.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

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
