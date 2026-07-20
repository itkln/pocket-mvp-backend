DROP TRIGGER IF EXISTS auth_login_attempts_set_updated_at ON auth_login_attempts;
DROP TABLE IF EXISTS auth_login_attempts;

DROP INDEX IF EXISTS users_email_lookup_unique;
ALTER TABLE users DROP COLUMN IF EXISTS email_lookup;
CREATE UNIQUE INDEX users_email_unique ON users (lower(email)) WHERE deleted_at IS NULL;

