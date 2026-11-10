CREATE TABLE venue_order_alert_settings (
 venue_id uuid PRIMARY KEY REFERENCES venues(id) ON DELETE CASCADE,
 warning_minutes integer NOT NULL DEFAULT 10 CHECK(warning_minutes > 0),
 critical_minutes integer NOT NULL DEFAULT 20 CHECK(critical_minutes > warning_minutes AND critical_minutes <= 1440),
 warning_color text NOT NULL DEFAULT '#F59E0B' CHECK(warning_color ~ '^#[0-9A-Fa-f]{6}$'),
 critical_color text NOT NULL DEFAULT '#EF4444' CHECK(critical_color ~ '^#[0-9A-Fa-f]{6}$'),
 preorder_lead_minutes integer NOT NULL DEFAULT 30 CHECK(preorder_lead_minutes BETWEEN 0 AND 1440)
);
