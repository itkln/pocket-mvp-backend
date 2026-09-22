-- Owned by the Spots favorites context. IDs are references across domain
-- boundaries, deliberately without cross-domain foreign keys.
CREATE TABLE spots_saved_venues (
  user_id uuid NOT NULL,
  venue_id uuid NOT NULL,
  saved_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, venue_id)
);

CREATE INDEX spots_saved_venues_user_recent_idx
  ON spots_saved_venues (user_id, saved_at DESC, venue_id);
