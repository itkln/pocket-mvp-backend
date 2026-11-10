package demodata

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func seedWorkforce(ctx context.Context, tx pgx.Tx, ownerID, venueID string) error {
	for _, member := range workforceMembers(venueID) {
		acceptedAt := any(nil)
		if member.Status == "active" {
			acceptedAt = "now()"
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO venue_staff (
				id, venue_id, display_name, invited_email, role, status,
				invited_by_user_id, invited_at, accepted_at
			)
			VALUES (
				$1, $2, $3, $4, $5, $6, $7,
				now() - interval '30 days',
				CASE WHEN $8::text IS NULL THEN NULL ELSE now() - interval '28 days' END
			)`,
			member.ID, venueID, member.Name, member.Email, member.Role, member.Status, ownerID, acceptedAt,
		)
		if err != nil {
			return fmt.Errorf("seed staff member %s: %w", member.Name, err)
		}
	}
	return nil
}

func workforceMembers(venueID string) []staffSeed {
	if venueID == secondaryVenueID {
		return []staffSeed{
			{ID: "50000000-0000-4000-8000-000000000101", Name: "Нина Белова", Email: "nina@pocket.local", Role: "manager", Status: "active"},
			{ID: "50000000-0000-4000-8000-000000000102", Name: "Мартин Ковач", Email: "martin@pocket.local", Role: "waiter", Status: "active"},
		}
	}
	return []staffSeed{
		{ID: "50000000-0000-4000-8000-000000000001", Name: "София Марек", Email: "sofia@pocket.local", Role: "waiter", Status: "active"},
		{ID: "50000000-0000-4000-8000-000000000002", Name: "Адам Ковач", Email: "adam@pocket.local", Role: "manager", Status: "active"},
		{ID: "50000000-0000-4000-8000-000000000003", Name: "Лукас Новак", Email: "lukas@pocket.local", Role: "waiter", Status: "inactive"},
		{ID: "50000000-0000-4000-8000-000000000004", Name: "Эма Варга", Email: "ema@pocket.local", Role: "waiter", Status: "invited"},
		{ID: "50000000-0000-4000-8000-000000000005", Name: "Петер Грубер", Email: "peter@pocket.local", Role: "waiter", Status: "active"},
	}
}
