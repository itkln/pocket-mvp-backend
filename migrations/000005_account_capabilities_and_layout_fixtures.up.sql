ALTER TABLE users
  ALTER COLUMN account_role SET DEFAULT 'customer';

ALTER TABLE venue_staff
  ADD COLUMN display_name text;

DROP INDEX IF EXISTS venue_layouts_one_active_idx;

CREATE TABLE venue_layout_fixtures (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  venue_id uuid NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
  layout_id uuid NOT NULL REFERENCES venue_layouts(id) ON DELETE CASCADE,
  client_key text NOT NULL,
  fixture_type text NOT NULL CHECK (fixture_type IN ('bar', 'window', 'entrance')),
  position_x numeric(10,2) NOT NULL DEFAULT 0,
  position_y numeric(10,2) NOT NULL DEFAULT 0,
  rotation smallint NOT NULL DEFAULT 0 CHECK (rotation >= 0 AND rotation < 360),
  size numeric(10,2) NOT NULL DEFAULT 100 CHECK (size > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (layout_id, client_key)
);

CREATE INDEX venue_layout_fixtures_layout_idx
  ON venue_layout_fixtures (layout_id);

CREATE TRIGGER venue_layout_fixtures_set_updated_at
  BEFORE UPDATE ON venue_layout_fixtures
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
