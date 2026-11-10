DROP INDEX venue_staff_invite_token_idx;
ALTER TABLE venue_staff DROP COLUMN invite_token_hash, DROP COLUMN invite_expires_at, DROP COLUMN invite_sent_at;
