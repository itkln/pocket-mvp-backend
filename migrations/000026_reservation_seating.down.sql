DROP INDEX IF EXISTS reservations_seated_idx;
ALTER TABLE reservations DROP COLUMN IF EXISTS walk_in;
ALTER TABLE reservations DROP COLUMN IF EXISTS seated_at;
