package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/security"
)

type CapabilityReader interface {
	ListCapabilities(context.Context, string) ([]string, error)
}

type PasswordResetSender interface {
	SendPasswordReset(context.Context, string, string, string) error
}

type PasswordResetOptions struct {
	Sender  PasswordResetSender
	BaseURL string
	TTL     time.Duration
}

type Service struct {
	db           *pgxpool.Pool
	protector    *security.DataProtector
	capabilities CapabilityReader
	ttl          time.Duration
	dummyHash    string
	resetSender  PasswordResetSender
	resetBaseURL string
	resetTTL     time.Duration
}

func NewService(db *pgxpool.Pool, protector *security.DataProtector, capabilities CapabilityReader, ttl time.Duration, reset PasswordResetOptions) (*Service, error) {
	dummyHash, err := security.HashPassword("dummy-password-never-used")
	if err != nil {
		return nil, err
	}
	if reset.Sender == nil || strings.TrimSpace(reset.BaseURL) == "" || reset.TTL <= 0 {
		return nil, errors.New("password reset configuration is incomplete")
	}
	return &Service{
		db:           db,
		protector:    protector,
		capabilities: capabilities,
		ttl:          ttl,
		dummyHash:    dummyHash,
		resetSender:  reset.Sender,
		resetBaseURL: strings.TrimRight(reset.BaseURL, "/"),
		resetTTL:     reset.TTL,
	}, nil
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (User, Session, error) {
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Email = security.NormalizeEmail(input.Email)
	if !validRegistration(input) {
		return User{}, Session{}, ErrInvalidInput
	}

	passwordHash, err := security.HashPassword(input.Password)
	if err != nil {
		return User{}, Session{}, fmt.Errorf("hash password: %w", err)
	}
	encrypted, err := s.encryptRegistration(input)
	if err != nil {
		return User{}, Session{}, err
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, Session{}, fmt.Errorf("begin registration: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, email_lookup, password_hash, first_name, last_name, phone, account_role)
		VALUES ($1, $2, $3, $4, $5, $6, 'customer')
		RETURNING id::text`,
		encrypted.email, s.protector.Lookup(input.Email), passwordHash,
		encrypted.firstName, encrypted.lastName, encrypted.phone,
	).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, Session{}, ErrEmailAlreadyExists
		}
		return User{}, Session{}, fmt.Errorf("insert user: %w", err)
	}

	session, err := s.createSession(ctx, tx, userID, input.UserAgent, input.IPAddress)
	if err != nil {
		return User{}, Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, Session{}, fmt.Errorf("commit registration: %w", err)
	}
	return User{
		ID: userID, Email: input.Email, FirstName: input.FirstName, LastName: input.LastName,
		Phone: input.Phone, Role: "customer", Capabilities: []string{"customer"},
	}, session, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (User, Session, error) {
	email := security.NormalizeEmail(input.Email)
	if !validEmail(email) || len(input.Password) == 0 || len(input.Password) > 128 {
		return User{}, Session{}, ErrInvalidCredentials
	}
	lookup := s.protector.Lookup(email)
	ip := normalizedIP(input.IPAddress)

	blocked, err := s.loginBlocked(ctx, lookup, ip)
	if err != nil {
		return User{}, Session{}, err
	}
	if blocked {
		return User{}, Session{}, ErrTooManyAttempts
	}

	record, found, err := s.findUserByLookup(ctx, lookup)
	if err != nil {
		return User{}, Session{}, err
	}
	passwordHash := record.passwordHash
	if !found {
		passwordHash = s.dummyHash
	}
	verified, verifyErr := security.VerifyPassword(input.Password, passwordHash)
	if verifyErr != nil {
		verified = false
	}
	if !found || !verified {
		if err := s.recordFailure(ctx, lookup, ip); err != nil {
			return User{}, Session{}, err
		}
		return User{}, Session{}, ErrInvalidCredentials
	}

	user, err := s.decryptUser(record.user)
	if err != nil {
		return User{}, Session{}, err
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, Session{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM auth_login_attempts WHERE email_lookup = $1 AND ip_address = $2`, lookup, ip); err != nil {
		return User{}, Session{}, fmt.Errorf("clear login attempts: %w", err)
	}
	session, err := s.createSession(ctx, tx, user.ID, input.UserAgent, ip)
	if err != nil {
		return User{}, Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, Session{}, err
	}
	user.Capabilities, err = s.capabilities.ListCapabilities(ctx, user.ID)
	if err != nil {
		return User{}, Session{}, err
	}
	return user, session, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthorized
	}
	var record encryptedUser
	err := s.db.QueryRow(ctx, `
		SELECT u.id::text, u.email, u.first_name, u.last_name, u.phone, u.account_role
		FROM user_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.refresh_token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now()
		  AND u.status = 'active' AND u.deleted_at IS NULL`, security.HashSessionToken(token),
	).Scan(
		&record.id, &record.email, &record.firstName, &record.lastName,
		&record.phone, &record.role,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, fmt.Errorf("authenticate session: %w", err)
	}
	user, err := s.decryptUser(record)
	if err != nil {
		return User{}, err
	}
	user.Capabilities, err = s.capabilities.ListCapabilities(ctx, user.ID)
	return user, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.Exec(ctx, `
		UPDATE user_sessions
		SET revoked_at = now()
		WHERE refresh_token_hash = $1 AND revoked_at IS NULL`, security.HashSessionToken(token))
	return err
}

func validRegistration(input RegisterInput) bool {
	return validName(input.FirstName) && validName(input.LastName) && validEmail(input.Email) &&
		validPassword(input.Password) && utf8.RuneCountInString(input.Phone) <= 40
}
