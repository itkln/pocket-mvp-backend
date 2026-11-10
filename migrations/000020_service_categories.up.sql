ALTER TABLE bookable_services ADD COLUMN category_name text NOT NULL DEFAULT '', ADD COLUMN resource_id uuid REFERENCES venue_tables(id);
