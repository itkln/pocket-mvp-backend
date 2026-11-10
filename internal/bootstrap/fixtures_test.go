package bootstrap

import (
	"bytes"
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/money"
	"pocket-mvp-backend/internal/security"
)

// Fixtures for integration tests: each helper inserts one row with sensible defaults and fails
// the test on error. Tests spell out only what matters to them.

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// seedUser inserts an active account. accountRole may be empty to keep the column default.
func seedUser(t *testing.T, db *pgxpool.Pool, email, accountRole string) string {
	t.Helper()
	var id string
	must(t, db.QueryRow(context.Background(), `
		INSERT INTO users (email, password_hash, first_name, last_name, account_role)
		VALUES ($1, 'test-only', 'Test', 'User', COALESCE(NULLIF($2, ''), 'customer'))
		RETURNING id::text`, email, accountRole).Scan(&id))
	return id
}

// venueSeed describes a venue in Bratislava (SK, EUR). Empty fields take the defaults: the name
// is the slug, the status is active and the settings are empty.
type venueSeed struct {
	Slug     string
	Name     string
	Status   string
	Settings string // JSON object
}

func seedVenue(t *testing.T, db *pgxpool.Pool, ownerID string, venue venueSeed) string {
	t.Helper()
	if venue.Name == "" {
		venue.Name = venue.Slug
	}
	if venue.Status == "" {
		venue.Status = "active"
	}
	if venue.Settings == "" {
		venue.Settings = "{}"
	}
	var id string
	must(t, db.QueryRow(context.Background(), `
		INSERT INTO venues (owner_user_id, slug, name, address_line1, city, country_code, timezone, currency, status, settings)
		VALUES ($1, $2, $3, 'Main 1', 'Bratislava', 'SK', 'Europe/Bratislava', 'EUR', $4, $5::jsonb)
		RETURNING id::text`, ownerID, venue.Slug, venue.Name, venue.Status, venue.Settings).Scan(&id))
	return id
}

// seedStaff adds userID to the venue team with an active membership.
func seedStaff(t *testing.T, db *pgxpool.Pool, venueID, userID, email, role, invitedBy string) {
	t.Helper()
	_, err := db.Exec(context.Background(), `
		INSERT INTO venue_staff (venue_id, user_id, invited_email, role, status, invited_by_user_id)
		VALUES ($1, $2, $3, $4, 'active', $5)`, venueID, userID, email, role, invitedBy)
	must(t, err)
}

// seedMenuItem inserts a category with one available item and returns both IDs.
func seedMenuItem(t *testing.T, db *pgxpool.Pool, venueID, category string, preorder bool, item string, price money.Minor) (string, string) {
	t.Helper()
	ctx := context.Background()
	var categoryID, itemID string
	must(t, db.QueryRow(ctx, `
		INSERT INTO menu_categories (venue_id, name, preorder_enabled)
		VALUES ($1, $2, $3)
		RETURNING id::text`, venueID, category, preorder).Scan(&categoryID))
	must(t, db.QueryRow(ctx, `
		INSERT INTO menu_items (venue_id, category_id, name, price_minor)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text`, venueID, categoryID, item, price).Scan(&itemID))
	return categoryID, itemID
}

// testProtector encrypts personal data with fixed test-only keys.
func testProtector(t *testing.T) *security.DataProtector {
	t.Helper()
	protector, err := security.NewDataProtector(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	must(t, err)
	return protector
}
