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
	if !validProfileUpdate(UpdateProfileInput{FirstName: "Denis", LastName: "Itkin", Phone: "+421 900 123 456"}) {
		t.Fatal("valid profile update was rejected")
	}
	if validProfileUpdate(UpdateProfileInput{FirstName: "", LastName: "Itkin"}) {
		t.Fatal("profile update must require a first name")
	}
}
