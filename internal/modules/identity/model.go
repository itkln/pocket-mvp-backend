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
