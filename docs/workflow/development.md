---
type: reference
status: active
---

# Development workflow

All application tooling runs through Docker Compose. Do not run host pnpm, npm, Go, or other application toolchains.

## Local Development Workflow

- `docker compose up` starts the main local stack: web, api, and postgres.
- One-off web pnpm commands can run through `bin/web-pnpm.sh`, which uses the dependency-free `web-helper` Compose service.
- Example: `./bin/web-pnpm.sh build`
- Database migrations run through `bin/migrate.sh`, which wraps the tools-profile `migrate` Compose service, mounts `db/migrations`, and waits for the Postgres healthcheck.
- Examples: `./bin/migrate.sh up`, `./bin/migrate.sh down 1`, `./bin/migrate.sh down -all`


- API checks: `docker compose run --rm api go test ./...` and `make lint-api`.
- Database migrations are manual via `golang-migrate`; the API must never apply migrations at startup.
- Migration status: `./bin/migrate.sh version`.

## Dev and Deployment

- One monorepo
- Everything containerized
- Local development runs via `docker compose up`
- Services:
  - web
  - api
  - postgres/postgis
- Production MVP targets a single VPS with containers
- HTTPS required in production

## Project Agent Skills

- `icm-architect` is installed at `.agents/skills/icm-architect` from https://github.com/RinDig/icm-architect.
- The installation is scoped to this repository and includes the upstream skill, references, assets, and license.
- Invoke it with `$icm-architect` when working in this project.


Sources: [Compose services](../../docker-compose.yml), [Makefile](../../Makefile), [migration helper](../../bin/migrate.sh), [web helper](../../bin/web-pnpm.sh).
