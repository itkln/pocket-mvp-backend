package identity

import (
	"time"

	"pocket-mvp-backend/internal/security"
)

type encryptedRegistration struct {
	email     string
	firstName string
	lastName  string
	phone     *string
}

type encryptedUser struct {
	id              string
	email           string
	firstName       string
	lastName        string
	phone           *string
	role            string
	avatarUpdatedAt *time.Time
}

type credentialRecord struct {
	user         encryptedUser
	passwordHash string
}

type encryptedProfile struct {
	firstName string
	lastName  string
	phone     *string
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

func (s *Service) encryptProfile(input UpdateProfileInput) (encryptedProfile, error) {
	firstName, err := s.protector.Encrypt(input.FirstName, "users.first_name")
	if err != nil {
		return encryptedProfile{}, err
	}
	lastName, err := s.protector.Encrypt(input.LastName, "users.last_name")
	if err != nil {
		return encryptedProfile{}, err
	}
	var phone *string
	if input.Phone != "" {
		value, encryptErr := s.protector.Encrypt(input.Phone, "users.phone")
		if encryptErr != nil {
			return encryptedProfile{}, encryptErr
		}
		phone = &value
	}
	return encryptedProfile{firstName: firstName, lastName: lastName, phone: phone}, nil
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
		Phone: phone, Role: record.role, AvatarVersion: avatarVersion(record.avatarUpdatedAt),
	}, nil
}

func avatarVersion(updatedAt *time.Time) int64 {
	if updatedAt == nil {
		return 0
	}
	return updatedAt.UnixMilli()
}

func (s *Service) newSession(userID, userAgent, ip string) (Session, storedSession, error) {
	token, err := security.NewSessionToken()
	if err != nil {
		return Session{}, storedSession{}, err
	}
	expiresAt := time.Now().UTC().Add(s.ttl)
	return Session{Token: token, ExpiresAt: expiresAt}, storedSession{
		userID:    userID,
		tokenHash: security.HashSessionToken(token),
		userAgent: truncate(userAgent, 512),
		ipAddress: normalizedIP(ip),
		expiresAt: expiresAt,
	}, nil
}
