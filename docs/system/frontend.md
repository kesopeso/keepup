---
type: reference
status: active
---

# Frontend boundaries

## Frontend

- Next.js
- TypeScript
- MapLibre GL JS
- Mobile-first route experience
- Native share integration where available

### Main Screens

- Create route
- Join route
- Live route view
- Closed route archive view

### Frontend State Boundaries

- Route snapshot from REST
- Live events from WebSocket
- Local browser identity and per-route tokens from local storage
- Map adapter separated from tile provider config

### Current Frontend Foundation

- Browser identity helpers live in `apps/web/lib/identity-storage.ts`
- Route API helpers live in `apps/web/lib/routes-api.ts`
- The root page renders the first create-route flow and posts to `POST /routes`
- Successful create responses save route-scoped member/owner tokens before navigating to `/routes/{code}`
- `/routes/{code}` checks for saved member access before showing the join flow
- Browsers without saved access fetch `GET /routes/{code}/access`, then join with `POST /routes/{code}/members`
- Successful join responses save the route-scoped member token before fetching the authenticated snapshot
- Browsers with saved member access fetch `GET /routes/{code}` with `Authorization: Bearer <memberToken>`
- Unauthorized snapshot responses clear route-scoped auth and fall back to the join flow
- The authenticated route screen uses a route header, MapLibre-backed map surface, and member bottom sheet
- The member bottom sheet renders route metadata, viewer capabilities, and the snapshot member list
- The member bottom sheet uses viewer capabilities to show a start/stop sharing action, sends WebSocket sharing commands, then updates local member/viewer state from live events without refreshing the authenticated snapshot
- The authenticated route screen opens an authenticated WebSocket live connection for active routes with saved member access
- Active tracking viewers stream browser geolocation samples as `position_update` messages over the live connection
- Incoming `position_updated` events update the in-memory map state so live markers and paths move without refetching the snapshot
- Incoming sharing and presence status events update local member/viewer state without replacing rendered route paths
- Active route snapshots that initially load with a `stale` current viewer show a blocking recovery prompt; later stale events in the same live session do not trigger it
- The recovery actions wait for `connection_established`; resume requests browser location permission and sends `start_sharing`, while continue-as-spectator sends `stop_sharing`
- The stale recovery prompt is dismissed by the resulting member status event rather than optimistically on command acknowledgement
- Owner route management uses the route-scoped owner token with `PATCH /routes/{code}` for close and `DELETE /routes/{code}` for permanent deletion
- Close uses an explicit confirmation and applies the resulting `route_closed` event to connected clients; delete requires exact route-code confirmation, clears local route auth, and navigates the deleting owner to the create screen
- If a second tab/device opens the same active route with the same member token, the route screen shows a blocking notice instead of map/member content after `live_connection_rejected`
- Browser position access is isolated behind `apps/web/lib/navigation-service.ts`
- Browser geolocation failures are normalized by the navigation service into denied, unavailable, timeout, and generic failure codes before the route screen renders actionable guidance
- In development, the navigation service emits the first real browser position, then broadcasts simulated movement every 2 seconds in roughly 10m direction-biased steps with small random turns
- Map rendering is behind a framework-neutral `RouteMapRenderer` interface in `apps/web/lib/map`
- Snapshot DTOs are converted to a map-specific `RouteMapState` before reaching the renderer
- The renderer factory returns a MapLibre adapter that consumes route paths and latest member points from `RouteMapState`
- Tile provider configuration lives separately from the renderer in `apps/web/lib/map/tile-provider.ts`
- The MapLibre adapter renders historical path polylines and latest member point markers, fits the initial viewport to visible route geometry, and switches to manual viewport mode on map interaction so live updates do not reset user pan/zoom until `Fit` is pressed again
- The helper owns local storage access for:
  - stable `clientId`
  - saved `displayName`
  - preferred `transportMode`
  - per-route `memberToken`
  - per-route `ownerToken`
- Route codes are normalized to uppercase before reading or writing route-scoped auth
- The helper guards server rendering by returning safe defaults when browser storage is unavailable


## Map Abstraction

Keep two abstraction layers:

1. tile provider config
2. map renderer adapter

This allows switching basemap providers without rewriting route rendering logic.


Sources: [route screen](../../apps/web/app/routes/[code]/join-route-screen.tsx), [API client](../../apps/web/lib/routes-api.ts), [navigation service](../../apps/web/lib/navigation-service.ts), [map adapter](../../apps/web/lib/map/maplibre-route-map-renderer.ts).
