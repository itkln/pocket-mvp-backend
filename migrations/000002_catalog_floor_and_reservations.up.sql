CREATE TABLE menu_categories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  venue_id uuid NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
  name text NOT NULL,
  description text,
  sort_order integer NOT NULL DEFAULT 0,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (venue_id, name)
);

CREATE INDEX menu_categories_venue_sort_idx ON menu_categories (venue_id, sort_order);
CREATE TRIGGER menu_categories_set_updated_at BEFORE UPDATE ON menu_categories
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE menu_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  venue_id uuid NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
  category_id uuid NOT NULL REFERENCES menu_categories(id),
  name text NOT NULL,
  description text,
  price_minor integer NOT NULL CHECK (price_minor >= 0),
  currency char(3) NOT NULL DEFAULT 'EUR' CHECK (currency ~ '^[A-Z]{3}$'),
  tax_rate_basis_points integer NOT NULL DEFAULT 0 CHECK (tax_rate_basis_points BETWEEN 0 AND 10000),
  is_available boolean NOT NULL DEFAULT true,
  is_popular boolean NOT NULL DEFAULT false,
  sort_order integer NOT NULL DEFAULT 0,
  attributes jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE INDEX menu_items_catalog_idx
  ON menu_items (venue_id, category_id, sort_order) WHERE deleted_at IS NULL;
CREATE INDEX menu_items_available_idx
  ON menu_items (venue_id, is_available) WHERE deleted_at IS NULL;
CREATE TRIGGER menu_items_set_updated_at BEFORE UPDATE ON menu_items
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE menu_item_images (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  menu_item_id uuid NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
  storage_key text NOT NULL UNIQUE,
  public_url text NOT NULL,
  alt_text text,
  content_type text NOT NULL,
  byte_size bigint NOT NULL CHECK (byte_size > 0),
  width integer CHECK (width > 0),
  height integer CHECK (height > 0),
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX menu_item_images_item_idx ON menu_item_images (menu_item_id, sort_order);

CREATE TABLE venue_layouts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  venue_id uuid NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
  name text NOT NULL DEFAULT 'Основной зал',
  width numeric(10,2) NOT NULL DEFAULT 100 CHECK (width > 0),
  height numeric(10,2) NOT NULL DEFAULT 100 CHECK (height > 0),
  background jsonb NOT NULL DEFAULT '{}'::jsonb,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (venue_id, name)
);

CREATE UNIQUE INDEX venue_layouts_one_active_idx ON venue_layouts (venue_id) WHERE is_active;
CREATE TRIGGER venue_layouts_set_updated_at BEFORE UPDATE ON venue_layouts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE venue_tables (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  venue_id uuid NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
  layout_id uuid NOT NULL REFERENCES venue_layouts(id) ON DELETE CASCADE,
  identifier text NOT NULL,
  seats integer NOT NULL CHECK (seats > 0),
  shape text NOT NULL DEFAULT 'rectangle' CHECK (shape IN ('rectangle', 'round', 'square', 'custom')),
  position_x numeric(10,2) NOT NULL DEFAULT 0,
  position_y numeric(10,2) NOT NULL DEFAULT 0,
  width numeric(10,2) NOT NULL DEFAULT 10 CHECK (width > 0),
  height numeric(10,2) NOT NULL DEFAULT 10 CHECK (height > 0),
  chairs jsonb NOT NULL DEFAULT '[]'::jsonb,
  status text NOT NULL DEFAULT 'available'
    CHECK (status IN ('available', 'occupied', 'reserved', 'disabled')),
  qr_token_hash text NOT NULL,
  qr_version integer NOT NULL DEFAULT 1 CHECK (qr_version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  UNIQUE (venue_id, identifier),
  UNIQUE (venue_id, qr_token_hash)
);

CREATE INDEX venue_tables_layout_idx ON venue_tables (layout_id) WHERE deleted_at IS NULL;
CREATE TRIGGER venue_tables_set_updated_at BEFORE UPDATE ON venue_tables
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE table_waiter_assignments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  venue_table_id uuid NOT NULL REFERENCES venue_tables(id) ON DELETE CASCADE,
  venue_staff_id uuid NOT NULL REFERENCES venue_staff(id),
  starts_at timestamptz NOT NULL DEFAULT now(),
  ends_at timestamptz,
  assigned_by_user_id uuid NOT NULL REFERENCES users(id),
  CHECK (ends_at IS NULL OR ends_at > starts_at)
);

CREATE UNIQUE INDEX table_waiter_one_active_idx
  ON table_waiter_assignments (venue_table_id) WHERE ends_at IS NULL;
CREATE INDEX table_waiter_staff_idx
  ON table_waiter_assignments (venue_staff_id, starts_at DESC);

CREATE TABLE reservations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  venue_id uuid NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
  venue_table_id uuid NOT NULL REFERENCES venue_tables(id),
  customer_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  reservation_code text NOT NULL UNIQUE,
  guest_name text NOT NULL,
  guest_email text,
  guest_phone text,
  guest_count integer NOT NULL CHECK (guest_count > 0),
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  status text NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'confirmed', 'seated', 'completed', 'cancelled', 'no_show')),
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (ends_at > starts_at),
  EXCLUDE USING gist (
    venue_table_id WITH =,
    tstzrange(starts_at, ends_at, '[)') WITH &&
  ) WHERE (status IN ('pending', 'confirmed'))
);

CREATE INDEX reservations_venue_time_idx ON reservations (venue_id, starts_at, status);
CREATE INDEX reservations_customer_idx ON reservations (customer_user_id, starts_at DESC)
  WHERE customer_user_id IS NOT NULL;
CREATE TRIGGER reservations_set_updated_at BEFORE UPDATE ON reservations
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE reservation_preorder_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reservation_id uuid NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
  menu_item_id uuid REFERENCES menu_items(id) ON DELETE SET NULL,
  item_name text NOT NULL,
  unit_price_minor integer NOT NULL CHECK (unit_price_minor >= 0),
  quantity integer NOT NULL CHECK (quantity > 0),
  notes text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX reservation_preorder_items_reservation_idx
  ON reservation_preorder_items (reservation_id);
