COMPOSE ?= docker compose

.PHONY: up down logs ps restart lint-api

up:
	$(COMPOSE) up --build -d

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f

ps:
	$(COMPOSE) ps

restart: down up
	# $(COMPOSE) down
	# $(COMPOSE) up --build

lint-api:
	$(COMPOSE) run --rm api golangci-lint run
