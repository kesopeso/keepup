---
type: plan
status: planned
---

# Implementation sequence

These are planning phases, not completion checkboxes. See [current status](status.md) before selecting work.

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

## Original first slice

The initial create → join → snapshot → live tracking → archive slice is implemented. Its detailed behavior lives in [system design](../architecture.md).

## Phase 9: Mobile client, Android first

The owner selected Android development now, with iOS later. The starter, tooling, API connection check, route joining, secure membership storage, authenticated snapshot screen, native snapshot map, and live viewing are complete; see [mobile delivery status](status.md#mobile-foundation). The app approach is owned by [mobile boundaries](../system/mobile.md).

The implementation checkpoints are:

1. Live viewing, complete. Authenticate with saved membership, update paths and markers automatically, reflect membership and route changes, and recover missed events after reconnection. Preserve manual map positioning. Verify with a second client and an interrupted connection.
2. Foreground location sharing, implemented with physical review pending. Request Android location permission, add Start/Stop sharing, and submit accuracy and client timestamps through the existing protocol. Show permission failures, GPS rejection feedback, and stale recovery. Verify on a physical phone with the app open.
3. Screen-off tracking, implemented with physical review pending. Android background tracking and its persistent notification are implemented. Validate screen locking, permissions, network loss and recovery, and stop/restart on a physical phone. Record reliability and battery observations.
4. Extend mobile product behavior using the shared product references; schedule iOS separately in the [backlog](backlog.md#planned-later).

Each task is a separate implementation and validation checkpoint. Live viewing is complete. Foreground sharing needs its physical-phone review before the tracking reliability claims are complete. All three implementation slices are implemented. Physical-phone tracking and battery review remain before closing their validation checkpoints.
