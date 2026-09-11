---
type: reference
status: specified
---

# Map and route experience

## Map and UI

- Mobile-first
- Balanced dark UI
- Map style should remain readable outdoors
- Same route screen structure for active and closed routes
- Route screen includes:
  - route header
  - map
  - member bottom sheet
- Incoming `route_closed` events transition connected route screens into archive mode without a snapshot refresh
- Route code is visible but secondary to share action
- Share uses native Web Share API when available, with copy-link fallback

## Map Behavior

- Initial load fits full known route history plus active markers
- Default live viewport mode auto-fits group/route
- Manual pan/zoom disables auto-follow
- Live position updates preserve manual pan/zoom until the user presses `Fit`
- User can re-center/re-fit with the map `Fit` control
- Path polyline and live marker are separate render states
- Show:
  - polyline for historical path
  - live marker for active trackers
- Do not show per-segment start/end markers in MVP

## Data and Timing

- Store exact timestamps in UTC
- Client displays localized times
- API snapshot returns full route history and current statuses
- Snapshot also returns current viewer capability booleans
- Return full snapshot for MVP; no chunked history yet

See [system design](../architecture.md) for implementation and [delivery status](../planning/status.md) for progress.
