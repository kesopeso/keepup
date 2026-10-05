---
type: reference
status: active
---

# Frontend boundaries

This page owns the web client. See [mobile boundaries](mobile.md) for the React Native client.

## Frontend

- Next.js
- TypeScript
- MapLibre GL JS
- Mobile-first route experience
- Native share integration where available

Production packaging uses [Next.js standalone output](../../apps/web/next.config.ts) and a [multi-stage Dockerfile](../../apps/web/Dockerfile). See the [build and deployment commands](../workflow/development.md#production-web-image).

Browser REST calls use same-origin `/api` paths. Live connections compute `/ws` from the browser origin when opening a connection, using `wss` on HTTPS pages. Nginx owns [upstream routing](../workflow/development.md#same-origin-proxy), so frontend images need no environment-specific URL configuration.

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
- The authenticated route screen uses a route header with the K4-10 logo and KeepUp wordmark beside the route title, a MapLibre-backed map surface, and a member bottom sheet
- The member bottom sheet renders sharing state/actions first, then a collapsible member list with initials, member colors, and explicit presence badges
- Route details and owner controls live in a native modal dialog with keyboard focus containment and restoration
- Shared brand, transport selector, status badge, and modal components live in `apps/web/app/components/ui.tsx`
- Create uses a settings disclosure for description, password, and sharing policy; its collapsed summary shows the selected policy and password protection
- The route header shares the route with native Web Share, clipboard fallback, or a selectable link if automatic sharing fails. Clipboard success and manual-copy fallback add popup notification cards. Up to three cards peek behind the newest one; each exits after its own six-second timer. The stack sits below the header on desktop and above the bottom edge on mobile.
- The map wrapper shows a member/sharing summary, an empty-history message, and a `Fit group` control instead of a point counter
- The `Fit group` button fades out and becomes unavailable in automatic fit mode, then fades back in after manual map interaction. No map-movement status label appears. The fade respects reduced-motion preferences.
- The member bottom sheet uses viewer capabilities to show a start/stop sharing action, sends WebSocket sharing commands, then updates local member/viewer state from live events without refreshing the authenticated snapshot
- The authenticated route screen opens an authenticated WebSocket live connection for active routes with saved member access
- Unexpected live-connection closures reconnect with exponential backoff; sharing controls remain unavailable only until the replacement connection authenticates.
- Active tracking viewers stream browser geolocation samples as `position_update` messages over the live connection
- Incoming `position_updated` events update the in-memory map state so live markers and paths move without refetching the snapshot
- Rejected GPS samples show guidance for the specific validation code. The next accepted `position_updated` for the current viewer clears the location error; other members' updates leave it visible. Confirmed stop-sharing clears the viewer's location error, and late rejections are ignored when the viewer is no longer tracking or stale.
- Incoming `member_joined` events insert the new member into local snapshot/map state immediately; repeated member events upsert by member ID instead of creating duplicates
- Incoming leave, sharing, stale, online, and offline events update local member/viewer state without replacing rendered route paths
- Active route snapshots that initially load with a `stale` current viewer show a blocking recovery prompt; later stale events in the same live session do not trigger it
- The recovery actions wait for `connection_established`; resume requests browser location permission and sends `start_sharing`, while continue-as-spectator sends `stop_sharing`
- The stale recovery prompt is dismissed by the resulting member status event rather than optimistically on command acknowledgement
- Owner route management uses the route-scoped owner token with `PATCH /routes/{code}` for close and `DELETE /routes/{code}` for permanent deletion
- Close uses an explicit confirmation and applies the resulting `route_closed` event to connected clients; delete requires exact route-code confirmation, clears local route auth, and navigates the deleting owner to the create screen
- If a second tab/device opens the same active route with the same member token, the route screen shows a blocking notice instead of map/member content after `live_connection_rejected`
- Browser position access is isolated behind `apps/web/lib/navigation-service.ts`
- Browser geolocation failures are normalized by the navigation service into denied, unavailable, timeout, and generic failure codes before the route screen renders actionable guidance
- In development, sharing bypasses browser location permission and real GPS. The navigation service starts at simulated coordinates `46.0569, 14.5058`, emits a sample immediately with `5m` accuracy, and broadcasts simulated movement every 2 seconds in roughly 10m direction-biased steps with small random turns. Stopping cancels the timer; restarting retains the last simulated coordinates, refreshes the timestamp, and immediately resumes samples.
- Map rendering is behind a framework-neutral `RouteMapRenderer` interface in `apps/web/lib/map`
- Snapshot DTOs are converted to a map-specific `RouteMapState` before reaching the renderer
- The renderer factory returns a MapLibre adapter that consumes route paths and latest member points from `RouteMapState`
- A ResizeObserver in the MapLibre adapter resizes the map when the member sheet or viewport changes, preserves manual positioning, and refits only in automatic mode; it disconnects on renderer teardown
- Optional renderer readiness/error callbacks let the map wrapper explain failed initialization or tile loading without an unhandled rejection
- Map fit animations honor the browser reduced-motion preference
- Tile provider configuration lives separately from the renderer in `apps/web/lib/map/tile-provider.ts`
- The MapLibre adapter renders historical path polylines and latest member point markers, fits the initial viewport to visible route geometry, and switches to manual viewport mode on map interaction so live updates do not reset user pan/zoom until `Fit group` is pressed again
- The helper owns local storage access for:
  - stable `clientId`
  - saved `displayName`
  - preferred `transportMode`
  - per-route `memberToken`
  - per-route `ownerToken`
- Route codes are normalized to uppercase before reading or writing route-scoped auth
- The helper guards server rendering by returning safe defaults when browser storage is unavailable
- The KeepUp identity uses the selected K4-10 Waypoint K mark, with one white circle on the lower green route. The same mark appears in the web header, browser icon, install manifest icons, Apple touch icon, and Android icon assets. Master SVG and platform exports live under `apps/web/public/brand`, `apps/web/public/icons`, and `assets/brand`.


## Map Abstraction

Keep two abstraction layers:

1. tile provider config
2. map renderer adapter

This allows switching basemap providers without rewriting route rendering logic.


Sources: [route screen](../../apps/web/app/routes/[code]/join-route-screen.tsx), [API client](../../apps/web/lib/routes-api.ts), [navigation service](../../apps/web/lib/navigation-service.ts), [map adapter](../../apps/web/lib/map/maplibre-route-map-renderer.ts).
