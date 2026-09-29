ALTER TABLE reservations ADD COLUMN hold_expires_at timestamptz;
ALTER TABLE reservations ADD COLUMN confirmation_hash text;

CREATE INDEX reservations_expired_holds_idx ON reservations (hold_expires_at)
  WHERE status = 'pending' AND hold_expires_at IS NOT NULL;

-- A seated party still occupies the table if another booking is submitted concurrently.
ALTER TABLE reservations ADD CONSTRAINT reservations_seated_slot_excl EXCLUDE USING gist (
  venue_table_id WITH =,
  tstzrange(starts_at, ends_at, '[)') WITH &&
) WHERE (status IN ('pending', 'confirmed', 'seated'));
