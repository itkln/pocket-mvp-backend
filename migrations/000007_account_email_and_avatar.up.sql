ALTER TABLE users
  ADD COLUMN avatar_data bytea,
  ADD COLUMN avatar_mime_type text
    CHECK (avatar_mime_type IS NULL OR avatar_mime_type IN ('image/jpeg', 'image/png', 'image/webp')),
  ADD COLUMN avatar_updated_at timestamptz;
