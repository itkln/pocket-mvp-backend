-- Reservation receipt, customer history and cancellation all locate preorders
-- by reservation. Other orders do not need entries in this partial index.
CREATE INDEX orders_reservation_preorder_idx
  ON orders (reservation_id, created_at, id)
  WHERE channel = 'preorder' AND reservation_id IS NOT NULL;
