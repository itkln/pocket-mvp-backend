// Package validate holds the input checks the modules share, so a UUID or a colour means the same
// everywhere. Domain rules (lengths, ranges) stay in the modules.
package validate

import (
	"regexp"
	"slices"
	"time"
)

var (
	uuidPattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	hexColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

// UUID reports whether value is a UUID in its canonical text form (any version, as PostgreSQL
// accepts). Checking before a query turns malformed IDs into 422/404 instead of SQL errors.
func UUID(value string) bool { return uuidPattern.MatchString(value) }

// Date reports whether value is a calendar date written as YYYY-MM-DD.
func Date(value string) bool {
	_, err := time.Parse(time.DateOnly, value)
	return err == nil && len(value) == len(time.DateOnly)
}

// HexColor reports whether value is a colour written as #RRGGBB.
func HexColor(value string) bool { return hexColorPattern.MatchString(value) }

// OneOf reports whether value is one of the allowed values.
func OneOf(value string, allowed ...string) bool { return slices.Contains(allowed, value) }
