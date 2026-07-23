package identity

import (
	"context"
	"time"
)

type Repository interface {
	Register(context.Context, newUserRecord, storedSession) (string, error)
	LoginBlocked(context.Context, []byte, string) (bool, error)
	FindCredentials(context.Context, []byte) (credentialRecord, bool, error)
	RecordLoginFailure(context.Context, []byte, string) error
	CompleteLogin(context.Context, []byte, string, storedSession) error
	FindUserBySession(context.Context, string) (encryptedUser, error)
	UpdateProfile(context.Context, string, encryptedProfile) (encryptedUser, error)
	RevokeSession(context.Context, string) error
	LoadUser(context.Context, string) (encryptedUser, error)
	PasswordHash(context.Context, string) (string, error)
	ChangeEmail(context.Context, string, string, []byte, string) error
	UpdateAvatar(context.Context, string, string, []byte) error
	Avatar(context.Context, string) (Avatar, error)
	ChangePassword(context.Context, string, string, string, string) error
	CreatePasswordReset(context.Context, string, string, string, time.Time, time.Duration) (bool, error)
	DeletePasswordReset(context.Context, string) error
	ResetPassword(context.Context, string, string) error
}

type newUserRecord struct {
	email        string
	emailLookup  []byte
	passwordHash string
	firstName    string
	lastName     string
	phone        *string
}

type storedSession struct {
	userID    string
	tokenHash string
	userAgent string
	ipAddress string
	expiresAt time.Time
}
