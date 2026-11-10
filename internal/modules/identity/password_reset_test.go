package identity

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"pocket-mvp-backend/internal/security"
)

func TestNormalizeResetLocale(t *testing.T) {
	for input, expected := range map[string]string{"ru": "ru", "UA": "ua", "UK": "uk", " sk ": "sk", "": "en", "de": "en"} {
		if actual := normalizeResetLocale(input); actual != expected {
			t.Fatalf("normalizeResetLocale(%q) = %q, want %q", input, actual, expected)
		}
	}
}

type resetRepositorySpy struct {
	Repository
	found     bool
	created   bool
	lookups   int
	tokenHash string
	expiresAt time.Time
	deleted   string
}

func (r *resetRepositorySpy) FindCredentials(context.Context, []byte) (credentialRecord, bool, error) {
	r.lookups++
	return credentialRecord{user: encryptedUser{id: "guest"}}, r.found, nil
}

func (r *resetRepositorySpy) CreatePasswordReset(_ context.Context, _, tokenHash, _ string, expiresAt time.Time, _ time.Duration) (bool, error) {
	r.tokenHash, r.expiresAt = tokenHash, expiresAt
	return r.created, nil
}

func (r *resetRepositorySpy) DeletePasswordReset(_ context.Context, hash string) error {
	r.deleted = hash
	return nil
}

type resetSenderSpy struct {
	url, email, locale string
	err                error
}

func (s *resetSenderSpy) SendPasswordReset(_ context.Context, email, resetURL, locale string) error {
	s.email, s.url, s.locale = email, resetURL, locale
	return s.err
}

func TestRequestPasswordResetSelectsConfiguredProductOrigin(t *testing.T) {
	protector, err := security.NewDataProtector([]byte("0123456789abcdef0123456789abcdef"), []byte("fedcba9876543210fedcba9876543210"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ product, host string }{
		{"dish", "dish.example"}, {"vendor", "vendor.example"}, {"", "vendor.example"},
	} {
		t.Run("product="+tc.product, func(t *testing.T) {
			repo := &resetRepositorySpy{found: true, created: true}
			sender := &resetSenderSpy{}
			svc, err := NewService(repo, protector, capabilityReaderStub{}, time.Hour, PasswordResetOptions{
				Sender: sender, BaseURL: "https://vendor.example/", DishBaseURL: "https://dish.example/", TTL: 30 * time.Minute,
			})
			if err != nil {
				t.Fatal(err)
			}
			before := time.Now()
			if err := svc.RequestPasswordReset(context.Background(), PasswordResetRequest{Email: " GUEST@example.com ", Locale: "sk", Product: tc.product, IPAddress: "127.0.0.1"}); err != nil {
				t.Fatal(err)
			}
			link, err := url.Parse(sender.url)
			if err != nil {
				t.Fatal(err)
			}
			if link.Scheme != "https" || link.Host != tc.host || link.Path != "/sk/reset-password" || sender.email != "guest@example.com" || sender.locale != "sk" {
				t.Fatalf("unexpected destination: %s", sender.url)
			}
			token := link.Query().Get("token")
			if len(token) < 32 || repo.tokenHash != security.HashSessionToken(token) || repo.tokenHash == token {
				t.Fatal("token must be random and stored only as a hash")
			}
			if repo.expiresAt.Before(before.Add(30*time.Minute)) || repo.expiresAt.After(time.Now().Add(30*time.Minute)) {
				t.Fatal("configured expiry not applied")
			}
			for _, invalid := range []string{"https://evil.example", "//evil.example", "spots", "eats", "unknown"} {
				beforeLookups := repo.lookups
				if err := svc.RequestPasswordReset(context.Background(), PasswordResetRequest{Email: "guest@example.com", Product: invalid}); !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("accepted product %q", invalid)
				}
				if repo.lookups != beforeLookups {
					t.Fatal("invalid product must fail before account lookup")
				}
			}
			sender.url = ""
			repo.found = false
			if err := svc.RequestPasswordReset(context.Background(), PasswordResetRequest{Email: "unknown@example.com", Product: tc.product}); err != nil || sender.url != "" {
				t.Fatal("unknown account must silently skip delivery")
			}
			repo.found, repo.created = true, false
			if err := svc.RequestPasswordReset(context.Background(), PasswordResetRequest{Email: "guest@example.com", Product: tc.product}); err != nil || sender.url != "" {
				t.Fatal("cooldown must silently skip delivery")
			}
			repo.created = true
			sender.err = errors.New("SMTP unavailable")
			if err := svc.RequestPasswordReset(context.Background(), PasswordResetRequest{Email: "guest@example.com", Product: tc.product}); err == nil || repo.deleted != repo.tokenHash {
				t.Fatal("failed delivery must invalidate the pending token")
			}
		})
	}
}

func TestPasswordResetBaseURLRejectsUntrustedURLShapes(t *testing.T) {
	for _, value := range []string{"", "//dish.example", "javascript:alert(1)", "https://user:pass@dish.example", "https://dish.example/path", "https://dish.example?next=x", "https://dish.example?", "https://dish.example/#token", "https:///path"} {
		if _, err := passwordResetBaseURL(value); err == nil {
			t.Errorf("accepted invalid origin %q", value)
		}
	}
	for _, value := range []string{"https://dish.example", "https://dish.example/", "http://localhost:3001", "http://127.0.0.1:3001"} {
		if _, err := passwordResetBaseURL(value); err != nil {
			t.Errorf("rejected configured origin %q: %v", value, err)
		}
	}
}
