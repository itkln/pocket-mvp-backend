package identity

import (
	"context"
	"testing"
	"time"

	"pocket-mvp-backend/internal/security"
)

type registerRepositorySpy struct {
	Repository
	user    newUserRecord
	session storedSession
}

func (r *registerRepositorySpy) Register(_ context.Context, user newUserRecord, session storedSession) (string, error) {
	r.user = user
	r.session = session
	return "user-1", nil
}

type capabilityReaderStub struct{}

func (capabilityReaderStub) ListCapabilities(context.Context, string) ([]string, error) {
	return []string{"customer"}, nil
}

type resetSenderStub struct{}

func (resetSenderStub) SendPasswordReset(context.Context, string, string, string) error {
	return nil
}

func TestRegisterPassesEncryptedDataAndHashedCredentialsToRepository(t *testing.T) {
	protector, err := security.NewDataProtector(
		[]byte("0123456789abcdef0123456789abcdef"),
		[]byte("fedcba9876543210fedcba9876543210"),
	)
	if err != nil {
		t.Fatal(err)
	}
	repository := &registerRepositorySpy{}
	service, err := NewService(
		repository,
		protector,
		capabilityReaderStub{},
		time.Hour,
		PasswordResetOptions{Sender: resetSenderStub{}, BaseURL: "http://localhost:3000", TTL: time.Minute},
	)
	if err != nil {
		t.Fatal(err)
	}

	user, session, err := service.Register(context.Background(), RegisterInput{
		FirstName: " Denis ",
		LastName:  " Itkin ",
		Email:     " OWNER@example.com ",
		Phone:     " +421900123456 ",
		Password:  "a secure password",
		UserAgent: "test-agent",
		IPAddress: "127.0.0.1:1234",
	})
	if err != nil {
		t.Fatal(err)
	}

	if user.ID != "user-1" || user.Email != "owner@example.com" || session.Token == "" {
		t.Fatalf("unexpected registration result: user=%#v session=%#v", user, session)
	}
	if repository.user.email == user.Email || repository.user.firstName == user.FirstName {
		t.Fatal("personal data reached the repository without encryption")
	}
	decryptedEmail, err := protector.Decrypt(repository.user.email, "users.email")
	if err != nil || decryptedEmail != user.Email {
		t.Fatalf("stored email cannot be decrypted: value=%q err=%v", decryptedEmail, err)
	}
	passwordValid, err := security.VerifyPassword("a secure password", repository.user.passwordHash)
	if err != nil || !passwordValid {
		t.Fatalf("stored password hash is invalid: valid=%v err=%v", passwordValid, err)
	}
	if repository.session.tokenHash != security.HashSessionToken(session.Token) ||
		repository.session.ipAddress != "127.0.0.1" {
		t.Fatalf("unexpected stored session: %#v", repository.session)
	}
}
