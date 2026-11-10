DROP INDEX orders_customer_venue_history_idx;
DROP INDEX orders_guest_venue_history_idx;
ALTER TABLE orders DROP COLUMN table_qr_version;
ALTER TABLE orders DROP COLUMN table_identifier_snapshot;
ALTER TABLE orders DROP COLUMN guest_history_hash;
