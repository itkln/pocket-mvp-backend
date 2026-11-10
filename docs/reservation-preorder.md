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

## E-15 / E-27: cancelling an account reservation

`POST /api/v1/customer/reservations/{reservationID}/cancel` requires the current
account session. Ownership is taken from the session; foreign, missing and
unfinished hold IDs all return `404`. Malformed IDs return `422`.

The booking list and cancellation response include `cancellation` (`allowed`,
optional `reason` and `deadline`) and `preorder_status`. The client uses these
server decisions and asks for confirmation in the existing details panel.
A successful cancellation returns the complete updated reservation. Cancelled
bookings and their original preorder snapshots remain visible in history.

The transaction locks the reservation, linked preorder orders, bills and payments
before rechecking eligibility with the database clock. It cancels the reservation,
preorder orders and order items together, releasing the table exclusion constraint
and recording one `reservation.cancelled_by_customer` audit event. Repeat requests
return the same cancelled booking. Vendor status updates cannot revive cancelled
orders. An error rolls the whole transaction back.

### Approved MVP rule

The user approved the default on 2026-10-09: cancellation is allowed until the
visit starts, provided preorder preparation has not started. This applies to
existing and new venues with an absent or null cancellation policy:
`cutoff_minutes = 0`, `preorder_until = preparing`. Draft, new, accepted and
already cancelled preorders allow cancellation; preparing, ready, served and
completed preorders block it. Cancellation at or after the visit start is blocked.

The policy mechanism supports `venues.settings.guest_cancellation`:

- `cutoff_minutes`: nonnegative whole minutes before the reservation starts
  (0 means until the visit starts; maximum 43200).
- `preorder_until`: `accepted`, `preparing`, or `visit` — the first two block
  cancellation at that stage and later; `visit` uses only the visit deadline.

Explicit venue settings override this default. Invalid explicit settings fail
closed with `cancellation_policy_unset`. There is currently no Vendor UI for
editing this policy.

Any payment state other than `failed` or `cancelled`, or a paid/payment-pending
bill, blocks cancellation with `cancellation_payment_started`. This MVP does not
capture, cancel provider payments or issue refunds; paid bookings require a
separate refund workflow.

Other denial codes: `cancellation_deadline_passed`, `cancellation_status_blocked`,
`cancellation_preorder_locked`. Denied operations return `409` and leave all
reservation, order, item and payment records intact.

### Verification

`internal/bootstrap/reservation_cancellation_integration_test.go` runs only on an
isolated database whose name ends in `_test`, creating and dropping its own schema.
It uses a venue without an explicit policy to verify the approved default and
covers ownership, holds, deadlines, venue overrides, plain bookings, preorder
cancellation, payment/preparation rejection, transaction rollback, concurrent
retries and preparation, slot reuse, Vendor visibility and history after reload.
Frontend tests cover confirmation dismissal, pending state, duplicate clicks,
translated denial messages, retry and history updates.
