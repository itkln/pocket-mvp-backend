# Pocket MVP Backend

Go API and local infrastructure for Pocket MVP venue ordering, reservations,
payments, and operations.

## Included infrastructure

- Go 1.23 API with JSON logs, CORS, security headers, graceful shutdown, and
  PostgreSQL connection pooling.
- `chi` routing with routes grouped by business domain.
- PostgreSQL 17 with persistent local storage.
- Versioned SQL migrations executed before the API starts.
- Separate Pocket Vendor and Pocket Spots Next.js images from the sibling
  `frontend` repository.
- Health-gated startup: PostgreSQL -> migrations -> backend -> both frontends.
- Secure account authentication with Argon2id password hashes, encrypted personal
  data, rate-limited login attempts, and revocable HttpOnly cookie sessions.

API error responses use stable machine-readable codes and English messages.
Client applications localize those codes for the selected interface language.

## Architecture

The API is a modular monolith split into bounded contexts under
`internal/modules`: identity, venues, favorites, catalog, workforce, ordering, feedback,
billing, floor plan, and reporting. Each domain separates use-case services,
repository ports, and PostgreSQL adapters. HTTP handlers depend on narrow
interfaces, and architecture tests enforce domain and persistence boundaries.

See [docs/architecture.md](docs/architecture.md) for dependency rules, data
ownership, and the path for extracting a module into a microservice.

## Pocket Spots saved venues (E-07)

Authenticated routes under `/api/v1/spots/me/saved-venues` use the session
cookie, never a client-supplied user ID:

- `GET` returns `{ "venue_ids": ["..."] }` in most-recent-first order.
- `PUT /{venueID}` saves a published venue; repeated calls are idempotent.
- `DELETE /{venueID}` removes it; repeated calls are idempotent.

Invalid IDs return 422, unpublished or missing venues return 404 on save, and
guests receive 401. The public catalog accepts up to 50 comma-separated UUIDs
in `GET /api/v1/public/venues?ids=...` for bulk hydration. The
`GET /api/v1/public/venues/{venueID}` endpoint returns a current, published
venue read model.
Favorites persist only stable UUIDs, so renames and multi-device sessions stay
in sync. Legacy browser favorites stored by name are intentionally not
auto-imported because duplicate names cannot be resolved safely to an ID.

`TestSavedVenuesIntegration` is opt-in via `TEST_DATABASE_URL` and requires an
isolated PostgreSQL database whose name ends in `_test`.

## Run the complete stack

```bash
cp .env.example .env
docker compose up --build -d
docker compose ps
```

Services:

- Pocket Vendor: http://localhost:3000
- Pocket Spots: http://localhost:3001
- API metadata: http://localhost:8080/api/v1
- API liveness: http://localhost:8080/healthz
- API readiness: http://localhost:8080/readyz
- PostgreSQL: localhost:5432

Stop containers while keeping database data:

```bash
docker compose down
```

Remove containers and the local database volume:

```bash
docker compose down --volumes
```

## Demo workspace

Populate a local database with a repeatable owner workspace:

```bash
make seed-demo
```

Then open Pocket Vendor at http://localhost:3000 or Pocket Spots at
http://localhost:3001 and sign in with:

- E-mail: `demo@pocket.local`
- Password: `PocketDemo123!`

The seed includes two venues, categories and menu items with images, staff,
multi-floor layouts, orders in different states, payments, reviews, analytics,
and an active Business trial. Running the command again replaces only this demo
workspace, so it does not create duplicates. Override `DEMO_EMAIL` and
`DEMO_PASSWORD` in `.env` when needed.

The command is blocked outside `APP_ENV=development`. A non-development run
requires the explicit `ALLOW_DEMO_SEED=true` override.

## Migrations

Migration files live in `migrations/` and follow the `golang-migrate` naming
convention. The Compose `migrate` service applies all pending migrations before
the API starts.

```bash
docker compose run --rm migrate \
  -path=/migrations \
  -database="postgres://pocket:pocket_local_password@postgres:5432/pocket?sslmode=disable" \
  version
```

Create every schema change as a new up/down migration. Do not edit a migration
that has already been used outside a disposable local database.

## Run only the API

Set `DATABASE_URL`, then run:

```bash
go run ./cmd/api
```

Required environment:

- `DATABASE_URL`: PostgreSQL connection URL.

Optional environment is documented in `.env.example`.

## Authentication

Endpoints under `/api/v1/auth`:

- `POST /register` creates an account and session. Owner capability is added
  when the user creates a venue.
