#!/bin/sh
# Runs all Go tests, including PostgreSQL tests, without accessing application data.
# PostgreSQL uses tmpfs and exposes no host port. Existing Go caches are reused.
set -eu
backend_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
database_container=''
cleanup() {
  if [ -n "$database_container" ]; then
    docker rm -f "$database_container" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
database_container=$(docker run -d --rm --tmpfs /var/lib/postgresql/data \
  -e POSTGRES_DB=pocket_integration_test \
  -e POSTGRES_PASSWORD=integration-test-only postgres:17-alpine)
attempt=0
until docker exec "$database_container" pg_isready -U postgres -d pocket_integration_test >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    docker logs "$database_container"
    exit 1
  fi
  sleep 1
done
if [ "$#" -eq 0 ]; then set -- ./...; fi
docker run --rm --network "container:$database_container" \
  -v "$backend_root:/src:ro" -w /src \
  -v pocket-go-mod:/go/pkg/mod -v pocket-go-build:/root/.cache/go-build \
  -e 'TEST_DATABASE_URL=postgres://postgres:integration-test-only@127.0.0.1:5432/pocket_integration_test?sslmode=disable' \
  golang:1.23-alpine go test "$@"
