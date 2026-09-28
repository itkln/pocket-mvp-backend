DROP INDEX IF EXISTS reservations_idempotency_unique;
ALTER TABLE reservations DROP COLUMN IF EXISTS request_hash;
ALTER TABLE reservations DROP COLUMN IF EXISTS idempotency_key;
