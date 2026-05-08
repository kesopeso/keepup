# KeepUp MVP Spec

## Product Summary

KeepUp is a mobile-first web app for live route sharing between friends, groups, or coordinators monitoring participants across different vehicles. Users join routes via shareable links/codes and can watch live progress or share their own location when allowed.

## Core Concepts

- Route owner: user who creates and manages the route
- Route member: any user who joins the route
- Spectator: joined member who is not sharing location
- Tracker: joined member who is actively sharing location
- Route archive: closed route that remains viewable but no longer live

## Ubiquitous Language

- Route: one shareable tracking session
- Route code: the short human-enterable identifier used in URLs and manual entry
- Route owner: the member with management authority over the route
- Route member: a browser/device-specific participant record within a route
- Membership: the relationship created when a browser joins a route
- Spectator: a member who is present but not sharing location
- Tracker: a member who is actively sharing location
- Sharing policy: the rule that determines who may start sharing location
- Access metadata: the public pre-join route data returned by the access endpoint
- Snapshot: the authenticated full route bootstrap payload for rendering the route screen
- Archive: a closed, read-only route that preserves historical data
- Live connection: one authenticated WebSocket connection for a route member
- First-message authentication: the required first WebSocket message that authenticates a live connection with a member token
- WebSocket authentication timeout: the maximum time a live connection may remain unauthenticated after opening
- Live hub: the server-side coordinator that tracks live connections by route room
- Route room: the live fan-out group for all live connections subscribed to one route
- Route room subscription: the membership of one live connection in one route room
- Live event: a server-published realtime fact sent to route room subscribers
- Position update: a live event carrying one accepted location point for a tracker

## Route Access

- Routes are accessible only by share link/code
- Route code is human-friendly, uppercase, case-insensitive
- Route names are required and not unique
- Route descriptions are optional
- Routes may be:
  - open by link/code
  - password-protected
- Password is required only to gain membership
- Returning browsers with valid member tokens do not re-enter the password
- Closed routes remain accessible by link/code and password if protected
- Creating a route collects the owner's display name and transport mode
- Successful route creation stores member and owner tokens for that route in the browser
- Successful route creation takes the owner to the route page for the new route code
- Opening a route page without saved member access fetches access metadata and shows the join flow
- Joining a route collects display name, transport mode, and password when required
- Successful join stores the member token for that route in the browser
- Opening a route page with saved member access fetches the authenticated route snapshot
- Expired or invalid saved member access is cleared and the browser returns to the join flow

Current API naming:

- `POST /routes`
- `GET /routes/{code}/access`
- `POST /routes/{code}/members`
- `GET /routes/{code}`
- `PATCH /routes/{code}`
- `DELETE /routes/{code}`
- `DELETE /routes/{code}/members/me`
- `PUT /routes/{code}/members/me/sharing`

## Membership and Identity

- Users are anonymous for MVP
- Browser stores:
  - `clientId`
  - `displayName`
  - preferred `transportMode`
  - per-route member/owner tokens
- Alias must be unique within a route
- Membership is browser/device-specific
- Every viewer becomes a route member, including spectators
- Members can leave the route
- Leaving preserves history and keeps the member visible as `Left`

## Owner Rules

- Owner is a role, not an automatically tracked participant
- Owner may spectate or track
- Owner may leave and return later
- Owner authority persists via owner token
- Owner can:
  - edit route name/description
  - close route
  - delete route
- Closing requires confirmation
- Deleting requires stronger confirmation
- Closed routes cannot be reopened

## Sharing Policies

- `everyone_can_share`
  - any joined member may start sharing if tracking slots are available
- `joiners_can_view_only`
  - non-owner members are spectators only
  - owner may still choose to track or spectate

## Tracking

- Members explicitly press `Start sharing location`
- Starting sharing requires usable location access
- Route creation does not require location access
- Stopping sharing returns member to spectator state
- Current backend sharing state updates use `PUT /routes/{code}/members/me/sharing` with `{ "enabled": true }` to start sharing and `{ "enabled": false }` to stop sharing
- Starting sharing updates member status to `tracking` and opens a path segment
- Stopping sharing updates member status to `spectating` and closes open path segments
- Active trackers send live position samples over the authenticated WebSocket as `position_update` messages
- The backend accepts WebSocket position updates only from members who are currently `tracking`
- Accepted WebSocket position updates are persisted to the open path segment and broadcast as `position_updated`
- On refresh, if a member was previously sharing:
  - rejoin route automatically
  - show prompt:
    - Continue sharing
    - Continue as spectator
