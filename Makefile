.PHONY: build up down logs ps migrate-up migrate-down migrate-version test

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

test:
	docker run --rm -v "$(CURDIR):/src" -w /src golang:1.23-alpine \
		sh -c "go test ./... && go vet ./..."
