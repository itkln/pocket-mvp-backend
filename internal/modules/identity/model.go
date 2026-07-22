package identity

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTooManyAttempts    = errors.New("too many login attempts")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidResetToken  = errors.New("invalid or expired password reset token")
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

type PasswordResetRequest struct {
	Email     string
	Locale    string
	IPAddress string
}

type PasswordResetConfirmation struct {
	Token    string
	Password string
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
