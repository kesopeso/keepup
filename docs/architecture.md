# KeepUp Architecture

## Repository Layout

```text
apps/
  web/
  api/
db/
  migrations/
docs/
docker-compose.yml
```

## Local Development Workflow

- `docker compose up` starts the main local stack: web, api, and postgres.
- One-off web pnpm commands can run through `bin/web-pnpm.sh`, which uses the dependency-free `web-helper` Compose service.
- Example: `./bin/web-pnpm.sh build`

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
- The member bottom sheet uses viewer capabilities to show a start/stop sharing action, calls the sharing state endpoint, then updates local member/viewer state without refreshing the authenticated snapshot
- The authenticated route screen opens an authenticated WebSocket live connection for active routes with saved member access
- Active tracking viewers stream browser geolocation samples as `position_update` messages over the live connection
- Incoming `position_updated` events update the in-memory map state so live markers and paths move without refetching the snapshot
- Incoming sharing status events update local member/viewer state without replacing rendered route paths
- Browser position access is isolated behind `apps/web/lib/navigation-service.ts`
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

## Backend

- Go service
- REST API for lifecycle and snapshots
- WebSocket server for live route events
- PostgreSQL + PostGIS for persistence

### Suggested Go Stack

- Router: `chi`
- WebSocket: `nhooyr.io/websocket` or equivalent
- Postgres: `pgx`
- Queries: `sqlc`
- Migrations: `golang-migrate`
- Logging: `slog`

### Current Backend Foundation

- API config is loaded from environment variables
- PostgreSQL connectivity is established on startup with retry/backoff inside a bounded startup window
- API dev tooling runs inside the Docker Compose API service, including `golangci-lint`
- `/livez` reports process liveness
- `/healthz` checks database reachability
- HTTP server shutdown is tied to process signal cancellation
- Migrations are manual and are not applied automatically on API startup

## Ubiquitous Language

- Route: one tracking session and the top-level aggregate
- Route code: public short identifier for a route
- Route member: a participant record scoped to one route
- Membership: creation of a route member
- Route owner: a route member with management authority
- Spectator: member state without active location sharing
- Tracker: member state with active location sharing
- Access metadata: public route pre-join information
- Snapshot: authenticated route bootstrap payload used to render the route screen
- Archive: closed route state with preserved history
- Live connection: one authenticated WebSocket connection for a route member
- First-message authentication: the required first WebSocket message that authenticates a live connection with a member token
- WebSocket authentication timeout: the maximum time a live connection may remain unauthenticated after opening
- Live hub: server-side coordinator that tracks live connections by route room
- Route room: live fan-out group for all live connections subscribed to one route
- Route room subscription: membership of one live connection in one route room
- Live event: server-published realtime fact sent to route room subscribers
- Position update: live event carrying one accepted location point for a tracker

## Service Responsibilities

### REST

- create route
- inspect route access requirements
- create membership
- fetch route snapshot
- edit route metadata
- leave route
- close route
- delete route

Current path shape:

- `POST /routes`
- `GET /routes/{code}/access`
- `POST /routes/{code}/members`
- `GET /routes/{code}`
- `PATCH /routes/{code}`
- `DELETE /routes/{code}`
- `DELETE /routes/{code}/members/me`
- `PUT /routes/{code}/members/me/sharing`

### WebSocket

- accept `GET /ws` and require the first client message to authenticate with a member token
- close unauthenticated live connections if the auth message does not arrive before `WEBSOCKET_AUTH_TIMEOUT`
- subscribe connection to route live events
- receive `position_update`
- publish live membership/status updates

Business logic must not live only in the WebSocket handlers. Tracking rules belong in application services.

Current live foundation:

