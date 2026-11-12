-- V-05: Vendor and Dish read the same venue switches.
--  * the preorder switch was saved by Vendor as "preorder" while Dish and bookings read
--    "preorder_enabled"; the Vendor value wins;
--  * demo venues stored dine-in as "dine_in_enabled"; QR ordering reads "dine_in";
--  * switches Vendor showed as on by default are stored explicitly.
UPDATE venues
SET settings = (settings - 'preorder' - 'dine_in_enabled') || jsonb_build_object(
  'preorder_enabled', COALESCE(settings->'preorder', settings->'preorder_enabled', 'true'::jsonb),
  'dine_in', COALESCE(settings->'dine_in', settings->'dine_in_enabled', 'true'::jsonb),
  'schedule_enabled', COALESCE(settings->'schedule_enabled', 'true'::jsonb),
  'same_day_booking', COALESCE(settings->'same_day_booking', 'true'::jsonb)
);
