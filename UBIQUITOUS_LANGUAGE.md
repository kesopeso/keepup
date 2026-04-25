# Ubiquitous Language

## Route Lifecycle

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Route** | One shareable live tracking session. | Trip, session, channel |
| **Route code** | The short human-enterable identifier used to access a route. | Invite code, room code |
| **Archive** | A closed route that remains readable but is no longer live. | Closed route history, replay |
| **Snapshot** | The authenticated full route bootstrap payload used to render the route screen. | Initial state, route payload |
| **Access metadata** | The public pre-join route data shown before membership exists. | Public route, preview payload |

## Membership

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Route member** | A browser/device-specific participant record within a route. | User, participant, connection |
| **Membership** | The relationship created when a browser joins a route. | Join, participant account |
| **Route owner** | The route member with management authority over the route. | Admin, creator, host |
| **Spectator** | A route member who is present but not sharing location. | Viewer, watcher |
| **Tracker** | A route member who is actively sharing location. | Sharer, active user |
| **Member token** | The route-scoped credential that authenticates a route member. | Access token, user token |
| **Owner token** | The route-scoped credential that authorizes owner actions. | Admin token, creator token |

## Live Realtime

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Live connection** | One authenticated WebSocket connection for a route member. | Socket, client connection, session |
| **First-message authentication** | The required first WebSocket message that authenticates a live connection with a member token. | Query-token auth, socket login |
| **WebSocket authentication timeout** | The maximum time a live connection may remain unauthenticated after opening. | Auth deadline, socket timeout |
| **Live hub** | The server-side coordinator that tracks live connections by route room. | Hub, broadcaster, socket manager |
| **Route room** | The live fan-out group for all live connections subscribed to one route. | Room, channel, topic |
| **Route room subscription** | The membership of one live connection in one route room. | Socket membership, room join |
| **Live event** | A server-published realtime fact sent to route room subscribers. | Message, notification, delta |
| **Connection established event** | The live event confirming that first-message authentication and route room subscription succeeded. | Auth success, connected message |
| **Position update** | A live event carrying one accepted location point for a tracker. | Location update, GPS ping |

## Tracking

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Sharing policy** | The rule that determines who may start sharing location on a route. | Permission mode, visibility policy |
| **Tracking slot** | One available active tracker capacity within a route. | Seat, active slot |
| **Path segment** | A contiguous sequence of accepted points for one tracker. | Track, line, trace |
| **Route point** | One accepted raw GPS reading stored for a route member. | GPS point, location sample |

## Relationships

- A **Route** has zero or more **Route members**.
- A **Route owner** is exactly one **Route member** with owner authority.
- A **Route member** may have zero or one active **Live connection** per browser tab, but the model should tolerate multiple live connections.
- A **Live connection** must complete **First-message authentication** before it can create a **Route room subscription**.
- A **Route room** belongs to exactly one **Route**.
- A **Route room** may contain zero or more **Route room subscriptions**.
- A **Live event** is published to one **Route room**.
- A **Tracker** produces **Position updates** that become **Route points**.
- A **Path segment** contains one or more **Route points**.

## Example Dialogue

> **Dev:** "When a browser opens `GET /ws`, is it already a **Route member**?"

> **Domain expert:** "No. It is only a **Live connection** until **First-message authentication** proves the **Member token**."

> **Dev:** "After authentication, do we put the connection in a channel?"

> **Domain expert:** "Call that channel a **Route room**. The **Live hub** creates a **Route room subscription** for the authenticated **Route member**."

> **Dev:** "So a `position_update` is a **Live event** published to the **Route room**?"

> **Domain expert:** "Exactly. The **Tracker** sends a location, the server accepts it as a **Route point**, and then publishes a **Position update** to the route's subscribers."

## Flagged Ambiguities

- "User" is too broad for MVP; use **Route member** for a joined participant and **Route owner** for management authority.
- "Socket" describes a transport detail; use **Live connection** when discussing product behavior.
- "Hub" is vague by itself; use **Live hub** for the server-side realtime coordinator.
- "Room", "channel", and "topic" should collapse to **Route room** for per-route realtime fan-out.
- "Message", "delta", and "notification" should collapse to **Live event** unless the distinction matters technically.
- **Route room subscription** is not the same as **Membership**; membership is persistent route participation, while route room subscription is an active live connection registration.
