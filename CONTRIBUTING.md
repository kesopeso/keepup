# Contributing

## Working Model

- The repo root is a monorepo.
- Product and architecture decisions live in `docs/`.
- When a product rule or architectural decision changes, update the relevant doc in the same change.
- On completion of any meaningful task, refresh the affected files in `docs/` automatically.
- Do not wait for a user reminder before updating project docs.

## Local Development

- Main entrypoint: `docker compose up`
- Root helper commands are in the `Makefile`
- API linting runs inside the API container with `make lint-api`
- Database migrations are manual via `golang-migrate`; they are not applied automatically on API startup

## Repository Structure

- `apps/web`: Next.js frontend
- `apps/api`: Go backend
- `db/migrations`: database migrations
- `docs`: product, architecture, backlog, and planning docs
