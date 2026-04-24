# KeepUp

KeepUp is a mobile-first web app for live route sharing. A route owner creates a route, shares a link/code, and other members can join as spectators or trackers depending on route policy. Active routes show live positions and path history on a map; closed routes remain available as read-only archives.

## Stack

- Frontend: Next.js, TypeScript
- Backend: Go
- Database: PostgreSQL + PostGIS
- Realtime: WebSockets
- Local development: Docker Compose

## Docs

- [MVP Spec](./docs/mvp-spec.md)
- [Architecture](./docs/architecture.md)
- [Implementation Plan](./docs/implementation-plan.md)
- [Backlog](./docs/backlog.md)

## Database Migrations

Migrations are manual and use `golang-migrate`.

- `make migrate-up`
- `make migrate-down`
- `make migrate-drop`
- `make migrate-version`
