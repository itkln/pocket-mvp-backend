# Pocket MVP Backend

Go API and local infrastructure for Pocket MVP venue ordering, reservations,
payments, and operations.

## Included infrastructure

- Go 1.23 API with JSON logs, CORS, security headers, graceful shutdown, and
  PostgreSQL connection pooling.
- PostgreSQL 17 with persistent local storage.
- Versioned SQL migrations executed before the API starts.
- Production Next.js frontend image from the sibling `frontend` repository.
- Health-gated startup: PostgreSQL -> migrations -> backend -> frontend.
- Secure account authentication with Argon2id password hashes, encrypted personal
  data, rate-limited login attempts, and revocable HttpOnly cookie sessions.

## Run the complete stack

```bash
cp .env.example .env
docker compose up --build -d
docker compose ps
```

Services:

- Frontend: http://localhost:3000
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

- `POST /register` creates a customer or venue-owner account and session.
- `POST /login` verifies credentials and creates a new session.
- `GET /me` returns the user represented by the HttpOnly session cookie.
- `POST /logout` revokes the session and clears the cookie.

Passwords are hashed with Argon2id. E-mail, first name, last name, and phone are
encrypted with AES-256-GCM; a separate HMAC-SHA256 blind index supports exact
e-mail lookup without storing searchable plaintext. Session tokens are returned
only in cookies, while PostgreSQL stores their SHA-256 hashes.

Generate independent production keys and enable secure cookies behind HTTPS:

```bash
openssl rand -base64 32 # DATA_ENCRYPTION_KEY
openssl rand -base64 32 # DATA_LOOKUP_KEY
COOKIE_SECURE=true
```

Never reuse the example development keys outside local development. Rotating the
encryption key requires a planned data re-encryption migration.

## Tests

```bash
go test ./...
```
