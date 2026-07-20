CREATE TABLE bills (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  venue_id uuid NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
  venue_table_id uuid REFERENCES venue_tables(id),
  assigned_waiter_staff_id uuid REFERENCES venue_staff(id) ON DELETE SET NULL,
  status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'payment_pending', 'paid', 'void')),
  currency char(3) NOT NULL DEFAULT 'EUR' CHECK (currency ~ '^[A-Z]{3}$'),
  subtotal_minor integer NOT NULL DEFAULT 0 CHECK (subtotal_minor >= 0),
  tax_minor integer NOT NULL DEFAULT 0 CHECK (tax_minor >= 0),
  service_fee_minor integer NOT NULL DEFAULT 0 CHECK (service_fee_minor >= 0),
  tip_minor integer NOT NULL DEFAULT 0 CHECK (tip_minor >= 0),
  total_minor integer NOT NULL DEFAULT 0 CHECK (total_minor >= 0),
  opened_at timestamptz NOT NULL DEFAULT now(),
  closed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (closed_at IS NULL OR closed_at >= opened_at)
);

CREATE UNIQUE INDEX bills_one_open_per_table_idx
  ON bills (venue_table_id) WHERE venue_table_id IS NOT NULL AND status IN ('open', 'payment_pending');
CREATE INDEX bills_venue_status_idx ON bills (venue_id, status, opened_at DESC);
CREATE TRIGGER bills_set_updated_at BEFORE UPDATE ON bills
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE orders (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_number bigint GENERATED ALWAYS AS IDENTITY,
  venue_id uuid NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
  bill_id uuid REFERENCES bills(id) ON DELETE SET NULL,
  reservation_id uuid REFERENCES reservations(id) ON DELETE SET NULL,
  venue_table_id uuid REFERENCES venue_tables(id) ON DELETE SET NULL,
  customer_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  assigned_waiter_staff_id uuid REFERENCES venue_staff(id) ON DELETE SET NULL,
  channel text NOT NULL CHECK (channel IN ('dine_in', 'online', 'pickup', 'preorder')),
  status text NOT NULL DEFAULT 'new'
    CHECK (status IN ('draft', 'new', 'accepted', 'preparing', 'ready', 'served', 'completed', 'cancelled')),
  guest_name text,
  guest_email text,
  currency char(3) NOT NULL DEFAULT 'EUR' CHECK (currency ~ '^[A-Z]{3}$'),
  subtotal_minor integer NOT NULL DEFAULT 0 CHECK (subtotal_minor >= 0),
  tax_minor integer NOT NULL DEFAULT 0 CHECK (tax_minor >= 0),
  service_fee_minor integer NOT NULL DEFAULT 0 CHECK (service_fee_minor >= 0),
  discount_minor integer NOT NULL DEFAULT 0 CHECK (discount_minor >= 0),
  total_minor integer NOT NULL DEFAULT 0 CHECK (total_minor >= 0),
  notes text,
  share_token_hash text,
  idempotency_key text,
  placed_at timestamptz,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (venue_id, order_number)
);

CREATE UNIQUE INDEX orders_idempotency_unique
  ON orders (venue_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE UNIQUE INDEX orders_share_token_unique
  ON orders (share_token_hash) WHERE share_token_hash IS NOT NULL;
CREATE INDEX orders_operations_idx ON orders (venue_id, status, created_at DESC);
CREATE INDEX orders_customer_history_idx ON orders (customer_user_id, created_at DESC)
  WHERE customer_user_id IS NOT NULL;
CREATE INDEX orders_bill_idx ON orders (bill_id) WHERE bill_id IS NOT NULL;
CREATE TRIGGER orders_set_updated_at BEFORE UPDATE ON orders
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE order_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  menu_item_id uuid REFERENCES menu_items(id) ON DELETE SET NULL,
  parent_item_id uuid REFERENCES order_items(id) ON DELETE CASCADE,
  item_name text NOT NULL,
  unit_price_minor integer NOT NULL CHECK (unit_price_minor >= 0),
  quantity integer NOT NULL CHECK (quantity > 0),
  total_minor integer NOT NULL CHECK (total_minor >= 0),
  status text NOT NULL DEFAULT 'new'
    CHECK (status IN ('new', 'preparing', 'ready', 'served', 'cancelled')),
  modifiers jsonb NOT NULL DEFAULT '[]'::jsonb,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX order_items_order_idx ON order_items (order_id, created_at);
CREATE INDEX order_items_kitchen_idx ON order_items (status, created_at)
  WHERE status IN ('new', 'preparing', 'ready');
CREATE TRIGGER order_items_set_updated_at BEFORE UPDATE ON order_items
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE payments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  venue_id uuid NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
  bill_id uuid REFERENCES bills(id) ON DELETE SET NULL,
  order_id uuid REFERENCES orders(id) ON DELETE SET NULL,
  provider text NOT NULL,
  provider_payment_id text NOT NULL,
  status text NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'requires_action', 'processing', 'paid', 'failed', 'refunded', 'disputed', 'cancelled')),
  amount_minor integer NOT NULL CHECK (amount_minor > 0),
  refunded_minor integer NOT NULL DEFAULT 0 CHECK (refunded_minor >= 0),
  currency char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  idempotency_key text,
  failure_code text,
  provider_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  paid_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_payment_id),
  CHECK (bill_id IS NOT NULL OR order_id IS NOT NULL),
  CHECK (refunded_minor <= amount_minor)
);

