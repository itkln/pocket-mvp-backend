package identity

import (
	"net"
	"net/mail"
	"strings"
	"unicode/utf8"
)

func validName(value string) bool {
	count := utf8.RuneCountInString(value)
	return count >= 1 && count <= 80
}

func validEmail(value string) bool {
	if len(value) > 254 || strings.ContainsAny(value, "\r\n") {
		return false
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}

func validPassword(value string) bool {
	length := utf8.RuneCountInString(value)
	return length >= 12 && length <= 128
}

func normalizedIP(value string) string {
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	if parsed := net.ParseIP(strings.TrimSpace(value)); parsed != nil {
		return parsed.String()
	}
	return "0.0.0.0"
}

func truncate(value string, length int) string {
	if len(value) <= length {
		return value
	}
	return value[:length]
}
