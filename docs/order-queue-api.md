# Vendor order queue API (V-ORD-02)

All routes require an authenticated account session. Membership is resolved on the
server for the venue in the URL. Both prefixes use the same ordering service:

- `/api/v1/owner/venues/{venueID}/orders`
- `/api/v1/staff/venues/{venueID}/orders`

## List

`GET {prefix}?scope=active&status=new,accepted&channel=dine_in&table_id={uuid}&limit=50`

| Parameter | Contract |
| --- | --- |
| `scope` | `active` (default) or `history` |
| `status` | Optional comma-separated statuses; repeated parameters also supported |
| `channel` | Optional `dine_in`, `preorder`, `online`, `pickup`; the latter two preserve existing data compatibility, not new MVP flows |
| `table_id` | Optional UUID of the table, not its display number |
| `limit` | Integer 1–100; default 50 |
| `cursor` | Opaque `next_cursor` from the previous response, used with the same filters and venue |

Active: `new`, `accepted`, `preparing`, `ready`, `served`.
History: `completed`, `cancelled`. Drafts are excluded from both lists and details.
Statuses outside the selected scope are invalid (422), not silently mixed in.

```json
{
  "orders": [],
  "has_more": false
}
```

Nonterminal pages include `next_cursor`. Follow it until `has_more=false`;
there is no total 200-record cap. Active order is `(created_at ASC, id ASC)` so
older pending work remains visible; history is `(created_at DESC, id DESC)`.
UUID tie-breaking makes pagination deterministic even for equal timestamps.
Cursors are versioned, validated and bound to normalized filters/venue. They are
not credentials; every page repeats access checks. Changing the page size is
allowed. Malformed, unsupported or filter-mismatched cursors return 422.

This is a live queue, not a database snapshot spanning requests. Orders may move
between active/history while paging. Refresh from the first page to reconcile
status changes and newly inserted records; deduplicate by order ID. No offsets
are used, so inserts before the cursor do not shift subsequent boundaries.

## Detail and order shape

`GET {prefix}/{orderID}` returns `{"order": {...}}`. Lookup is always scoped by
both venue ID and order ID. Unknown, foreign-venue or draft orders return 404.

Shared operational fields:

- `id`, `number`, `channel`, `source`, `status`, `notes`;
- `table_id`, `table_label`, `reservation_id`, `reservation_starts_at`;
- `created_at`, `updated_at`, `placed_at`, `completed_at`;
- `allowed_actions`: array of target statuses the current role can request now;
- `items`: ordered immutable purchase snapshots, including `id`, `name`,
  `quantity`, item `notes` and item `status`.

Owner, manager, waiter and viewer also receive:

- `currency`, `total_minor`, `tip_minor`, `bill_id`;
- `guest_name`, `guest_phone`, `guest_count`, `reservation_code`;
- item `unit_price_minor` and `total_minor` (integer minor currency units).

Kitchen responses omit financial fields, bill IDs, guest names/contact details
and reservation codes on list, detail and PATCH. Table and reservation UUIDs,
visit time, dish instructions and quantities remain available for preparation.
Optional associations can be absent; `placed_at` and `completed_at` are nullable.
All timestamps are RFC 3339. `updated_at` is the latest row update, not a promised
cooking deadline; we do not invent ETA, payment status or historical per-stage
timestamps that were never recorded.

## Actions and authorization

`allowed_actions` reuses the V-ORD-01 transition/role policy. It excludes no-op
replays and forbidden transitions. Final orders and viewer accounts get `[]`.
Cancellation is excluded when a linked bill is not open, there is a nonfailed,
noncancelled payment, or paid tips. A cancelled reservation disallows progress.
Actions are advisory: `PATCH {prefix}/{orderID}` still checks current membership,
locks the order/bill, validates the transition and requires `expected_status`.
The PATCH response uses the same detailed role-specific order contract.

- Owner/manager: legal progress and cancellation.
- Waiter: accept and serve when those are the next legal steps.
- Kitchen: start preparing and mark ready.
- Viewer: read-only.
- Inactive, removed and unrelated staff: 404 for list and detail; no access.

Missing authentication returns 401. Invalid filters/UUIDs return 422. A forbidden
mutation returns 403; a stale or invalid transition returns 409 with
`current_status`. Client-selected roles never affect authorization.

## Persistence and cost

Migration `000021_order_queue_indexes` adds partial active/history cursor indexes
and payment/tip guard indexes; it changes no existing order data. Every nonempty
page uses one order query plus one batched `ANY(uuid[])` query for its items,
independently of the number of orders. Authorization adds its own venue/membership
lookup. Details use the same projection, guards and item loader.

The Vendor screen uses the new contract, loads every active page, and initially
loads 100 history orders. “Show more” increases the history window by 100; refresh
preserves that requested depth. It respects server-provided actions. Staff routes
are ready for the separate staff-screen integration; this task does not replace
the existing staff UI/demo data.

## Verification

- `TestOrderQueueIntegration`: 231 active + 251 history orders, duplicate timestamps,
  complete cursors, combined filters, empty results, malformed/filter-mismatched
  cursors, role projections, foreign venues, drafts, disabled membership, payment
  cancellation guards, and two data queries for a 100-order page.
- `TestTableQRIntegration` and `TestReservationPreorderIntegration`: detailed
  readback of orders created through real QR and reservation services.
- Existing V-ORD-01 policy tests cover concurrent mutations and replay.
- Frontend tests cover page traversal beyond 200 active orders, lazy history,
  cross-scope deduplication, filter encoding, actions and history loading.
