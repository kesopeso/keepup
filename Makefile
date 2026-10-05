COMPOSE ?= docker compose

.PHONY: doctor doctor-mobile setup mobile-setup mobile up down logs ps restart lint-api

doctor:
	./bin/doctor.sh

doctor-mobile:
	./bin/doctor.sh mobile

setup: doctor
	$(MAKE) up
	./bin/migrate.sh up

mobile-setup: doctor-mobile
	$(COMPOSE) build mobile-helper
	./bin/mobile-pnpm.sh install
	./bin/mobile-pnpm.sh exec expo install --check
	./bin/mobile-pnpm.sh exec expo run:android --device

mobile: doctor-mobile
	./bin/start-metro.sh

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
