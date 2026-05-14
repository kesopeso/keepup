# KeepUp Implementation Plan

## Phase 1: Repository and Local Dev

1. Create monorepo structure:
   - `apps/web`
   - `apps/api`
   - `db/migrations`
   - `docs`
2. Add Docker-based development:
   - Next.js dev container
   - Go API dev container
   - Postgres/PostGIS container
3. Make `docker compose up` the main local workflow

## Phase 2: Backend Skeleton

1. Scaffold Go API
2. Add config, logging, health endpoint
3. Add DB connection and migration runner
4. Define core schemas:
   - routes
   - route_members
   - path_segments
   - position_points
   - owner/member tokens

## Phase 3: Core Route Lifecycle APIs

1. Create route
2. Route access metadata
3. Join route
4. Edit route metadata
5. Leave route
6. Close route
7. Delete route
8. Fetch route snapshot

## Phase 4: Realtime Layer

1. WebSocket authentication by member token
2. Route room subscription
3. Membership/status events
4. Position update ingestion
5. Server-side tracking permission checks
6. Stale/disconnect handling

## Phase 5: Frontend Skeleton

1. Scaffold Next.js app
2. Add balanced dark mobile-first layout
3. Add local identity storage:
   - alias
   - clientId
   - preferred transport mode
4. Add route create flow
5. Add route join-by-link flow

## Phase 6: Route Screen

1. MapLibre integration
2. Route header with share action and route code
3. Member bottom sheet
4. Snapshot rendering
5. Live marker/path updates
6. Tracking controls
7. Refresh recovery prompt

## Phase 7: Validation and UX Rules

1. Route-local alias uniqueness
2. Password-protected join flow
3. Active tracker limit enforcement
4. Explicit close/delete confirmations
5. Clear tracking/spectator/stale/offline indicators
6. Keep-page-open and battery/data messaging

## Phase 8: Observability

1. Structured logs
2. Snapshot size logging
3. Position acceptance/rejection logging
4. Basic request/live metrics

## Suggested First Implementation Slice

Build the thinnest end-to-end path first:

1. create route
2. join route
3. fetch snapshot
4. open live connection
5. start sharing
6. broadcast position updates
7. draw paths on the map
8. close route into archive

Then fill in:

- password protection
- delete flow
- recovery prompt
- tracker limit
- member statuses

## Current Status

The backend foundation and first route lifecycle slice are complete:

1. `apps/web` is scaffolded as a Next.js app
2. `apps/api` is scaffolded as a Go service with:
   - config loading
   - PostgreSQL connectivity
   - DB-backed health checks
   - route lifecycle REST endpoints
3. `docker compose up` brings up web, api, and postgres together
4. Manual `golang-migrate` up/down migrations exist for the core schema
5. The current route API surface includes:
   - `POST /routes`
   - `GET /routes/{code}/access`
   - `POST /routes/{code}/members`
   - `GET /routes/{code}`
   - `PATCH /routes/{code}`
   - `DELETE /routes/{code}`
   - `DELETE /routes/{code}/members/me`
6. Frontend browser identity storage is implemented in `apps/web/lib/identity-storage.ts` for:
   - `clientId`
   - `displayName`
   - preferred `transportMode`
   - route-scoped `memberToken`
   - route-scoped `ownerToken`
7. Frontend create route flow is implemented:
   - root page renders a mobile-first create route form
   - form posts to `POST /routes`
   - browser profile is saved before creation
   - returned `memberToken` and `ownerToken` are stored by route code
   - successful creation navigates to `/routes/{code}`
8. `/routes/{code}` exists as the minimal route landing surface and checks saved member access
9. Frontend join route flow is implemented:
   - route pages fetch `GET /routes/{code}/access` when no member token is saved
   - join form posts to `POST /routes/{code}/members`
   - browser profile is saved before joining
   - returned `memberToken` is stored by route code
   - saved member access skips the join form
10. Authenticated route bootstrap is implemented:
   - route pages call `GET /routes/{code}` with the saved member token
   - invalid saved member access is cleared and returns to the join flow
   - the first route screen shell renders route metadata, viewer capabilities, and members