CREATE UNIQUE INDEX payments_idempotency_unique
  ON payments (venue_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX payments_venue_status_idx ON payments (venue_id, status, created_at DESC);
CREATE TRIGGER payments_set_updated_at BEFORE UPDATE ON payments
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE payment_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  payment_id uuid REFERENCES payments(id) ON DELETE SET NULL,
  provider text NOT NULL,
  provider_event_id text NOT NULL,
  event_type text NOT NULL,
  payload jsonb NOT NULL,
  processed_at timestamptz,
  processing_error text,
  received_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_event_id)
);

CREATE INDEX payment_events_unprocessed_idx ON payment_events (received_at)
  WHERE processed_at IS NULL;

CREATE TABLE tips (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  venue_id uuid NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
  order_id uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  payment_id uuid REFERENCES payments(id) ON DELETE SET NULL,
  waiter_staff_id uuid REFERENCES venue_staff(id) ON DELETE SET NULL,
  amount_minor integer NOT NULL CHECK (amount_minor > 0),
  currency char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX tips_venue_created_idx ON tips (venue_id, created_at DESC);
CREATE INDEX tips_waiter_created_idx ON tips (waiter_staff_id, created_at DESC)
  WHERE waiter_staff_id IS NOT NULL;

CREATE TABLE reviews (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  venue_id uuid NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
  order_id uuid REFERENCES orders(id) ON DELETE SET NULL,
  customer_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  venue_table_id uuid REFERENCES venue_tables(id) ON DELETE SET NULL,
  rating smallint NOT NULL CHECK (rating BETWEEN 1 AND 5),
  body text,
  status text NOT NULL DEFAULT 'published' CHECK (status IN ('pending', 'published', 'hidden')),
  owner_reply text,
  replied_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  replied_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX reviews_one_per_order_idx ON reviews (order_id) WHERE order_id IS NOT NULL;
CREATE INDEX reviews_venue_rating_idx ON reviews (venue_id, rating, created_at DESC)
  WHERE status = 'published';
CREATE TRIGGER reviews_set_updated_at BEFORE UPDATE ON reviews
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE workspace_subscriptions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider text NOT NULL,
  provider_customer_id text,
  provider_subscription_id text,
  plan text NOT NULL CHECK (plan IN ('start', 'business', 'pro')),
  billing_cycle text NOT NULL CHECK (billing_cycle IN ('monthly', 'yearly')),
  status text NOT NULL DEFAULT 'trialing'
    CHECK (status IN ('trialing', 'active', 'past_due', 'cancelled', 'expired')),
  venue_limit integer CHECK (venue_limit IS NULL OR venue_limit > 0),
  trial_ends_at timestamptz,
  current_period_ends_at timestamptz,
  cancelled_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX workspace_subscriptions_one_current_idx
  ON workspace_subscriptions (owner_user_id) WHERE status IN ('trialing', 'active', 'past_due');
CREATE TRIGGER workspace_subscriptions_set_updated_at BEFORE UPDATE ON workspace_subscriptions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE audit_logs (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  venue_id uuid REFERENCES venues(id) ON DELETE SET NULL,
  actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  actor_staff_id uuid REFERENCES venue_staff(id) ON DELETE SET NULL,
  action text NOT NULL,
  entity_type text NOT NULL,
  entity_id uuid,
  changes jsonb NOT NULL DEFAULT '{}'::jsonb,
  ip_address inet,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_logs_venue_created_idx ON audit_logs (venue_id, created_at DESC);
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity_type, entity_id, created_at DESC);

CREATE TABLE outbox_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  topic text NOT NULL,
  aggregate_type text NOT NULL,
  aggregate_id uuid NOT NULL,
  payload jsonb NOT NULL,
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  available_at timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX outbox_events_pending_idx ON outbox_events (available_at, created_at)
  WHERE processed_at IS NULL;
