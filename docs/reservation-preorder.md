# Reservation preorder (E-10)

Spots creates a reservation and its preorder together via
`POST /api/v1/public/venues/{venueID}/reservations`, or confirms an existing
temporary hold with the same contact and item payload. Each item supplies
`menu_item_id`, `quantity` and `expected_price_minor`.

The server reads current prices and eligibility, then saves the reservation,
`reservation_preorder_items`, the linked `orders` row (`channel = preorder`)
and its items in one transaction. Names and prices are snapshots. An
idempotent retry returns the same booking and preorder number. Empty items
create an ordinary booking without an order. No online payment is collected.

The venue's `settings.preorder_enabled` is the global switch. Each category
also has `preorder_enabled`, editable through the existing owner category
create/update API. It defaults to true for new and existing categories;
omitting it in an update preserves the current value. Category publication
(`is_active`) remains independent.

The public menu returns `preorder_enabled` on each category. Ordinary menu
pages display every published category; desktop and mobile preorder pickers
display only enabled categories. Both direct booking and hold confirmation
reject dishes from disabled categories, hidden categories, unavailable or
deleted items, and other venues with `item_unavailable` (409).
`price_changed` (409) requires checking the refreshed total and confirming
again. Failed validation creates no preorder or confirmed reservation.

Vendor's venue-scoped orders list includes the reservation code, date/time,
guest contacts/count and saved dish quantities/prices. Changing the menu or
category settings does not modify previously saved reservations.

Database acceptance coverage: `TestReservationPreorderIntegration` and
`TestPublicMenuIntegration`, run with an isolated `TEST_DATABASE_URL` ending
in `_test`.
