package demodata

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/security"
)

func (s *Seeder) seedOwner(ctx context.Context, tx pgx.Tx) (string, error) {
	passwordHash, err := security.HashPassword(s.password)
	if err != nil {
		return "", fmt.Errorf("hash demo password: %w", err)
	}
	email, err := s.protector.Encrypt(s.email, "users.email")
	if err != nil {
		return "", fmt.Errorf("encrypt demo email: %w", err)
	}
	firstName, err := s.protector.Encrypt("Демо", "users.first_name")
	if err != nil {
		return "", fmt.Errorf("encrypt demo first name: %w", err)
	}
	lastName, err := s.protector.Encrypt("Владелец", "users.last_name")
	if err != nil {
		return "", fmt.Errorf("encrypt demo last name: %w", err)
	}
	phone, err := s.protector.Encrypt("+421 910 555 000", "users.phone")
	if err != nil {
		return "", fmt.Errorf("encrypt demo phone: %w", err)
	}
	lookup := s.protector.Lookup(security.NormalizeEmail(s.email))

	var ownerID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE email_lookup = $1 AND deleted_at IS NULL`, lookup).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			INSERT INTO users (
				id, email, email_lookup, password_hash, first_name, last_name, phone,
				account_role, status, email_verified_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'customer', 'active', now())
			RETURNING id::text`,
			demoOwnerID, email, lookup, passwordHash, firstName, lastName, phone,
		).Scan(&ownerID)
	} else if err == nil {
		_, err = tx.Exec(ctx, `
			UPDATE users
			SET email = $2, password_hash = $3, first_name = $4, last_name = $5,
			    phone = $6, account_role = 'customer', status = 'active',
			    email_verified_at = now(), deleted_at = NULL
			WHERE id = $1`, ownerID, email, passwordHash, firstName, lastName, phone)
	}
	if err != nil {
		return "", fmt.Errorf("upsert demo owner: %w", err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM auth_login_attempts WHERE email_lookup = $1`, lookup); err != nil {
		return "", fmt.Errorf("clear demo login attempts: %w", err)
	}
	return ownerID, nil
}
