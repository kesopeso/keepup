---
type: reference
status: active
---

# API and live protocol

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

### WebSocket

- accept `GET /ws` and require the first client message to authenticate with a member token
- close unauthenticated live connections if the auth message does not arrive before `WEBSOCKET_AUTH_TIMEOUT`
- subscribe connection to route live events
- receive `position_update`
- publish live membership/status updates

Business logic must not live only in the WebSocket handlers. Tracking rules belong in application services.

Implementation uses an in-memory route-room hub with buffered subscription channels; horizontal scaling remains [deferred](../planning/backlog.md).

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


## Tracking Rules

Permissions and slot semantics are owned by [routes and membership](../product/routes.md); user-facing recovery rules by [tracking](../product/tracking.md). The route service enforces these rules before persistence.

Position ingestion requires an active route, a tracking or stale member, valid coordinates, and an open segment. `start_sharing` opens a segment; `stop_sharing` closes it with reason `stopped`. Accepted positions from stale members publish `member_back_online` before `position_updated`. Failed position submissions receive `position_rejected`; sharing commands receive `command_ack` or `command_rejected`.

### Presence and Status Timers

Timer transitions and defaults are listed under Live Protocol below. Timer configuration comes from the [API config](../../apps/api/internal/config/config.go).

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
- `member_went_offline`
- `position_updated`
- `route_updated`
- `route_closed`

Current backend broadcasts `member_joined`, `member_left`, `route_updated`, and `route_closed` over authenticated WebSocket route rooms.
Current backend also broadcasts `member_started_sharing`, `member_stopped_sharing`, `member_became_stale`, `member_back_online`, and `member_went_offline` after successful live status updates.
Current backend accepts authenticated WebSocket `position_update` messages and broadcasts accepted points as `position_updated`.
Current frontend connects to the authenticated WebSocket for active routes, sends `start_sharing`/`stop_sharing` commands, sends `position_update` messages while the viewer is tracking, applies `position_updated` events to the displayed map state, and applies sharing/status events without refreshing the route snapshot.
Current frontend shows a blocking stale recovery prompt when an active route initially loads with the viewer as `stale`, with explicit resume-sharing and continue-as-spectator actions. A viewer who becomes stale during an existing live session can still recover automatically when accepted positions resume.

Live connection rules:

- Active routes attempt one authenticated WebSocket per member.
- A second live connection for the same member is rejected with `live_connection_rejected` and reason `already_active_connection`; the existing connection remains active.
- Closed route archive screens do not open WebSockets.
- `offline -> spectating` happens after successful live authentication and broadcasts `member_back_online`.
- `tracking -> stale` happens immediately on live connection close, or after `ROUTES_TRACKING_STALE_AFTER` without accepted positions.
- `stale -> offline` happens after `ROUTES_TRACKING_OFFLINE_AFTER` spent stale and closes open segments with reason `disconnected`.
- `spectating -> offline` happens after `ROUTES_SPECTATOR_OFFLINE_AFTER` without reconnect.
- Default timing values are `20s`, `5m`, and `20s` respectively.


Sources: [HTTP and WebSocket handling](../../apps/api/internal/httpapi/server.go), [hub](../../apps/api/internal/live/hub.go), [DTOs](../../apps/api/internal/routes/models.go).

Product semantics: [tracking and presence](../product/tracking.md).
