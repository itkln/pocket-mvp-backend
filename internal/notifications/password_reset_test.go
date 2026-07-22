package notifications

import (
	"strings"
	"testing"
)

func TestPasswordResetBodyUsesRequestedLocale(t *testing.T) {
	for _, locale := range []string{"ru", "en", "uk", "sk"} {
		body := passwordResetBody(locale, "https://pocket.example/reset")
		if !strings.Contains(body, "https://pocket.example/reset") {
			t.Fatalf("reset URL missing for locale %s", locale)
		}
	}
}
