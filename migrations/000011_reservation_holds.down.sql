ALTER TABLE reservations DROP CONSTRAINT IF EXISTS reservations_seated_slot_excl;
DROP INDEX IF EXISTS reservations_expired_holds_idx;
ALTER TABLE reservations DROP COLUMN IF EXISTS confirmation_hash;
ALTER TABLE reservations DROP COLUMN IF EXISTS hold_expires_at;
