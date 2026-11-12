DROP INDEX IF EXISTS bills_payment_requested_idx;
ALTER TABLE payments DROP COLUMN IF EXISTS recorded_by_user_id, DROP COLUMN IF EXISTS method;
ALTER TABLE bills DROP COLUMN IF EXISTS payment_requested_at;
