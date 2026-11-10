CREATE TABLE order_status_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 order_id uuid NOT NULL REFERENCES orders(id),
 actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
 actor_name text NOT NULL,
 previous_status text NOT NULL,
 new_status text NOT NULL,
 reason text NOT NULL DEFAULT '',
 comment text NOT NULL DEFAULT '' CHECK (char_length(comment) <= 500),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX order_status_events_order ON order_status_events(order_id, created_at, id);
