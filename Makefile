.PHONY: build up down logs ps migrate-up migrate-down migrate-version seed-demo test test-integration lint

build:
	docker compose build

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

ps:
	docker compose ps

migrate-up:
	docker compose run --rm migrate 'migrate -path=/migrations -database="$$DATABASE_URL" up'

migrate-down:
	docker compose run --rm migrate 'migrate -path=/migrations -database="$$DATABASE_URL" down 1'

migrate-version:
	docker compose run --rm migrate 'migrate -path=/migrations -database="$$DATABASE_URL" version'

seed-demo:
	docker compose build backend
	docker compose --profile tools run --rm seed

GO_CACHE = -v pocket-go-mod:/go/pkg/mod -v pocket-go-build:/root/.cache/go-build

# Unit tests and vet; tests that need PostgreSQL skip themselves.
test:
	docker run --rm -v "$(CURDIR):/src" -w /src $(GO_CACHE) golang:1.23-alpine \
		sh -c "go vet ./... && go test ./..."

# Every test, including PostgreSQL ones, against a disposable database (what CI runs).
test-integration:
	./scripts/test-integration.sh

lint:
	docker run --rm -v "$(CURDIR):/src" -w /src $(GO_CACHE) -v pocket-golangci:/root/.cache/golangci-lint \
		golangci/golangci-lint:v1.61.0 golangci-lint run ./...
