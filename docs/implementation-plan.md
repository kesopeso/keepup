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
4. open websocket
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
