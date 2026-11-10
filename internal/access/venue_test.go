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
		{RoleOwner, []Capability{ManageOrders, ManageMenu, ManageFloor, ManageVenue}},
		{RoleManager, []Capability{ManageOrders, ManageMenu, ManageFloor}},
		{RoleWaiter, []Capability{ManageOrders}},
		{"kitchen", nil},
		{"viewer", nil},
		{"", nil},
	} {
		for _, capability := range []Capability{ManageOrders, ManageMenu, ManageFloor, ManageVenue} {
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
