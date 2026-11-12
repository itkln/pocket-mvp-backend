-- V-10: venue_tables follows the saved floor plan. Venues saved before the sync existed get their
-- floors as active layouts and every plan table as a row; rows the plan no longer has are
-- soft-deleted unless guests still depend on them.
WITH plan_floors AS (
  SELECT v.id AS venue_id, f.ordinality AS floor_no,
         COALESCE(NULLIF(btrim(f.value->>'name'), ''), 'Зал ' || f.ordinality) AS name, f.value AS floor
  FROM venues v,
       jsonb_array_elements(CASE WHEN jsonb_typeof(v.settings->'floor_plan') = 'array'
                                 THEN v.settings->'floor_plan' ELSE '[]'::jsonb END) WITH ORDINALITY f
  WHERE v.deleted_at IS NULL AND jsonb_typeof(f.value) = 'object'
)
INSERT INTO venue_layouts (venue_id, name, is_active)
SELECT DISTINCT venue_id, name, true FROM plan_floors
ON CONFLICT (venue_id, name) DO UPDATE SET is_active = true;

WITH plan_tables AS (
  SELECT DISTINCT ON (v.id, t.value->>'id')
         v.id AS venue_id,
         COALESCE(NULLIF(btrim(f.value->>'name'), ''), 'Зал ' || f.ordinality) AS floor_name,
         t.value->>'id' AS identifier,
         (t.value->>'seats')::numeric::int AS seats,
         CASE WHEN t.value->>'shape' IN ('round', 'square') THEN t.value->>'shape' ELSE 'rectangle' END AS shape,
         COALESCE((t.value->>'x')::numeric, 0) AS x,
         COALESCE((t.value->>'y')::numeric, 0) AS y
  FROM venues v,
       jsonb_array_elements(CASE WHEN jsonb_typeof(v.settings->'floor_plan') = 'array'
                                 THEN v.settings->'floor_plan' ELSE '[]'::jsonb END) WITH ORDINALITY f,
       jsonb_array_elements(CASE WHEN jsonb_typeof(f.value->'tables') = 'array'
                                 THEN f.value->'tables' ELSE '[]'::jsonb END) t
  WHERE v.deleted_at IS NULL AND jsonb_typeof(f.value) = 'object'
    AND COALESCE(t.value->>'id', '') <> ''
    AND jsonb_typeof(t.value->'seats') = 'number' AND (t.value->>'seats')::numeric >= 1
  ORDER BY v.id, t.value->>'id', f.ordinality
)
INSERT INTO venue_tables (venue_id, layout_id, identifier, seats, shape, position_x, position_y, qr_token_hash)
SELECT p.venue_id, l.id, p.identifier, p.seats, p.shape, p.x, p.y, 'unissued:' || gen_random_uuid()::text
FROM plan_tables p JOIN venue_layouts l ON l.venue_id = p.venue_id AND l.name = p.floor_name
ON CONFLICT (venue_id, identifier) DO UPDATE SET
  layout_id = EXCLUDED.layout_id, seats = EXCLUDED.seats, shape = EXCLUDED.shape,
  position_x = EXCLUDED.position_x, position_y = EXCLUDED.position_y, deleted_at = NULL;

UPDATE venue_tables t SET deleted_at = now()
FROM venues v
WHERE v.id = t.venue_id AND t.deleted_at IS NULL
  AND jsonb_typeof(v.settings->'floor_plan') = 'array'
  AND NOT EXISTS (
    SELECT 1 FROM jsonb_array_elements(v.settings->'floor_plan') f,
         jsonb_array_elements(CASE WHEN jsonb_typeof(f.value->'tables') = 'array'
                                   THEN f.value->'tables' ELSE '[]'::jsonb END) ft
    WHERE jsonb_typeof(f.value) = 'object' AND ft->>'id' = t.identifier)
  AND NOT EXISTS (SELECT 1 FROM reservations r WHERE r.venue_table_id = t.id
    AND r.status IN ('pending', 'confirmed', 'seated') AND r.ends_at > now() - interval '24 hours')
  AND NOT EXISTS (SELECT 1 FROM bills b WHERE b.venue_table_id = t.id AND b.status IN ('open', 'payment_pending'))
  AND NOT EXISTS (SELECT 1 FROM orders o WHERE o.venue_table_id = t.id
    AND o.status IN ('new', 'accepted', 'preparing', 'ready'));
