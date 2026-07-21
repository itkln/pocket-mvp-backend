package identity

import "testing"

func TestIdentityValidation(t *testing.T) {
	if !validEmail("owner@example.com") || validEmail("Owner <owner@example.com>") {
		t.Fatal("email validation must accept only normalized address values")
	}
	if validPassword("too-short") || !validPassword("a secure password") {
		t.Fatal("password length policy is not enforced")
	}
	if got := normalizedIP("127.0.0.1:8080"); got != "127.0.0.1" {
		t.Fatalf("unexpected normalized IP: %s", got)
	}
}
