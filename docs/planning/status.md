---
type: plan
status: active
---

# Delivery status

## Immediate next step

Continue presence visual polish: add clearer offline/stale visual treatment in the member sheet.

## Implemented areas

| Area | Owning reference |
|---|---|
| Compose stack, API startup, health checks, manual migrations | [Development](../workflow/development.md), [backend](../system/backend.md) |
| Create/join, browser identity, authenticated snapshots | [Frontend](../system/frontend.md) |
| Route lifecycle REST operations and owner controls | [API](../system/api-and-live.md), [frontend](../system/frontend.md) |
| MapLibre rendering, viewport control, snapshot and live path state | [Frontend](../system/frontend.md) |
| WebSocket authentication, sharing commands, position ingestion | [API and live protocol](../system/api-and-live.md) |
| Saved segments/points, snapshot history | [Persistence](../system/data.md) |
| Presence transitions, duplicate connection rejection, stale recovery | [API](../system/api-and-live.md), [frontend](../system/frontend.md) |
| Owner close/delete confirmations | [Frontend](../system/frontend.md) |

This consolidates the previous 23-item implementation status without making the plan a second behavior specification. It records existing project status; the documentation restructure did not run application tests.

## Specification versus implementation

The [GPS requirements](../product/tracking.md#gps-validation) include accuracy thresholds, timestamp duplicate rejection, and impossible-jump checks. The current [normalizer](../../apps/api/internal/routes/service.go) checks coordinate ranges and numeric metadata. Those additional filters must not be assumed implemented; track the gap in the [backlog](backlog.md).

## Knowledge-base maintenance

The Markdown knowledge base is organized by ownership with folder contracts and stable entry paths. See the [migration record](icm-migration.md).