- The backend owns an in-memory live hub in `apps/api/internal/live`
- A live connection must first send `{ "type": "authenticate", "memberToken": "..." }`
- `WEBSOCKET_AUTH_TIMEOUT` controls the first-message auth deadline and defaults to `5s`
- Authenticated live connections are registered in a route room keyed by route ID
- The server sends `connection_established` with route/member identity after successful auth
- Each route room subscription owns a buffered live event channel
- The live hub can broadcast live events to all active subscriptions in a route room
- Authenticated WebSocket clients send `position_update` messages for live tracking samples
- Accepted position updates are persisted to the member's open path segment and broadcast as `position_updated`
- Invalid or disallowed position updates return `position_rejected` to the sending connection
- REST lifecycle mutations currently broadcast:
  - `member_joined` after a successful join
  - `member_left` after a successful leave
  - `route_updated` after owner metadata updates
  - `route_closed` after owner close
  - `member_started_sharing` after sharing is enabled
  - `member_stopped_sharing` after sharing is disabled
- The current hub is single-process only; Redis-backed presence/pubsub remains deferred until horizontal scale is needed

## Data Model

### routes

- `id`
- `code`
- `name`
- `description`
- `password_hash`
- `sharing_policy`
- `status`
- `max_tracking_members`
- `created_at`
- `closed_at`

### route_members

- `id`
- `route_id`
- `client_id`
- `display_name`
- `transport_mode`
- `is_owner`
- `status`
- `joined_at`
- `left_at`
- `color`

### path_segments

- `id`
- `route_id`
- `member_id`
- `started_at`
- `ended_at`
- `end_reason`

### position_points

- `id`
- `route_id`
- `member_id`
- `segment_id`
- `seq`
- `recorded_at`
- `client_recorded_at`
- `location` (`geography(Point, 4326)`)
- `latitude`
- `longitude`
- `accuracy_m`
- `altitude_m`
- `speed_mps`
- `heading_deg`
- `raw_payload`

### member_tokens

- `id`
- `member_id`
- `token_hash`
- `created_at`
- `revoked_at`

### owner_tokens

- `id`
- `route_id`
- `member_id`
- `token_hash`
- `created_at`
- `revoked_at`

## Snapshot Contract

Snapshot should return:

- route metadata
- route status
- member list
- member current statuses
- member colors and transport modes
- full path history
- latest known live points where relevant
- current viewer capabilities

The route snapshot currently loads persisted path segments and position points. The goal is to render the route page fully before live events arrive.

## Realtime Model

WebSocket event stream should be event-based, not positions-only.

Examples:

- `member_joined`
- `member_started_sharing`
- `member_stopped_sharing`
- `member_became_stale`
- `member_left`
- `position_updated`
- `route_closed`

## Tracking Rules

- Route owner may or may not track
- Starting tracking always requires explicit user action
- Tracking slot count is enforced server-side
- Limit counts only active trackers
- Spectators remain unlimited
- Sharing state is currently updated through `PUT /routes/{code}/members/me/sharing` with an `enabled` boolean payload
- Enabling sharing updates the member to `tracking` and opens a path segment
- Disabling sharing updates the member to `spectating` and ends open path segments with reason `stopped`
- Position updates are ingested through the authenticated WebSocket, not a REST endpoint
- Position ingestion requires an active route, a tracking member, valid latitude/longitude, and an open path segment
- Reconnects inside the grace window keep the same segment
- Prolonged disconnect ends the segment

## Map Abstraction

Keep two abstraction layers:

1. tile provider config
2. map renderer adapter

This allows switching basemap providers without rewriting route rendering logic.

## Storage and Derived Data

- Raw accepted points are the source of truth
- Derived data can be added later:
  - simplified paths
  - snapped paths
  - replay timelines

Never overwrite raw points with derived geometry.

## Migration Workflow

- SQL migrations live under `db/migrations`
- Migrations use `golang-migrate` up/down files
- Migration execution is manual through root `Makefile` commands
- The API service does not mutate schema state automatically on boot

## Observability

Use structured logs from the start. Add metrics for:

- snapshot sizes
- point counts
- request durations
- accepted/rejected position updates
- live connections/events

This is enough to decide later when snapshot chunking, caching, or path derivation is needed.
