---
type: reference
status: specified
---

# Map and route experience

## Map and UI

- Mobile-first
- Balanced dark UI with charcoal surfaces, green primary actions, and rounded controls
- Create starts with route name, member name, and transport; optional description, password, and sharing policy live under route settings
- Create/join explain that location stays private until the member explicitly starts sharing
- Map style should remain readable outdoors
- Same route screen structure for active and closed routes
- Route screen includes:
  - route header
  - map
  - member bottom sheet with sharing controls first and an explicit member expand/collapse button
- Desktop places members beside the map; mobile uses a full-width map above the sheet
- Route details and owner actions open in a keyboard-accessible modal
- Member rows show initials, persistent member color, transport, owner/self labels, and written presence status
- Stale, offline, spectating, tracking, and left states must remain distinguishable without relying on color
- Closed archives use the same layout without location-sharing controls
- Incoming `route_closed` events transition connected route screens into archive mode without a snapshot refresh
- Route code is visible but secondary to share action
- Share uses native Web Share API when available, with copy-link fallback and a selectable URL when automatic sharing/copying fails

## Map Behavior

- Initial load fits full known route history plus active markers
- Default live viewport mode auto-fits group/route
- Manual pan/zoom disables auto-follow
- Live position updates preserve manual pan/zoom until the user presses `Fit group`
- User can re-center/re-fit with the map `Fit group` control
- Map resizing after sheet or viewport changes preserves manual pan/zoom; automatic fitting remains active until manual interaction
- An empty map explains when locations will appear; the main map does not expose an internal point counter
- Map loading/rendering failures show a readable message without blocking the member controls
- Fit animations respect reduced-motion preferences
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
