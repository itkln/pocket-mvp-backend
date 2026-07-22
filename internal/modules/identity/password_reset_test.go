package identity

import "testing"

func TestNormalizeResetLocale(t *testing.T) {
	for input, expected := range map[string]string{"ru": "ru", "UK": "uk", " sk ": "sk", "": "en", "de": "en"} {
		if actual := normalizeResetLocale(input); actual != expected {
			t.Fatalf("normalizeResetLocale(%q) = %q, want %q", input, actual, expected)
		}
	}
}
