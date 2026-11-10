CREATE TABLE bookable_services (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 venue_id uuid NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
 name text NOT NULL,
 description text NOT NULL DEFAULT '',
 duration_minutes integer NOT NULL CHECK (duration_minutes BETWEEN 15 AND 480),
 price_minor integer NOT NULL CHECK (price_minor >= 0),
 currency char(3) NOT NULL DEFAULT 'EUR',
 is_active boolean NOT NULL DEFAULT true,
 sort_order integer NOT NULL DEFAULT 0
);
CREATE INDEX bookable_services_venue_idx ON bookable_services(venue_id, sort_order) WHERE is_active;
CREATE TABLE reservation_services (
 reservation_id uuid PRIMARY KEY REFERENCES reservations(id) ON DELETE CASCADE,
 service_id uuid REFERENCES bookable_services(id) ON DELETE SET NULL,
 name text NOT NULL,
 duration_minutes integer NOT NULL,
 price_minor integer NOT NULL,
 currency char(3) NOT NULL
);
