# KeepUp Agent Notes

## Documentation Discipline

- Treat files in `docs/` as project memory and source-of-truth planning artifacts.
- On completion of any meaningful task, update the relevant files in `docs/` in the same change.
- Do not wait for the user to remind you to refresh docs after implementation, architecture, API, workflow, or scope changes.
- If a change affects product behavior, update `docs/mvp-spec.md`.
- If a change affects system design, data flow, API shape, or infrastructure workflow, update `docs/architecture.md`.
- If a change affects sequencing or what should be built next, update `docs/implementation-plan.md`.
- If a change defers work or introduces future work, update `docs/backlog.md`.

## Current Workflow

- Local development runs via `docker compose up`.
- All application tooling runs through Docker Compose services.
- Do not run host `pnpm`, `npm`, Go, or other app toolchain commands directly; use Docker Compose instead.
- Database migrations are manual via `golang-migrate`.
- The API must not auto-apply migrations on startup.
