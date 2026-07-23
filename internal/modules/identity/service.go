package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

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
	repository   Repository
	protector    *security.DataProtector
	capabilities CapabilityReader
	ttl          time.Duration
	dummyHash    string
	resetSender  PasswordResetSender
	resetBaseURL string
	resetTTL     time.Duration
}

func NewService(repository Repository, protector *security.DataProtector, capabilities CapabilityReader, ttl time.Duration, reset PasswordResetOptions) (*Service, error) {
	dummyHash, err := security.HashPassword("dummy-password-never-used")
	if err != nil {
		return nil, err
	}
	if repository == nil || protector == nil || capabilities == nil ||
		reset.Sender == nil || strings.TrimSpace(reset.BaseURL) == "" || ttl <= 0 || reset.TTL <= 0 {
		return nil, errors.New("identity service configuration is incomplete")
	}
	return &Service{
		repository:   repository,
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
	session, stored, err := s.newSession("", input.UserAgent, input.IPAddress)
	if err != nil {
		return User{}, Session{}, err
	}
	userID, err := s.repository.Register(ctx, newUserRecord{
		email:        encrypted.email,
		emailLookup:  s.protector.Lookup(input.Email),
		passwordHash: passwordHash,
		firstName:    encrypted.firstName,
		lastName:     encrypted.lastName,
		phone:        encrypted.phone,
	}, stored)
	if err != nil {
		return User{}, Session{}, err
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

	blocked, err := s.repository.LoginBlocked(ctx, lookup, ip)
	if err != nil {
		return User{}, Session{}, err
	}
	if blocked {
		return User{}, Session{}, ErrTooManyAttempts
	}

	record, found, err := s.repository.FindCredentials(ctx, lookup)
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
		if err := s.repository.RecordLoginFailure(ctx, lookup, ip); err != nil {
			return User{}, Session{}, err
		}
		return User{}, Session{}, ErrInvalidCredentials
	}

	user, err := s.decryptUser(record.user)
	if err != nil {
		return User{}, Session{}, err
	}
	session, stored, err := s.newSession(user.ID, input.UserAgent, ip)
	if err != nil {
		return User{}, Session{}, err
	}
	if err := s.repository.CompleteLogin(ctx, lookup, ip, stored); err != nil {
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
	record, err := s.repository.FindUserBySession(ctx, security.HashSessionToken(token))
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

func (s *Service) UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) (User, error) {
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Phone = strings.TrimSpace(input.Phone)
	if userID == "" || !validProfileUpdate(input) {
		return User{}, ErrInvalidInput
	}

	encrypted, err := s.encryptProfile(input)
	if err != nil {
		return User{}, fmt.Errorf("encrypt profile: %w", err)
	}

	record, err := s.repository.UpdateProfile(ctx, userID, encrypted)
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

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repository.RevokeSession(ctx, security.HashSessionToken(token))
}

func validRegistration(input RegisterInput) bool {
	return validName(input.FirstName) && validName(input.LastName) && validEmail(input.Email) &&
		validPassword(input.Password) && utf8.RuneCountInString(input.Phone) <= 40
}

func validProfileUpdate(input UpdateProfileInput) bool {
	return validName(input.FirstName) && validName(input.LastName) && utf8.RuneCountInString(input.Phone) <= 40
}
