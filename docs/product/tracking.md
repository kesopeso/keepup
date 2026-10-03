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

Every position submission passes shared API validation before it can affect history or presence.

- Require finite latitude in `[-90, 90]` and longitude in `[-180, 180]`.
- Require finite, nonnegative `accuracyM`. Reject values above the configured maximum, default `100m`, including the first point of a segment. Equality is accepted.
- Require `clientRecordedAt` with a timezone. Normalize to UTC and microsecond precision. Reject samples older than `2m` or more than `30s` in the future relative to server receive time; both limits are configurable and equality is accepted.
- Within a segment, reject timestamps equal to the latest accepted timestamp as duplicates and earlier timestamps as out of order.
- Reject impossible jumps relative to the latest accepted point in the same segment. Subtract both accuracy radii from geographic distance, clamp to zero, and divide by elapsed client measurement time. Reject speeds above the configurable default `400m/s`; equality is accepted. This generous ceiling accommodates airplane mode and does not enforce transport-specific speeds.
- Optional altitude must be finite, speed finite and nonnegative, and heading finite in `[0, 360]`.
- The first accepted point establishes the segment baseline. Brief reconnects retain it; a new segment resets it. Legacy points without a client timestamp cannot supply a measurement-time baseline.
- Rejected points and their payloads are not stored, consume no sequence number, and do not refresh tracking health or recover stale members. Later valid points can recover normally.
- Keep the live connection open for malformed position fields and GPS quality rejections.
- Show guidance for the rejection reason and clear the location error when the viewer's next sample is accepted. Other members' accepted samples do not clear it. Confirmed stop-sharing also clears the error; late rejections do not show an error for spectators.

Server receive time remains canonical for history; client time measures elapsed movement. Accepted raw coordinates and payloads remain the source of truth. No offline buffering or path snapping is introduced.

Configuration and integration test commands: [development workflow](../workflow/development.md#gps-validation). Rejection codes and transaction behavior: [API and live protocol](../system/api-and-live.md#gps-validation).

See [system design](../architecture.md) for implementation and [delivery status](../planning/status.md) for progress.
