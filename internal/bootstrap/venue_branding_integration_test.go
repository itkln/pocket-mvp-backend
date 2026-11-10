package bootstrap

import (
	"context"
	"errors"
	"testing"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/venues"
)

func TestVenueBrandingIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	ownerID := seedUser(t, db, "branding@example.test", "venue_owner")
	service := venues.NewService(venues.NewPostgresRepository(db), access.NewVenueAuthorizer(db))
	input := venues.Input{Name: "First Bistro", Address: "Main 1", City: "Bratislava", CountryCode: "SK", Timezone: "Europe/Bratislava", Currency: "EUR", Status: "active", Settings: map[string]any{"cover_image_url": "https://example.test/cover.jpg", "preorder_enabled": true}}
	first, err := service.Create(ctx, ownerID, input)
	if err != nil {
		t.Fatal(err)
	}
	input.Name = "Second Cafe"
	input.Settings = map[string]any{"logo_image_url": "https://example.test/cafe.png"}
	second, err := service.Create(ctx, ownerID, input)
	if err != nil {
		t.Fatal(err)
	}
	input.Name = first.Name
	input.Settings = map[string]any{"logo_image_url": "data:image/webp;base64,dGVzdA=="}
	updated, err := service.Update(ctx, ownerID, first.ID, input)
	if err != nil || updated.Settings["cover_image_url"] != "https://example.test/cover.jpg" {
		t.Fatalf("branding update must preserve existing settings: %v, %v", updated, err)
	}
	for id, logo := range map[string]string{first.ID: "data:image/webp;base64,dGVzdA==", second.ID: "https://example.test/cafe.png"} {
		detail, err := service.GetPublished(ctx, id)
		if err != nil || detail.LogoURL != logo {
			t.Fatalf("venue %s branding: %v, %v", id, detail, err)
		}
	}
	if _, err := service.Update(ctx, "00000000-0000-0000-0000-000000000001", first.ID, input); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("another owner may not change branding: %v", err)
	}
	input.Settings = map[string]any{"logo_image_url": ""}
	if _, err := service.Update(ctx, ownerID, first.ID, input); err != nil {
		t.Fatal(err)
	}
	detail, err := service.GetPublished(ctx, first.ID)
	if err != nil || detail.LogoURL != "" || detail.ImageURL != "https://example.test/cover.jpg" || !detail.PreorderEnabled {
		t.Fatalf("removing a logo must keep cover and preorder settings: %v, %v", detail, err)
	}
}
