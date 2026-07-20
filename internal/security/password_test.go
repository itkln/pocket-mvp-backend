package security

import (
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("a long and safe password")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected Argon2id hash, got %q", hash)
	}
	valid, err := VerifyPassword("a long and safe password", hash)
	if err != nil || !valid {
		t.Fatalf("expected password to verify: valid=%v err=%v", valid, err)
	}
	valid, err = VerifyPassword("wrong password", hash)
	if err != nil || valid {
		t.Fatalf("expected wrong password to fail: valid=%v err=%v", valid, err)
	}
}

func TestPasswordHashUsesUniqueSalt(t *testing.T) {
	first, err := HashPassword("same password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword("same password")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("password hashes must use unique salts")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	if _, err := VerifyPassword("password", "$argon2id$broken"); err == nil {
		t.Fatal("expected malformed hash to be rejected")
	}
}
