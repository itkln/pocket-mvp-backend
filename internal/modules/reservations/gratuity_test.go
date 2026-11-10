package reservations

import (
	"context"
	"errors"
	"testing"
	"time"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/gratuity"
)

func TestReservationTipsAreTableOnly(t *testing.T) {
	const venueID = "20000000-0000-4000-8000-000000000001"
	input := CreateInput{StartsAt: time.Now().Add(48 * time.Hour), Guests: 2, GuestName: "Test Guest", GuestPhone: "+421900000000", IdempotencyKey: "table-only-tips-test", Items: []PreorderItem{{MenuItemID: "30000000-0000-4000-8000-000000000001", Quantity: 1, ExpectedPriceMinor: 1200}}}
	// Nil dependencies ensure forbidden tips fail before any availability or database work.
	service := NewService(nil, nil)
	for _, tip := range []*gratuity.Selection{{Mode: "percent", Percent: 5}, {Mode: "custom", AmountMinor: 1}} {
		input.Tip = tip
		if _, err := service.Create(context.Background(), venueID, input); !errors.Is(err, appfault.ErrInvalidInput) {
			t.Fatalf("create accepted a tip: %v", err)
		}
		if _, err := service.Hold(context.Background(), venueID, input); !errors.Is(err, appfault.ErrInvalidInput) {
			t.Fatalf("hold accepted a tip: %v", err)
		}
		if _, err := service.Confirm(context.Background(), venueID, venueID, input); !errors.Is(err, appfault.ErrInvalidInput) {
			t.Fatalf("confirm accepted a tip: %v", err)
		}
	}
	for _, tip := range []*gratuity.Selection{nil, {Mode: "none"}, {Mode: "custom", AmountMinor: 0}, {Mode: "percent", Percent: 0}} {
		input.Tip = tip
		if _, err := normalizeAndValidate(venueID, input); err != nil {
			t.Fatalf("no-tip reservation rejected: %v", err)
		}
	}
}
