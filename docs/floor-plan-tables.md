# Floor plan and venue tables (V-10)

The Vendor editor saves the floor plan as JSON (`venues.settings.floor_plan`). Bookings, bills,
orders, QR codes, Spots availability and the dashboard work with `venue_tables` rows. Saving the
plan (`PUT /venues/{id}/floor-plan`) updates both in one transaction (`tablesync.Plan`), so they
never disagree.

## Rules

- A table is keyed by its number (`tables[].id` in the plan = `venue_tables.identifier`).
  Its row ID is returned to the editor as `tables[].table_id` on load and save; it is the ID Spots
  and bookings use. A `table_id` sent by the client is ignored and never stored in the plan.
- Each floor is an active `venue_layouts` row, matched by name. Floors missing from the plan are
  deactivated, so their tables disappear from Spots.
- Seats, shape, position and floor of an existing table are updated in place; the row ID stays.
- A table missing from the plan is soft-deleted (`deleted_at`). A table that comes back under the
  same number gets its old row (same ID, same QR code).
- **Removing a table guests still need is refused** with `409 table_in_use`, and nothing of that
  save is applied. A table is in use when it has an upcoming confirmed booking, a pending booking
  with a live hold, a party seated within the last 24 hours, an open or payment-pending bill, or a
  table order that is not yet served.
- The plan is validated: an array of floors, table numbers unique and non-empty (≤ 32 characters),
  1–50 seats; otherwise `422 invalid_input`.
- Dashboard: `total_tables` counts live, not disabled tables; `active_tables` counts those with a
  seated party or an open bill (no longer the static `status` column).

Migration `000027` backfills existing venues: their plan floors become active layouts, every plan
table gets a row, and rows absent from the plan are soft-deleted when nothing depends on them.
The lazy per-table sync (`tablesync.Table`, used by QR and seating) stays for plans written
directly to the settings.
