ALTER TABLE venue_staff
 ADD COLUMN invite_token_hash text,
 ADD COLUMN invite_expires_at timestamptz,
 ADD COLUMN invite_sent_at timestamptz;
CREATE UNIQUE INDEX venue_staff_invite_token_idx ON venue_staff(invite_token_hash) WHERE invite_token_hash IS NOT NULL;
