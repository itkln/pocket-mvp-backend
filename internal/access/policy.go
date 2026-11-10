package access

// Capability is something a venue role may do. Modules ask for a capability, never for a role,
// so the role model can change in one place.
type Capability string

const (
	// ManageOrders: the order queue, status changes and cancellation.
	ManageOrders Capability = "orders"
	// ManageMenu: menu categories and items, prices, availability (stop list), images, ordering.
	ManageMenu Capability = "menu"
	// ManageFloor: the floor plan and table QR codes.
	ManageFloor Capability = "floor"
	// ManageVenue: everything else — team, venue settings, reviews, payments, reports, subscription.
	ManageVenue Capability = "venue"
)

// Venue roles. The owner is resolved from the venue, the others from active staff membership.
const (
	RoleOwner   = "owner"
	RoleManager = "manager"
	RoleWaiter  = "waiter"
)

// MVP decision of 2026-11-10 (V-SRV-01, V-BE-02): waiters run orders; managers also run the menu
// and the floor; the owner can do everything.
var roleCapabilities = map[string][]Capability{
	RoleOwner:   {ManageOrders, ManageMenu, ManageFloor, ManageVenue},
	RoleManager: {ManageOrders, ManageMenu, ManageFloor},
	RoleWaiter:  {ManageOrders},
}

// Allows reports whether a venue role has a capability.
func Allows(role string, capability Capability) bool {
	for _, granted := range roleCapabilities[role] {
		if granted == capability {
			return true
		}
	}
	return false
}
