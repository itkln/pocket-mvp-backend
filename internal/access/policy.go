package access

// Capability is something a venue role may do. Modules ask for a capability, never for a role,
// so the role model can change in one place.
type Capability string

const (
	// ManageOrders: the order queue, status changes and cancellation.
	ManageOrders Capability = "orders"
	// SettleBills: table bills — the guests' calls for the bill and confirming that a visit was paid
	// in person (cash or card). Every operational role settles; there is no kitchen role since 000025.
	SettleBills Capability = "bills"
	// SeatGuests: the live floor — who sits where, today's bookings, seating booked and walk-in
	// guests, freeing a table — and reading the floor plan it is drawn on (V-SRV-02).
	SeatGuests Capability = "seating"
	// ManageMenu: menu categories and items, prices, availability (stop list), images, ordering.
	ManageMenu Capability = "menu"
	// ManageFloor: editing the floor plan and the table QR codes.
	ManageFloor Capability = "floor"
	// ManageSettings: the venue profile — name, contacts, address, hours, cover, logo, publication.
	ManageSettings Capability = "settings"
	// ManageVenue: everything else — team, reviews, payments, reports, subscription, deleting the venue.
	ManageVenue Capability = "venue"
)

// Venue roles. The owner is resolved from the venue, the others from active staff membership.
const (
	RoleOwner   = "owner"
	RoleManager = "manager"
	RoleWaiter  = "waiter"
)

// MVP decision of 2026-11-10 (V-SRV-01, V-BE-02), revised 2026-11-12: waiters run orders, settle
// table bills and seat guests on the live floor; managers also run the menu,
// the floor and the venue settings; the owner can do everything.
var roleCapabilities = map[string][]Capability{
	RoleOwner:   {ManageOrders, SettleBills, SeatGuests, ManageMenu, ManageFloor, ManageSettings, ManageVenue},
	RoleManager: {ManageOrders, SettleBills, SeatGuests, ManageMenu, ManageFloor, ManageSettings},
	RoleWaiter:  {ManageOrders, SettleBills, SeatGuests},
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
