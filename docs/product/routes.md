---
type: reference
status: specified
---

# Routes and membership

## Product Summary

KeepUp is a mobile-first web app for live route sharing between friends, groups, or coordinators monitoring participants across different vehicles. Users join routes via shareable links/codes and can watch live progress or share their own location when allowed.

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

API contracts: [REST and live protocol](../system/api-and-live.md).

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
- `Left` is terminal for that membership, revokes the member token, and does not block alias reuse
- Active-route owners cannot leave; they can stop sharing, close the route, or delete the route

## Owner Rules

- Owner is a role, not an automatically tracked participant
- Owner may spectate or track
- Owner cannot leave an active route in the MVP; they can close or delete it
- Owner authority persists via owner token
- Owner can:
  - edit route name/description
  - close route
  - delete route
- Closing requires confirmation explaining the permanent archive transition
- Deleting requires typing the exact route code
- Successful deletion clears route-scoped browser credentials and returns the deleting owner to the create screen
- Closed routes cannot be reopened

## Sharing Policies

- `everyone_can_share`
  - any joined member may start sharing if tracking slots are available
- `joiners_can_view_only`
  - non-owner members are spectators only
  - owner may still choose to track or spectate

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
- Limit counts tracking and stale members; spectators, offline members, and left members do not occupy slots
- Owner occupies a slot while tracking or stale
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

See [system design](../architecture.md) for implementation and [delivery status](../planning/status.md) for progress.
