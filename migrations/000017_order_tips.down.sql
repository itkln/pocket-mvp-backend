DROP INDEX tips_request_unique;
ALTER TABLE tips DROP COLUMN idempotency_key;
ALTER TABLE tips DROP COLUMN status;
ALTER TABLE orders DROP COLUMN tip_minor;