11. Route screen layout refinement is implemented:
   - authenticated route view has a route header
   - map area has stable pre-MapLibre dimensions
   - member bottom sheet renders route metadata, viewer capabilities, and sorted members
12. Map display abstraction is implemented:
   - `RouteMapRenderer` owns rendering and viewport commands only
   - snapshot DTOs are normalized into `RouteMapState`
   - route screen renders through the React map wrapper
13. MapLibre route rendering is implemented:
   - `maplibre-gl` is installed for the web app
   - tile provider configuration is isolated from renderer logic
   - renderer factory returns the MapLibre adapter
   - snapshot paths render as member-colored polylines
   - latest member points render as member-colored markers
   - initial and auto-follow viewport mode fits visible route geometry
   - manual map interaction disables auto-follow across live position updates until `Fit` is pressed again
14. Backend WebSocket authentication and route room subscription are implemented:
   - `GET /ws` upgrades to WebSocket
   - client must send first-message auth with `{ "type": "authenticate", "memberToken": "..." }`
   - first-message auth deadline is configured by `WEBSOCKET_AUTH_TIMEOUT`
   - default auth timeout is `5s`
   - valid member tokens subscribe the live connection to an in-memory route room
   - server sends `connection_established` after successful subscription
15. Backend live event broadcast primitives are implemented:
   - route room subscriptions expose buffered event channels
   - the live hub broadcasts events to active subscriptions by route ID
   - authenticated WebSocket connections forward broadcast events to clients
   - route join, leave, metadata update, and close mutations publish lifecycle events
16. Backend sharing state mutation is implemented:
   - authenticated WebSocket `start_sharing` and `stop_sharing` commands update sharing state
   - enabling sharing enforces route status, sharing policy, current member status, and tracking slot limits
   - enabling sharing updates the member to `tracking`, opens a path segment, and broadcasts `member_started_sharing`
   - disabling sharing updates the member to `spectating`, closes open path segments, and broadcasts `member_stopped_sharing`
   - the old REST sharing endpoint has been removed from the target MVP API
17. Frontend tracking controls are wired to backend sharing state:
   - the authenticated route screen keeps the saved member token available while rendering a snapshot
   - the member bottom sheet shows a start/stop sharing action from viewer capabilities
   - starting and stopping sharing send WebSocket commands
   - the screen updates local member/viewer state from live events without refreshing the snapshot
18. Backend WebSocket position ingestion is implemented:
   - authenticated live clients send `position_update` messages over the existing route WebSocket
   - the route service validates active route status, current tracking state, coordinates, and open path segment persistence
   - accepted points are stored in `position_points`
   - accepted points are broadcast to route subscribers as `position_updated`
   - invalid or disallowed points return `position_rejected` to the sender
19. Frontend live tracking updates are implemented:
   - authenticated route screens open a WebSocket live connection after active snapshot load
   - tracking viewers send browser geolocation samples as `position_update` messages over the live connection
   - accepted `position_updated` events append to the route map state for live marker and path updates without a snapshot refresh
20. Snapshot route history loading is implemented:
   - authenticated snapshots include persisted path segments
   - snapshot path segments include persisted position points ordered by sequence
   - route screen map state preserves already-rendered live points across local sharing status updates
21. Realtime member presence and WebSocket sharing commands are implemented:
   - active route screens use WebSocket `start_sharing` and `stop_sharing` commands instead of the old REST sharing endpoint
   - command responses use `command_ack` and `command_rejected`
   - the live hub rejects a second active live connection for the same route member
   - offline members become spectating after live authentication and emit `member_back_online`
   - tracking members become stale immediately on live disconnect or after the configured accepted-position timeout
   - stale members become offline after the configured stale timeout and close open path segments with reason `disconnected`
   - stale members recover to tracking automatically when an accepted position arrives, with `member_back_online` broadcast before `position_updated`
   - spectating members become offline after the configured spectator disconnect grace period
   - active-route owners cannot leave; they must close/delete the route instead

## Immediate Next Step

When work resumes, continue the realtime tracking slice with frontend recovery polish and edge-case hardening:

1. add the current-viewer stale recovery prompt:
   - Resume sharing
   - Continue as spectator
2. add explicit close/delete confirmations
3. add clearer offline/stale visual treatment in the member sheet
