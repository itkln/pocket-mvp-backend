DROP INDEX users_email_unique;

ALTER TABLE users
  ADD COLUMN email_lookup bytea;

CREATE UNIQUE INDEX users_email_lookup_unique
  ON users (email_lookup)
  WHERE deleted_at IS NULL;

CREATE TABLE auth_login_attempts (
  email_lookup bytea NOT NULL,
  ip_address inet NOT NULL,
  failure_count integer NOT NULL DEFAULT 0 CHECK (failure_count >= 0),
  window_started_at timestamptz NOT NULL DEFAULT now(),
  blocked_until timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (email_lookup, ip_address)
);

CREATE INDEX auth_login_attempts_cleanup_idx
  ON auth_login_attempts (updated_at);

CREATE TRIGGER auth_login_attempts_set_updated_at
  BEFORE UPDATE ON auth_login_attempts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

