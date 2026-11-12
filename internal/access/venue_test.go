package access

import (
	"context"
	"testing"
)

type venueAuthorizerContract interface {
	Require(context.Context, string, string, Capability) error
}

func TestVenueAuthorizerImplementsModuleContract(t *testing.T) {
	var _ venueAuthorizerContract = (*VenueAuthorizer)(nil)
}

func TestRoleCapabilities(t *testing.T) {
	for _, test := range []struct {
		role    string
		allowed []Capability
	}{
		{RoleOwner, []Capability{ManageOrders, SettleBills, SeatGuests, ManageMenu, ManageFloor, ManageSettings, ManageVenue}},
		{RoleManager, []Capability{ManageOrders, SettleBills, SeatGuests, ManageMenu, ManageFloor, ManageSettings}},
		{RoleWaiter, []Capability{ManageOrders, SettleBills, SeatGuests}},
		{"kitchen", nil},
		{"viewer", nil},
		{"", nil},
	} {
		for _, capability := range []Capability{ManageOrders, SettleBills, SeatGuests, ManageMenu, ManageFloor, ManageSettings, ManageVenue} {
			want := false
			for _, allowed := range test.allowed {
				want = want || allowed == capability
			}
			if Allows(test.role, capability) != want {
				t.Errorf("%q %s: want %v", test.role, capability, want)
			}
		}
	}
}
