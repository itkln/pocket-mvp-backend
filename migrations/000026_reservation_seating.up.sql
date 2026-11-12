-- Seating guests from the venue floor screen: when the party sat down, and parties seated
-- without a booking (walk-ins), which are reservations the staff create already seated.
ALTER TABLE reservations ADD COLUMN seated_at timestamptz;
ALTER TABLE reservations ADD COLUMN walk_in boolean NOT NULL DEFAULT false;

CREATE INDEX reservations_seated_idx ON reservations (venue_id, venue_table_id) WHERE status = 'seated';
