ALTER TABLE orders ADD COLUMN guest_history_hash text;
ALTER TABLE orders ADD COLUMN table_identifier_snapshot text;
ALTER TABLE orders ADD COLUMN table_qr_version integer;
UPDATE orders o SET table_identifier_snapshot = t.identifier
FROM venue_tables t WHERE o.venue_table_id = t.id AND o.channel = 'dine_in';
CREATE INDEX orders_guest_venue_history_idx ON orders (venue_id, guest_history_hash, created_at DESC, id DESC)
WHERE channel = 'dine_in' AND customer_user_id IS NULL AND guest_history_hash IS NOT NULL;
CREATE INDEX orders_customer_venue_history_idx ON orders (venue_id, customer_user_id, created_at DESC, id DESC)
WHERE channel = 'dine_in' AND customer_user_id IS NOT NULL;
