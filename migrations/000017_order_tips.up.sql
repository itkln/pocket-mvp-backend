ALTER TABLE orders ADD COLUMN tip_minor integer NOT NULL DEFAULT 0 CHECK (tip_minor >= 0);
-- Existing tips keep their settled meaning. New checkout selections are explicitly pending.
ALTER TABLE tips ADD COLUMN status text NOT NULL DEFAULT 'paid' CHECK (status IN ('pending','paid','cancelled'));
ALTER TABLE tips ADD COLUMN idempotency_key text;
CREATE UNIQUE INDEX tips_request_unique ON tips(venue_id,idempotency_key) WHERE idempotency_key IS NOT NULL;
