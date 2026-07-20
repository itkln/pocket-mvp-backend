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
