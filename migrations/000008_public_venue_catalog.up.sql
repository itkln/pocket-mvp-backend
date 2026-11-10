CREATE INDEX venues_public_catalog_idx
  ON venues (
    lower(city),
    created_at DESC
  )
  WHERE status = 'active' AND deleted_at IS NULL;
