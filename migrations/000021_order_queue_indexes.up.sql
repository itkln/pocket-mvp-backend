CREATE INDEX orders_active_queue_idx ON orders (venue_id, created_at, id)
 WHERE status IN ('new','accepted','preparing','ready','served');
CREATE INDEX orders_history_cursor_idx ON orders (venue_id, created_at DESC, id DESC)
 WHERE status IN ('completed','cancelled');
CREATE INDEX payments_order_guard_idx ON payments (order_id)
 WHERE status NOT IN ('failed','cancelled');
CREATE INDEX payments_bill_guard_idx ON payments (bill_id)
 WHERE status NOT IN ('failed','cancelled');
CREATE INDEX tips_order_paid_guard_idx ON tips (order_id) WHERE status='paid';
