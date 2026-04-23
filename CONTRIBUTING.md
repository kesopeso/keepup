# Contributing

## Working Model

- The repo root is a monorepo.
- Product and architecture decisions live in `docs/`.
- When a product rule or architectural decision changes, update the relevant doc in the same change.

## Local Development

- Main entrypoint: `docker compose up`
- Root helper commands are in the `Makefile`

## Repository Structure

- `apps/web`: Next.js frontend
- `apps/api`: Go backend
- `db/migrations`: database migrations
- `docs`: product, architecture, backlog, and planning docs