- `POST /login` verifies credentials and creates a new session.
- `GET /me` returns the user represented by the HttpOnly session cookie.
- `POST /logout` revokes the session and clears the cookie.
- `POST /password-reset/request` accepts `email`, `locale`, and `product`
  (`spots` or `vendor`) and returns the same 202 response whether or not the
  account exists. Missing `product` retains the legacy Vendor destination;
  unknown values return 422 before account lookup.
- `POST /password-reset/confirm` consumes a single-use reset token, updates the
  Argon2id password hash, and revokes every existing session for the account.

Passwords are hashed with Argon2id. E-mail, first name, last name, and phone are
encrypted with AES-256-GCM; a separate HMAC-SHA256 blind index supports exact
e-mail lookup without storing searchable plaintext. Session tokens are returned
only in cookies, while PostgreSQL stores their SHA-256 hashes.

Password-reset tokens are also stored only as SHA-256 hashes and expire after
30 minutes by default. In development, generated reset links are written to the
backend log. Set `SMTP_ADDRESS`, `SMTP_USERNAME`, `SMTP_PASSWORD`, and
`SMTP_FROM` to deliver real e-mail; production startup requires SMTP to be
configured.

Reset links return to the originating product using **server-configured origins**:

- `VENDOR_APP_BASE_URL`: Vendor origin; falls back to legacy `APP_BASE_URL`, then
  `http://localhost:3000`.
- `SPOTS_APP_BASE_URL`: Spots origin; defaults to `http://localhost:3001` only in
  development and is required in other environments.
- `EATS_APP_BASE_URL` is accepted only as a deprecated migration fallback when
  `SPOTS_APP_BASE_URL` is unset. Reset API requests with `product: "eats"` are
  likewise routed to Spots temporarily; new clients must send `product: "spots"`.

Set both to the externally reachable HTTPS origins in production (and adjust
them when changing local frontend ports). Origins must not contain credentials,
paths, queries, or fragments. The server never builds reset links from request
Host/Origin headers or arbitrary return URLs. The product selects only the
destination, not a different account/password or permission set. Locale is
preserved; successful reset returns to the local login page in the same app.

### Password recovery integration test (E-03)

`go test ./...` runs the unit/regression tests. The opt-in test below additionally
uses the real router, PostgreSQL migrations and the SMTP sender with a local TCP
mail capture server; it never sends external e-mail:

```bash
TEST_DATABASE_URL='postgres://user:password@127.0.0.1:55433/pocket_e03_test?sslmode=disable' \
  go test ./internal/bootstrap -run TestPasswordRecoveryIntegration -count=1 -v
```

Use a disposable PostgreSQL database whose name ends in `_test`. The test creates
and removes its own schema, and requires permission to install `pgcrypto` and
`btree_gist`. It checks Spots and legacy Vendor destinations, locale, hashed-token
storage and TTL, concurrent single-use consumption, expiration, revocation of
all old sessions, login with the new password, and non-enumerating responses.
Production SMTP delivery/TLS and public domains still need environment-specific
deployment verification.

Generate independent production keys and enable secure cookies behind HTTPS:

```bash
openssl rand -base64 32 # DATA_ENCRYPTION_KEY
openssl rand -base64 32 # DATA_LOOKUP_KEY
COOKIE_SECURE=true
```

Never reuse the example development keys outside local development. Rotating the
encryption key requires a planned data re-encryption migration.

## Public venue catalog

Pocket Spots reads published venues without authentication from
`GET /api/v1/public/venues`. Only non-deleted venues with `status=active` are
returned. Supported query parameters are `city`, `q`, `category`, `filter`,
`page`, and `page_size` (maximum 50). The response includes a public venue read
model plus `total`, `total_pages`, and `has_next`; owner contact details and raw
venue settings are never exposed.

Venue settings may define `public_category`, `public_filters`, `public_tags`,
`public_search_terms`, and `cover_image_url`. Missing or invalid categories fall
back to `restaurants`, so every newly published Vendor venue is discoverable.
Published review aggregates supply `rating` and `review_count`.

`GET /api/v1/public/venues/{venueID}/menu` returns the visible menu of a
published venue as `{ "categories": [{ "id", "name", "items": [...] }] }`.
It needs no session and includes only active categories and available,
non-deleted items. Invalid venue IDs return 422; unpublished or missing venues
return 404. Pocket Spots uses this read-only projection for venue menu search.

`GET /api/v1/public/venues/{venueID}/reviews` returns up to 50 most recent
published reviews for an active venue. The public review view omits guest names,
order IDs and table identifiers; the response contains only rating, body,
owner reply and creation time.

## Tests

```bash
go test ./...
```
