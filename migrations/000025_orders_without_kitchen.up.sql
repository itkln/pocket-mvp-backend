-- MVP without a kitchen (V-SRV-01, 2026-11-10): waiters take every order through
-- new -> preparing -> served -> completed, there is no pickup, and venue staff are
-- managers or waiters.

-- Orders in removed statuses or the removed channel are deleted. Their contribution
-- leaves open bills exactly as a cancellation would remove it.
CREATE TEMP TABLE removed_orders AS
  SELECT id, bill_id, status, subtotal_minor, tip_minor, total_minor
  FROM orders
  WHERE status IN ('accepted', 'ready') OR channel = 'pickup';

UPDATE bills b SET
  subtotal_minor = b.subtotal_minor - r.subtotal_minor,
  tip_minor = b.tip_minor - r.tip_minor,
  total_minor = b.total_minor - r.total_minor
FROM (
  SELECT bill_id, sum(subtotal_minor) AS subtotal_minor, sum(tip_minor) AS tip_minor,
         sum(total_minor) AS total_minor
  FROM removed_orders
  WHERE bill_id IS NOT NULL AND status NOT IN ('draft', 'cancelled')
  GROUP BY bill_id
) r
WHERE b.id = r.bill_id;

DELETE FROM order_status_events WHERE order_id IN (SELECT id FROM removed_orders);
DELETE FROM orders WHERE id IN (SELECT id FROM removed_orders);
DROP TABLE removed_orders;

ALTER TABLE orders DROP CONSTRAINT orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check
  CHECK (status IN ('draft', 'new', 'preparing', 'served', 'completed', 'cancelled'));
ALTER TABLE orders DROP CONSTRAINT orders_channel_check;
ALTER TABLE orders ADD CONSTRAINT orders_channel_check
  CHECK (channel IN ('dine_in', 'online', 'preorder'));

-- Order lines followed the kitchen's ready step; it no longer exists.
UPDATE order_items SET status = 'preparing' WHERE status = 'ready';
ALTER TABLE order_items DROP CONSTRAINT order_items_status_check;
ALTER TABLE order_items ADD CONSTRAINT order_items_status_check
  CHECK (status IN ('new', 'preparing', 'served', 'cancelled'));
DROP INDEX IF EXISTS order_items_kitchen_idx;

DROP INDEX IF EXISTS orders_active_queue_idx;
CREATE INDEX orders_active_queue_idx ON orders (venue_id, created_at, id)
  WHERE status IN ('new', 'preparing', 'served');

-- Kitchen and viewer staff keep their history but lose access until an owner
-- reactivates them as waiters or managers.
UPDATE venue_staff SET role = 'waiter', status = 'inactive', invite_token_hash = NULL
WHERE role IN ('kitchen', 'viewer');
ALTER TABLE venue_staff DROP CONSTRAINT venue_staff_role_check;
ALTER TABLE venue_staff ADD CONSTRAINT venue_staff_role_check
  CHECK (role IN ('manager', 'waiter'));
