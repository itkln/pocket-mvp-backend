# V-TBL-01: table visit and shared bill

A visit is an existing `bills` row, not a new parallel entity. Lifecycle:
`open` → `payment_pending` → `paid`; `void` terminates an abandoned visit.
Failed payment may return `payment_pending` to `open` in the future payment workflow.
`open` and `payment_pending` are active; the partial unique index permits only one
active bill per table. Settling a visit must set `closed_at`, retain orders and
must not update kitchen statuses. Staff settlement and acquiring belong to
V-TBL-05 (below) and V-TBL-06; there is no unauthenticated close endpoint.

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
being settled. Until online payment (V-TBL-06), paying in cash calls the staff (below). Tests:
`frontend/features/pocket/customer/table-entry.test.tsx`.

## Calling the staff and settling in person (V-TBL-05)

**Guest.** Choosing «Оплата наличными» and «Оплатить {amount}» calls
`POST /api/v1/public/table-bill/call-staff {token}` (Apple Pay, Google Pay, card and «Разделить
оплату» are a design mock-up until V-TBL-06/08). It sets
`bills.payment_requested_at` once (repeated calls, from any phone, keep the first time and write
one `bill.payment_requested` audit row) and returns the bill. Without an active visit it answers
`404 not_found`. The call is **not a payment**: the visit stays `open`, guests may still order,
and nothing is closed if the guest leaves the page. The bill shows «Сотрудник вызван» with the
call time until a staff member confirms; then the bill disappears and the screen says
«Оплата подтверждена» with a current bill of 0.

**Staff (Vendor, «Заказы»/«Сервис»).** Capability `bills` (`access.SettleBills`): owner,
manager and waiter. There is no kitchen role (removed in `000025`), so the kitchen cannot settle.
- `GET /api/v1/venues/{venueID}/bills` — active table visits, called ones first, each with
  its dishes and amounts computed like the guest's bill. Vendor polls it every 10 s while
  visible, rings and shows «Стол N просит рассчитать» for a new call; all open bills are one
  click away.
- `POST /api/v1/venues/{venueID}/bills/{billID}/settle {expected_total_minor, method}` with
  `method` `cash` or `card`. In one transaction it locks the bill row, recomputes the amount and
  refuses `409 bill_changed` if it differs from what the staff member checked (Vendor reloads the
  bill and requires «Сумма сверена» before confirming again). Otherwise it inserts a `payments`
  row (`provider='staff'`, `provider_payment_id`=bill ID, `status='paid'`, amount, currency,
  `method`, `recorded_by_user_id`, `paid_at`), sets the bill `paid` with `closed_at`, brings the
  cached totals in line and writes a `bill.settled` audit row. Orders and kitchen statuses are
  not touched. A visit whose every dish was cancelled closes as `void` without a payment.
- A repeated confirmation returns the recorded settlement (same amount, method and time) and
  writes nothing; the unique `(provider, provider_payment_id)` also allows one staff payment per
  bill. `payment_pending` or an online payment in progress → `409 bill_payment_in_progress`; a
  visit closed some other way → `409 bill_closed`.

**Order vs. settlement race.** Order creation and settlement both lock the bill row. An order
committed first changes the amount (settlement refused, check again); an order arriving after
the settlement finds no open bill and opens a new visit. An order is never added to a closing bill.

Known limit: staff cancelling an item does not lock the bill, so a cancellation committed in the
same instant as a settlement may not be reflected in the amount paid.

Migration `000029_table_bill_settlement`. Tests:
`internal/bootstrap/bill_settlement_integration_test.go` (`TestTableBillSettlementIntegration`),
`frontend/features/pocket/owner/table-bills.test.tsx`, `table-entry.test.tsx`.
