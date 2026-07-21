package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/security"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTooManyAttempts    = errors.New("too many login attempts")
	ErrUnauthorized       = errors.New("unauthorized")
)

type RegisterInput struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
	Password  string
	UserAgent string
	IPAddress string
}

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IPAddress string
}

type User struct {
	ID           string   `json:"id"`
	Email        string   `json:"email"`
	FirstName    string   `json:"first_name"`
	LastName     string   `json:"last_name"`
	Phone        string   `json:"phone,omitempty"`
	Role         string   `json:"role"`
	Capabilities []string `json:"capabilities"`
}

type Session struct {
	Token     string
	ExpiresAt time.Time
}

type Service struct {
	db        *pgxpool.Pool
	protector *security.DataProtector
	ttl       time.Duration
	dummyHash string
}

func NewService(db *pgxpool.Pool, protector *security.DataProtector, ttl time.Duration) (*Service, error) {
	dummyHash, err := security.HashPassword("dummy-password-never-used")
	if err != nil {
		return nil, err
	}
	return &Service{db: db, protector: protector, ttl: ttl, dummyHash: dummyHash}, nil
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (User, Session, error) {
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Email = security.NormalizeEmail(input.Email)
	if !validName(input.FirstName) || !validName(input.LastName) || !validEmail(input.Email) || !validPassword(input.Password) || utf8.RuneCountInString(input.Phone) > 40 {
		return User{}, Session{}, ErrInvalidInput
	}

	passwordHash, err := security.HashPassword(input.Password)
	if err != nil {
		return User{}, Session{}, fmt.Errorf("hash password: %w", err)
	}
	emailEncrypted, err := s.protector.Encrypt(input.Email, "users.email")
	if err != nil {
		return User{}, Session{}, err
	}
	firstNameEncrypted, err := s.protector.Encrypt(input.FirstName, "users.first_name")
	if err != nil {
		return User{}, Session{}, err
	}
	lastNameEncrypted, err := s.protector.Encrypt(input.LastName, "users.last_name")
	if err != nil {
		return User{}, Session{}, err
	}
	var phoneEncrypted *string
	if input.Phone != "" {
		value, encryptErr := s.protector.Encrypt(input.Phone, "users.phone")
		if encryptErr != nil {
			return User{}, Session{}, encryptErr
		}
		phoneEncrypted = &value
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, Session{}, fmt.Errorf("begin registration: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, email_lookup, password_hash, first_name, last_name, phone, account_role)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text`,
		emailEncrypted, s.protector.Lookup(input.Email), passwordHash, firstNameEncrypted, lastNameEncrypted, phoneEncrypted, "customer",
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, Session{}, ErrEmailAlreadyExists
		}
		return User{}, Session{}, fmt.Errorf("insert user: %w", err)
	}

	session, err := s.createSession(ctx, tx, id, input.UserAgent, input.IPAddress)
	if err != nil {
		return User{}, Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, Session{}, fmt.Errorf("commit registration: %w", err)
	}
	return User{ID: id, Email: input.Email, FirstName: input.FirstName, LastName: input.LastName, Phone: input.Phone, Role: "customer", Capabilities: []string{"customer"}}, session, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (User, Session, error) {
	email := security.NormalizeEmail(input.Email)
	if !validEmail(email) || len(input.Password) == 0 || len(input.Password) > 128 {
		return User{}, Session{}, ErrInvalidCredentials
	}
	lookup := s.protector.Lookup(email)
	ip := normalizedIP(input.IPAddress)

	var blocked bool
	err := s.db.QueryRow(ctx, `SELECT COALESCE(blocked_until > now(), false) FROM auth_login_attempts WHERE email_lookup = $1 AND ip_address = $2`, lookup, ip).Scan(&blocked)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return User{}, Session{}, fmt.Errorf("check login attempts: %w", err)
	}
	if blocked {
		return User{}, Session{}, ErrTooManyAttempts
	}

	var id, emailEncrypted, firstNameEncrypted, lastNameEncrypted, passwordHash, role string
	var phoneEncrypted *string
	err = s.db.QueryRow(ctx, `
		SELECT id::text, email, first_name, last_name, phone, password_hash, account_role
		FROM users
		WHERE email_lookup = $1 AND deleted_at IS NULL AND status = 'active'`, lookup,
	).Scan(&id, &emailEncrypted, &firstNameEncrypted, &lastNameEncrypted, &phoneEncrypted, &passwordHash, &role)
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return User{}, Session{}, fmt.Errorf("find user: %w", err)
	}
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
	user, err := s.decryptUser(id, emailEncrypted, firstNameEncrypted, lastNameEncrypted, phoneEncrypted, role)
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
	session, err := s.createSession(ctx, tx, id, input.UserAgent, ip)
	if err != nil {
		return User{}, Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, Session{}, err
	}
	user.Capabilities, err = s.capabilities(ctx, user.ID)
	if err != nil {
		return User{}, Session{}, err
	}
	return user, session, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthorized
	}
	var id, emailEncrypted, firstNameEncrypted, lastNameEncrypted, role string
	var phoneEncrypted *string
	err := s.db.QueryRow(ctx, `
		SELECT u.id::text, u.email, u.first_name, u.last_name, u.phone, u.account_role
		FROM user_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.refresh_token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now()
		  AND u.status = 'active' AND u.deleted_at IS NULL`, security.HashSessionToken(token),
	).Scan(&id, &emailEncrypted, &firstNameEncrypted, &lastNameEncrypted, &phoneEncrypted, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, fmt.Errorf("authenticate session: %w", err)
	}
	user, err := s.decryptUser(id, emailEncrypted, firstNameEncrypted, lastNameEncrypted, phoneEncrypted, role)
	if err != nil {
		return User{}, err
	}
	user.Capabilities, err = s.capabilities(ctx, user.ID)
	return user, err
}

