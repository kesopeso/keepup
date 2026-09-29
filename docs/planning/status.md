---
type: plan
status: active
---

# Delivery status

## Immediate next step

Review the refreshed UI on a physical phone outdoors, including map readability, location permissions, and native sharing.

## UI redesign

The user selected the full layout refresh in the [Lavish visual proposal](../../.lavish/keepup-redesign.html), then requested implementation. The app now uses simpler create/join forms, a map-led route layout, sharing controls above collapsible members, explicit presence badges, and a route-details dialog. Behavior is owned by [map experience](../product/experience.md); implementation boundaries by [frontend](../system/frontend.md).

Validation on 2026-09-26:

- Production web build and all four existing live-state tests passed through Docker Compose.
- Chromium checks passed for create, password errors and protected join, live member insertion, start/stop sharing, clipboard sharing, duplicate connections, close/archive transitions, and code-confirmed deletion using disposable local routes.
- Isolated browser fixtures covered all five presence states, long names, restricted sharing, full tracking slots, stale recovery, member collapse, map resizing, and manual viewport preservation.
- Layout checks passed at 360px, 390px, 640px, 844px landscape, and 1280px. Automated accessibility checks found no violations on create, live route, or the populated member sheet. Nested dialogs restore focus and close confirmation initially focuses Cancel.
- Physical-phone outdoor readability and native share sheets still need a manual review. The existing basemap provider is unchanged.

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
| UI refresh, simpler entry forms, sharing, accessible dialogs, and presence styling | [Experience](../product/experience.md), [frontend](../system/frontend.md) |
| K4-10 logo in the header and platform icon exports | [Frontend](../system/frontend.md) |
| Owner close/delete confirmations | [Frontend](../system/frontend.md) |

This consolidates the previous 23-item implementation status without making the plan a second behavior specification. It records existing project status; the documentation restructure did not run application tests.

## Specification versus implementation

The [GPS requirements](../product/tracking.md#gps-validation) include accuracy thresholds, timestamp duplicate rejection, and impossible-jump checks. The current [normalizer](../../apps/api/internal/routes/service.go) checks coordinate ranges and numeric metadata. Those additional filters must not be assumed implemented; track the gap in the [backlog](backlog.md).

## Knowledge-base maintenance

The Markdown knowledge base is organized by ownership with folder contracts and stable entry paths. See the [migration record](icm-migration.md).
