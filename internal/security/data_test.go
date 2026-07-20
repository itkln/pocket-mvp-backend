package security

import (
	"bytes"
	"testing"
)

func TestDataProtectorRoundTrip(t *testing.T) {
	protector, err := NewDataProtector(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatal(err)
	}
	first, err := protector.Encrypt("denis@example.com", "users.email")
	if err != nil {
		t.Fatal(err)
	}
	second, err := protector.Encrypt("denis@example.com", "users.email")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || first == "denis@example.com" {
		t.Fatal("encryption must use a random nonce and hide plaintext")
	}
	plaintext, err := protector.Decrypt(first, "users.email")
	if err != nil || plaintext != "denis@example.com" {
		t.Fatalf("unexpected decrypted value %q: %v", plaintext, err)
	}
}

func TestDataProtectorAuthenticatesFieldContext(t *testing.T) {
	protector, err := NewDataProtector(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := protector.Encrypt("value", "users.email")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := protector.Decrypt(encrypted, "users.phone"); err == nil {
		t.Fatal("expected different authenticated context to fail")
	}
}

func TestLookupIsNormalizedAndDeterministic(t *testing.T) {
	protector, err := NewDataProtector(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatal(err)
	}
	first := protector.Lookup(NormalizeEmail("  Denis@Example.com "))
	second := protector.Lookup(NormalizeEmail("denis@example.com"))
	if !bytes.Equal(first, second) {
		t.Fatal("normalized e-mail must have stable lookup digest")
	}
}

func TestSessionTokenIsRandomAndHasStableHash(t *testing.T) {
	first, err := NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || len(first) < 40 {
		t.Fatal("session tokens need sufficient random entropy")
	}
	if HashSessionToken(first) != HashSessionToken(first) || HashSessionToken(first) == first {
		t.Fatal("session hash must be deterministic and must not expose token")
	}
}
