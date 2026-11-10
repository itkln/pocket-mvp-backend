-- Restores the wider constraints. Deleted orders and their payments, former pickup
-- channels and former kitchen/viewer roles are not restored.
ALTER TABLE venue_staff DROP CONSTRAINT venue_staff_role_check;
ALTER TABLE venue_staff ADD CONSTRAINT venue_staff_role_check
  CHECK (role IN ('manager', 'waiter', 'kitchen', 'viewer'));

DROP INDEX IF EXISTS orders_active_queue_idx;
CREATE INDEX orders_active_queue_idx ON orders (venue_id, created_at, id)
  WHERE status IN ('new', 'accepted', 'preparing', 'ready', 'served');

CREATE INDEX order_items_kitchen_idx ON order_items (status, created_at)
  WHERE status IN ('new', 'preparing', 'ready');
ALTER TABLE order_items DROP CONSTRAINT order_items_status_check;
ALTER TABLE order_items ADD CONSTRAINT order_items_status_check
  CHECK (status IN ('new', 'preparing', 'ready', 'served', 'cancelled'));

ALTER TABLE orders DROP CONSTRAINT orders_channel_check;
ALTER TABLE orders ADD CONSTRAINT orders_channel_check
  CHECK (channel IN ('dine_in', 'online', 'pickup', 'preorder'));
ALTER TABLE orders DROP CONSTRAINT orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check
  CHECK (status IN ('draft', 'new', 'accepted', 'preparing', 'ready', 'served', 'completed', 'cancelled'));