func (s *Service) capabilities(ctx context.Context, userID string) ([]string, error) {
	capabilities := []string{"customer"}
	var ownsVenue, worksAtVenue bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM venues WHERE owner_user_id = $1 AND deleted_at IS NULL)`, userID).Scan(&ownsVenue); err != nil {
		return nil, fmt.Errorf("load owner capability: %w", err)
	}
	if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM venue_staff WHERE user_id = $1 AND status = 'active')`, userID).Scan(&worksAtVenue); err != nil {
		return nil, fmt.Errorf("load staff capability: %w", err)
	}
	if ownsVenue {
		capabilities = append(capabilities, "owner")
	}
	if worksAtVenue {
		capabilities = append(capabilities, "staff")
	}
	return capabilities, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.Exec(ctx, `UPDATE user_sessions SET revoked_at = now() WHERE refresh_token_hash = $1 AND revoked_at IS NULL`, security.HashSessionToken(token))
	return err
}

func (s *Service) createSession(ctx context.Context, tx pgx.Tx, userID, userAgent, ip string) (Session, error) {
	token, err := security.NewSessionToken()
	if err != nil {
		return Session{}, err
	}
	expiresAt := time.Now().UTC().Add(s.ttl)
	_, err = tx.Exec(ctx, `INSERT INTO user_sessions (user_id, refresh_token_hash, user_agent, ip_address, expires_at) VALUES ($1, $2, $3, $4, $5)`,
		userID, security.HashSessionToken(token), truncate(userAgent, 512), normalizedIP(ip), expiresAt)
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
		  failure_count = CASE WHEN auth_login_attempts.window_started_at < now() - interval '15 minutes' THEN 1 ELSE auth_login_attempts.failure_count + 1 END,
		  window_started_at = CASE WHEN auth_login_attempts.window_started_at < now() - interval '15 minutes' THEN now() ELSE auth_login_attempts.window_started_at END,
		  blocked_until = CASE WHEN auth_login_attempts.window_started_at >= now() - interval '15 minutes' AND auth_login_attempts.failure_count + 1 >= 5 THEN now() + interval '15 minutes' ELSE NULL END`, lookup, ip)
	if err != nil {
		return fmt.Errorf("record login failure: %w", err)
	}
	return nil
}

func (s *Service) decryptUser(id, emailEncrypted, firstNameEncrypted, lastNameEncrypted string, phoneEncrypted *string, role string) (User, error) {
	email, err := s.protector.Decrypt(emailEncrypted, "users.email")
	if err != nil {
		return User{}, err
	}
	firstName, err := s.protector.Decrypt(firstNameEncrypted, "users.first_name")
	if err != nil {
		return User{}, err
	}
	lastName, err := s.protector.Decrypt(lastNameEncrypted, "users.last_name")
	if err != nil {
		return User{}, err
	}
	phone := ""
	if phoneEncrypted != nil {
		phone, err = s.protector.Decrypt(*phoneEncrypted, "users.phone")
		if err != nil {
			return User{}, err
		}
	}
	return User{ID: id, Email: email, FirstName: firstName, LastName: lastName, Phone: phone, Role: role}, nil
}

func validName(value string) bool {
	count := utf8.RuneCountInString(value)
	return count >= 1 && count <= 80
}

func validEmail(value string) bool {
	if len(value) > 254 || strings.ContainsAny(value, "\r\n") {
		return false
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}

func validPassword(value string) bool {
	length := utf8.RuneCountInString(value)
	return length >= 12 && length <= 128
}

func normalizedIP(value string) string {
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	if parsed := net.ParseIP(strings.TrimSpace(value)); parsed != nil {
		return parsed.String()
	}
	return "0.0.0.0"
}

func truncate(value string, length int) string {
	if len(value) <= length {
		return value
	}
	return value[:length]
}
