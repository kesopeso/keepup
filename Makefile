COMPOSE ?= docker compose
DATABASE_URL ?= postgres://keepup:keepup@localhost:5432/keepup?sslmode=disable
MIGRATE_VERSION ?= v4.18.3
MIGRATE := docker run --rm --network host -v $(CURDIR)/db/migrations:/migrations migrate/migrate:$(MIGRATE_VERSION)

.PHONY: up down logs ps restart lint-api migrate-up migrate-down migrate-drop migrate-version

up:
	$(COMPOSE) up --build

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f

ps:
	$(COMPOSE) ps

restart:
	$(COMPOSE) down
	$(COMPOSE) up --build

lint-api:
	$(COMPOSE) run --rm api golangci-lint run

migrate-up:
	$(MIGRATE) -path=/migrations -database "$(DATABASE_URL)" up

migrate-down:
	$(MIGRATE) -path=/migrations -database "$(DATABASE_URL)" down 1

migrate-drop:
	$(MIGRATE) -path=/migrations -database "$(DATABASE_URL)" drop -f

migrate-version:
	-$(MIGRATE) -path=/migrations -database "$(DATABASE_URL)" version
