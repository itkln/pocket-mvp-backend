package ordering

import (
	"encoding/json"
	"errors"
	"pocket-mvp-backend/internal/appfault"
	"strings"
	"testing"
)

func TestTransitionMatrix(t *testing.T) {
	statuses := []string{"new", "accepted", "preparing", "ready", "served", "completed", "cancelled"}
	for _, channel := range []string{"dine_in", "preorder", "pickup"} {
		for _, role := range []string{"owner", "manager", "waiter", "kitchen", "viewer"} {
			for _, from := range statuses {
				for _, to := range statuses {
					authorized := role == "owner" || role == "manager" || role == "waiter" && (to == "accepted" || to == "served") || role == "kitchen" && (to == "preparing" || to == "ready")
					edge := from == "new" && to == "accepted" || from == "accepted" && to == "preparing" || from == "preparing" && to == "ready" || from == "ready" && ((channel == "pickup" && to == "completed") || (channel != "pickup" && to == "served")) || from == "served" && to == "completed"
					want := authorized && (from == to || from != "completed" && from != "cancelled" && (edge || to == "cancelled"))
					err := validateTransition(role, channel, from, to, from)
					if (err == nil) != want {
						t.Fatalf("%s %s %s→%s: %v", role, channel, from, to, err)
					}
				}
			}
		}
	}
}
func TestStaleAndReplay(t *testing.T) {
	err := validateTransition("owner", "dine_in", "preparing", "ready", "accepted")
	var conflict *StatusConflict
	if !errors.As(err, &conflict) || conflict.CurrentStatus != "preparing" {
		t.Fatal(err)
	}
	if err := validateTransition("kitchen", "dine_in", "ready", "ready", "preparing"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(validateTransition("viewer", "dine_in", "new", "new", "new"), appfault.ErrForbidden) {
		t.Fatal("viewer mutation allowed")
	}
}
func TestKitchenJSON(t *testing.T) {
	order := Order{GuestName: "PRIVATE NAME", GuestPhone: "PRIVATE PHONE", TotalMinor: 12345, TipMinor: 42, Currency: "EUR", Items: []OrderItem{{Name: "Soup", Quantity: 2, UnitPriceMinor: 100}}, kitchenOnly: true}
	raw, err := json.Marshal(order)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"guest_name", "guest_phone", "total_minor", "tip_minor", "currency", "unit_price_minor", "PRIVATE"} {
		if strings.Contains(string(raw), key) {
			t.Fatalf("leaked %s: %s", key, raw)
		}
	}
	if !strings.Contains(string(raw), "Soup") {
		t.Fatal("missing dish")
	}
}
