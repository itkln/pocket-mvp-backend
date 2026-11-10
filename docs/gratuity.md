# E-14 · Optional tips

Only Spots orders placed at a table support optional tips. Reservation preorders do not offer tips, and reservation create/hold/confirm reject positive tips.
The default is no tip. The guest can select 5%, 10%, 15%, or enter an exact amount
with up to two decimal places (comma or dot). The old demo checkout uses the same
selector; it remains a demo, not a payment provider integration.

Presets are expanded inline in table checkout and update the total immediately.
The zero option is labelled “Нет” (localized). Only the custom amount opens a
dialog, with Save and Close; closing discards unsaved edits. There is no Back button.

## Request contract

`POST /public/table-orders` accepts:

```json
{
  "tip": { "mode": "percent", "percent": 10 },
  "expected_total_minor": 1320
}
```

Alternatives: `{"mode":"none"}` or `{"mode":"custom","amount_minor":123}`.
Legacy requests omitting both fields retain zero tips and their existing request
hash. Percent is an integer from 0 to 100; custom amounts cannot be negative.
Contradictory fields, unsupported modes and amounts exceeding the safe order
limit (2,000,000,000 minor units including tips) are rejected.

The server re-reads menu prices and calculates the tip on the aggregate dish
subtotal in minor units. Percentage rounding is half up, once per order:
`(subtotal_minor * percent + 50) / 100`. Thus 5% of 10.90 EUR is 0.55 EUR.
`expected_total_minor`, when present, must match the server's resulting total.
A changed price or grand total requires another explicit confirmation.

## Persistence and receipts

Migration 17 adds `orders.tip_minor` and tip lifecycle/idempotency fields.
The order, tip row and table bill totals are written in the same transaction.
A zero tip creates no tip row. Concurrent retries return the same order and
cannot increment the bill or create a second tip row. Changing the confirmed
payload with the same key returns a conflict. Reservation confirmation has the
same guarantees, including hold replay and saved confirmation lookup.

Order APIs expose `subtotal_minor`, `tip_minor`, `total_minor`. Reservation APIs
expose `preorder_subtotal_minor`, `preorder_tip_minor`, `preorder_total_minor`;
Vendor order read models include tips in their totals and expose the separate
tip amount. Reservation read models retain these fields for historical records;
new reservation preorders have zero tips. Receipts show tips only when positive.
Table drafts retain the selection on reload. Repeating an old order transfers
dishes, not its old tips: tips default to zero again.

New tip rows have `status=pending`; they do not represent settled money. Existing
historical tip rows keep their paid meaning. Cancelling an eligible unpaid order
cancels its pending tips and removes its contribution from the open table bill
exactly once. A payment in progress blocks modifying that bill. Cancelling an
eligible reservation also cancels any historical pending preorder tips.

## Remaining E-13 dependency

There is no live payment intent, provider webhook or payment settlement flow in
this MVP yet. Current orders/preorders are payable at the venue. E-14's provider
acceptance criterion remains pending E-13: create the payment using the stored
`orders.total_minor` / `bills.total_minor`, never a client-supplied amount, link
pending tips to the confirmed payment, and mark them paid only on verified,
idempotent settlement. Provider failures, refunds, distribution and staff
allocation must follow the agreed P-13 rules; no distribution promise is shown.

## Verification

- Backend unit tests: integer rounding, zero/custom tips, contradictory input,
  bounds and overflow.
- Isolated PostgreSQL integration: percentage/custom/zero, confirmed-total and
  menu-price changes, eight concurrent retries, bill/read-model totals,
  cancellation replay/locked bill, reservation hold/confirmation and direct
  creation replay, customer receipts and cancellation.
- Frontend tests: decimal parsing, invalid amounts, defaults, quantity changes,
  absence of tips in mobile/desktop reservation checkout, inline presets,
  custom-only dialog, exact table retry payload and receipt row.
