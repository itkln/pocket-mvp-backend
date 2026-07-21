DROP TRIGGER IF EXISTS venue_layout_fixtures_set_updated_at ON venue_layout_fixtures;
DROP TABLE IF EXISTS venue_layout_fixtures;

CREATE UNIQUE INDEX venue_layouts_one_active_idx
  ON venue_layouts (venue_id) WHERE is_active;

ALTER TABLE venue_staff DROP COLUMN IF EXISTS display_name;
ALTER TABLE users ALTER COLUMN account_role DROP DEFAULT;
