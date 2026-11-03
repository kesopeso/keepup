---
type: reference
status: specified
---

# Routes and membership

Route access requires a verified account on both web and Android. The [authentication PRD](authentication-prd.md) owns account identity and the [account contract](../system/authentication-contract.md#account-route-api) owns REST/live authority. Display names and ownership derive from the account; no per-route credentials are issued or restored.

## Product Summary

KeepUp is a mobile-first live route-sharing application with web and Android clients for friends, groups, or coordinators monitoring participants across different vehicles. Users join routes via shareable links/codes and can watch live progress or share their own location when allowed.

## Route Access

- Share links and case-insensitive uppercase codes open a route after sign-in and verification.
- My routes lists owned, current and former memberships across devices.
- Route names are required and need not be unique; descriptions are optional.
- Routes may be open by code or protected by a password.
- Current account memberships restore without repeating the password or transport selection.
- New memberships and explicit rejoin require transport mode and the route password when protected.
- Closed archives permit explicit rejoin and remain read-only.
- Creating a route or joining uses the immutable account username, never a device alias.
- Protected content checks server authority before displaying route history; cached summaries and old route tokens grant no access.

API contracts: [account REST and live protocol](../system/authentication-contract.md).

## Membership and Identity

- Membership and ownership belong to the account and restore across clients.
- Every viewer has a current membership, including spectators.
- Members may leave; history remains associated with that membership episode.
- Rejoining creates a new episode while preserving prior paths.
- Active-route owners cannot leave; they may stop sharing, close or delete the route.
- Multiple viewers may watch concurrently; one connection may share per account/route.

## Owner Rules

- Owner is a role, not an automatically tracked participant
- Owner may spectate or track
- Owner cannot leave an active route in the MVP; they can close or delete it
- Owner authority persists through the account
- Owner can:
  - edit route name/description
  - close route
  - delete route
- Closing requires confirmation explaining the permanent archive transition
- Deleting requires typing the exact route code
- Successful deletion returns the owner to My routes and removes the route from account lists
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
