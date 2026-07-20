CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE FUNCTION set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL,
  password_hash text NOT NULL,
  first_name text NOT NULL,
  last_name text NOT NULL,
  phone text,
  account_role text NOT NULL CHECK (account_role IN ('customer', 'venue_owner')),
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('pending', 'active', 'suspended', 'deleted')),
  email_verified_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE UNIQUE INDEX users_email_unique ON users (lower(email)) WHERE deleted_at IS NULL;
CREATE INDEX users_role_idx ON users (account_role) WHERE deleted_at IS NULL;
CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE user_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  refresh_token_hash text NOT NULL UNIQUE,
  user_agent text,
  ip_address inet,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX user_sessions_user_idx ON user_sessions (user_id, expires_at DESC);

CREATE TABLE venues (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL REFERENCES users(id),
  slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
  name text NOT NULL,
  description text,
  cuisine_type text,
  phone text,
  email text,
  address_line1 text NOT NULL,
  address_line2 text,
  city text NOT NULL,
  postal_code text,
  country_code char(2) NOT NULL,
  timezone text NOT NULL DEFAULT 'UTC',
  currency char(3) NOT NULL DEFAULT 'EUR' CHECK (currency ~ '^[A-Z]{3}$'),
  status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'paused', 'closed')),
  settings jsonb NOT NULL DEFAULT '{}'::jsonb,
  opened_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE INDEX venues_owner_idx ON venues (owner_user_id) WHERE deleted_at IS NULL;
CREATE INDEX venues_public_idx ON venues (status, city) WHERE deleted_at IS NULL;
CREATE TRIGGER venues_set_updated_at BEFORE UPDATE ON venues
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE venue_staff (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  venue_id uuid NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
  user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  invited_email text NOT NULL,
  role text NOT NULL CHECK (role IN ('manager', 'waiter', 'kitchen', 'viewer')),
  status text NOT NULL DEFAULT 'invited' CHECK (status IN ('invited', 'active', 'inactive', 'removed')),
  invited_by_user_id uuid NOT NULL REFERENCES users(id),
  invited_at timestamptz NOT NULL DEFAULT now(),
  accepted_at timestamptz,
  removed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX venue_staff_email_unique
  ON venue_staff (venue_id, lower(invited_email)) WHERE status <> 'removed';
CREATE UNIQUE INDEX venue_staff_user_unique
  ON venue_staff (venue_id, user_id) WHERE user_id IS NOT NULL AND status <> 'removed';
CREATE INDEX venue_staff_role_idx ON venue_staff (venue_id, role, status);
CREATE TRIGGER venue_staff_set_updated_at BEFORE UPDATE ON venue_staff
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE venue_staff_permissions (
  venue_staff_id uuid NOT NULL REFERENCES venue_staff(id) ON DELETE CASCADE,
  permission text NOT NULL,
  granted_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (venue_staff_id, permission)
);

CREATE TABLE venue_payment_accounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  venue_id uuid NOT NULL UNIQUE REFERENCES venues(id) ON DELETE CASCADE,
  provider text NOT NULL,
  provider_account_id text NOT NULL,
  onboarding_status text NOT NULL DEFAULT 'pending'
    CHECK (onboarding_status IN ('pending', 'restricted', 'enabled', 'disabled')),
  charges_enabled boolean NOT NULL DEFAULT false,
  payouts_enabled boolean NOT NULL DEFAULT false,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_account_id)
);

CREATE TRIGGER venue_payment_accounts_set_updated_at BEFORE UPDATE ON venue_payment_accounts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
