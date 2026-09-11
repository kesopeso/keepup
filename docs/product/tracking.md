---
type: reference
status: specified
---

# Tracking and presence

## Tracking

- Members explicitly press `Start sharing location`
- Starting sharing requires usable location access
- Location permission failures show actionable browser/device guidance and distinguish blocked permission, unavailable location, and timeout failures
- Route creation does not require location access
- Stopping sharing returns member to spectator state
- Sharing opens a path segment; stopping ends it. See [commands and position ingestion](../system/api-and-live.md#tracking-rules) for protocol behavior.
- On refresh, if a member was previously sharing:
  - rejoin route automatically
  - show prompt:
    - Resume sharing
    - Continue as spectator
- The stale recovery prompt waits for the authenticated live connection before enabling either action
- Resume sharing requests browser location permission, then sends `start_sharing`
- Continue as spectator sends `stop_sharing`
- The prompt remains visible until the server broadcasts the viewer's resulting member status
- No offline buffering in MVP
- No road/path snapping in MVP
- Path rendering is point-to-point between accepted positions

## Member Statuses

- `Owner`
- `Tracking`
- `Stale`
- `Spectating`
- `Offline`
- `Left`

Persistent member status semantics:

- `spectating`: active membership, connected/recent enough, not sharing location
- `tracking`: active membership, sharing location successfully
- `stale`: active membership, intended to share, but live connection or accepted position flow is interrupted
- `offline`: active membership, no active live connection/presence
- `left`: terminal membership created by explicit leave; token is revoked

Member sort order on active route:

1. Owner
2. Tracking
3. Stale
4. Spectating
5. Offline
6. Left

Within the same status group, sort by join time.

## GPS Validation

- Reject invalid coordinates
- Reject too-inaccurate first/live points based on configurable threshold
- Reject duplicate timestamp duplicates
- Reject impossible jumps using a generous speed threshold
- Do not store rejected points in MVP

See [system design](../architecture.md) for implementation and [delivery status](../planning/status.md) for progress.