- No offline buffering in MVP
- No road/path snapping in MVP
- Path rendering is point-to-point between accepted positions

## Transport Modes

- Selected per member on create/join
- Allowed values:
  - `walking`
  - `bicycle`
  - `car`
  - `bus`
  - `train`
  - `boat`
  - `airplane`
- Transport mode is fixed for MVP

## Limits

- No spectator limit
- Active tracking member limit exists
- Default active tracking member limit: `10`
- Limit counts only active trackers
- Owner counts only if actively tracking
- If limit is reached, members remain spectators and see an error

## Route Lifecycle

- Active route:
  - members can join
  - live updates run
  - tracking allowed depending on route policy and available slots
- Closed route:
  - no live updates
  - no new tracking sessions
  - read-only archive
  - still viewable by anyone with link/code and password if required
- Deleted route:
  - all related data removed permanently

## Map and UI

- Mobile-first
- Balanced dark UI
- Map style should remain readable outdoors
- Same route screen structure for active and closed routes
- Route screen includes:
  - route header
  - map
  - member bottom sheet
- Current route screen renders a stable MapLibre-backed map surface and the member bottom sheet from snapshot data
- Current member bottom sheet renders start/stop sharing controls from viewer capabilities and updates local member state after sharing changes
- Current map surface renders snapshot path polylines and latest member point markers through the map adapter
- Current route screen opens an authenticated WebSocket live connection for active routes after authenticated snapshot load
- Current tracking viewers stream browser geolocation samples over the authenticated WebSocket
- Current map surface applies accepted `position_updated` events directly to live marker and path state
- Current route snapshots include persisted path segments and position points for route history recovery
- Route code is visible but secondary to share action
- Share uses native Web Share API when available, with copy-link fallback

## Map Behavior

- Initial load fits full known route history plus active markers
- Default live viewport mode auto-fits group/route
- Manual pan/zoom disables auto-follow
- User can re-center/re-fit
- Path polyline and live marker are separate render states
- Show:
  - polyline for historical path
  - live marker for active trackers
- Do not show per-segment start/end markers in MVP

## Member Statuses

- `Owner`
- `Tracking`
- `Stale`
- `Spectating`
- `Offline`
- `Left`

Member sort order on active route:

1. Owner
2. Tracking
3. Stale
4. Spectating
5. Offline
6. Left

Within the same status group, sort by join time.

## Data and Timing

- Store exact timestamps in UTC
- Client displays localized times
- API snapshot returns full route history and current statuses
- Snapshot also returns current viewer capability booleans
- Return full snapshot for MVP; no chunked history yet

## Live Protocol

WebSocket authentication:

- Client connects to `GET /ws`
- Client sends first message:
  - `{ "type": "authenticate", "memberToken": "..." }`
- Server closes the live connection if authentication does not arrive before the configured timeout
- Default first-message authentication timeout: `5s`
- Server sends `connection_established` after successful authentication and route room subscription

Live stream includes:

- `member_joined`
- `member_left`
- `member_started_sharing`
- `member_stopped_sharing`
- `member_became_stale`
- `member_back_online`
- `position_updated`
- `route_updated`
- `route_closed`

Current backend broadcasts `member_joined`, `member_left`, `route_updated`, and `route_closed` over authenticated WebSocket route rooms.
Current backend also broadcasts `member_started_sharing` and `member_stopped_sharing` after successful sharing state updates.
Current backend accepts authenticated WebSocket `position_update` messages and broadcasts accepted points as `position_updated`.
Current frontend connects to the authenticated WebSocket for active routes, sends `position_update` messages while the viewer is tracking, applies `position_updated` events to the displayed map state, and applies sharing status events without refreshing the route snapshot.

## Persistence Rules

- Store accepted raw GPS readings as source of truth
- Preserve browser payload for accepted points
- Store:
  - server canonical timestamp
  - client timestamp if available
- Canonical ordering uses server receive time
- Brief reconnects within grace window keep the same path segment
- Prolonged disconnects end the segment

## GPS Validation

- Reject invalid coordinates
- Reject too-inaccurate first/live points based on configurable threshold
- Reject duplicate timestamp duplicates
- Reject impossible jumps using a generous speed threshold
- Do not store rejected points in MVP

## Dev and Deployment

- One monorepo
- Everything containerized
- Local development runs via `docker compose up`
- Services:
  - web
  - api
  - postgres/postgis
- Production MVP targets a single VPS with containers
- HTTPS required in production
