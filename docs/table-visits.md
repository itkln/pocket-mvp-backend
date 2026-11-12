# V-TBL-01: table visit and shared bill

A visit is an existing `bills` row, not a new parallel entity. Lifecycle:
`open` → `payment_pending` → `paid`; `void` terminates an abandoned visit.
Failed payment may return `payment_pending` to `open` in the future payment workflow.
`open` and `payment_pending` are active; the partial unique index permits only one
active bill per table. Settling a visit must set `closed_at`, retain orders and
must not update kitchen statuses. Staff settlement and acquiring belong to
V-TBL-05/06; this task does not introduce an unauthenticated close endpoint.

## Creation and concurrency

QR resolution and reading the bill never create a visit. The first valid order
creates it; subsequent orders from any phone reuse it. The existing transaction
locks the resolved table, then the active bill, and writes order snapshots,
items and totals atomically. Invalid menu/price requests create no empty bill.
The existing idempotency key returns the original receipt even after settlement;
a new key after settlement creates a new visit. `payment_pending` or an active
payment locks new orders. Keep this locking protocol when adding settlement.

## API

`POST /api/v1/public/table-bill` with JSON `{ "token": "<QR capability>" }`.
Returns `{ "bill": null }` when no active visit exists, otherwise a bill with
id, venue_id, table_id, status, currency, opened_at, items, subtotal_minor,
tip_minor, tax_minor, service_fee_minor and total_minor. Each item includes its
order ID/number, saved name, quantity, unit_price_minor and total_minor.
All money is integer minor units. The item total snapshot is authoritative and
supports saved modifiers; current menu prices are never queried. Cancelled
orders, draft orders and cancelled items are omitted. Tips from cancelled or
fully cancelled orders are omitted. Tax and service fee are bill-level fields;
current QR orders do not set these. The read is computed, not based on the
cached bill subtotal, so item cancellation cannot leave a stale guest total.
Future payment code must use the same calculation while locking settlement,
not trust cached bills.total_minor for charging after partial item cancellation.

The QR token is the capability for the shared current table bill. Client-supplied
bill/table/venue IDs cannot select another table. Rotated, disabled and removed
QRs fail the existing resolution checks. A repeatable-read snapshot keeps QR,
visit and items consistent during concurrent updates. No personal identity,
private notes or past visit is returned. Anyone retaining a valid static QR
can read the current bill; per-visit presence verification is outside this MVP.
Responses use Cache-Control: no-store. The per-device order-history API is
unchanged; the shared bill deliberately includes all guests' active items.

## Verification

Run `./scripts/test-integration.sh ./internal/bootstrap ./internal/httpapi ./internal/modules/dinein`.
`TestTableVisitIntegration` checks empty reads, eight concurrent orders sharing
one bill, independent HTTP reads, rejected substituted table IDs, saved prices,
order/item cancellations, another table's isolation, paid visit exclusion,
a fresh visit after settlement, and invalid/revoked QR rejection.
The QR landing page is V-TBL-02.

## Guest bill screen (V-TBL-04)

Dish `/{locale}/qr/{token}/bill` renders this endpoint only; it never adds up the local cart or
one device's orders. It refreshes every 15 s while visible, on returning to the page, when the
device goes back online and on «Обновить счёт», and shows the time of the last update. A failed
refresh keeps the last bill on screen with a «no connection» notice and a retry; a revoked QR
removes the bill. When the visit is settled while the guest watches, the screen says «Счёт закрыт»
and offers the menu (a new order opens a new visit). `payment_pending` shows that the bill is
being settled. Tests:
`frontend/features/pocket/customer/table-entry.test.tsx`.
