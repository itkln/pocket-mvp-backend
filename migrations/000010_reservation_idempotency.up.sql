ALTER TABLE reservations ADD COLUMN idempotency_key text;
ALTER TABLE reservations ADD COLUMN request_hash text;
CREATE UNIQUE INDEX reservations_idempotency_unique
  ON reservations (venue_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
