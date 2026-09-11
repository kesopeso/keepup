---
type: reference
status: active
---

# Change-impact guide

This is a compact edit map. Follow the owning reference and source links; it does not define a second specification.

| Change | Read first | Direct code surfaces | Usually unaffected |
|---|---|---|---|
| Route access, owner actions, membership | [Routes](../product/routes.md), [API](api-and-live.md) | [Service](../../apps/api/internal/routes/service.go), [repository](../../apps/api/internal/routes/postgres.go), [HTTP](../../apps/api/internal/httpapi/server.go), [client](../../apps/web/lib/routes-api.ts), [screen](../../apps/web/app/routes/[code]/join-route-screen.tsx) | Tile provider for lifecycle-only changes |
| Presence, sharing, position messages | [Tracking](../product/tracking.md), [protocol](api-and-live.md) | [HTTP/presence timers](../../apps/api/internal/httpapi/server.go), [hub](../../apps/api/internal/live/hub.go), [service](../../apps/api/internal/routes/service.go), [repository](../../apps/api/internal/routes/postgres.go), [screen](../../apps/web/app/routes/[code]/join-route-screen.tsx) | Tile configuration for status-only changes |
| Member status styling | [Experience](../product/experience.md), [frontend](frontend.md) | [Screen](../../apps/web/app/routes/[code]/join-route-screen.tsx), [CSS](../../apps/web/app/globals.css) | DB schema and protocol if status semantics stay the same |
| Map rendering / basemap | [Frontend](frontend.md) | [Renderer](../../apps/web/lib/map/maplibre-route-map-renderer.ts), [tile provider](../../apps/web/lib/map/tile-provider.ts), [map wrapper](../../apps/web/app/components/route-map.tsx) | Stored raw points for visual-only changes |
| Persistence / snapshot shape | [Data](data.md), [API](api-and-live.md) | [Migrations](../../db/migrations/), [repository](../../apps/api/internal/routes/postgres.go), [DTOs](../../apps/api/internal/routes/models.go), [client types](../../apps/web/lib/routes-api.ts), [map conversion](../../apps/web/lib/map/snapshot-map-state.ts) | Basemap vendor unless rendering requirements change |
| Local tooling | [Development](../workflow/development.md) | [Compose](../../docker-compose.yml), [Makefile](../../Makefile), [helpers](../../bin/) | Product rules |

Evidence boundary: these are file-level edit routes based on inspected source, not a complete dependency analysis. Reread the affected code and existing tests before changing behavior. External consumers of public APIs or document anchors cannot be established by a repository-only scan.
