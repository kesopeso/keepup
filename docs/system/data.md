---
type: reference
status: active
---

# Persistence and history

Account identity decisions are recorded in [ADR 0001](../adr/0001-account-authentication.md). Migrations 0003–0005 add staged account, recovery and Google tables below; route membership remains device-oriented until B5/B6. Adding a migration does not apply it to an existing database.

## Account foundation

[Migration 0003](../../db/migrations/0003_accounts_and_sessions.up.sql) adds `users`, `auth_sessions`, and `auth_attempts`. Users have a UUID primary key, preserved email/username strings, normalized unique lookup keys, Argon2id password hashes, optional verification timestamp, and creation timestamp. Checks tie lookup keys to lowercase identifiers; a trigger rejects identifier changes.

Sessions reference user IDs and store SHA-256 credential digests, creation/use/idle-expiry timestamps, and revocation. Atomic rotation replaces the digest while retaining session identity. Signup creates user/session/token/outbox together; login, rotation, revocation and password maintenance share a user-row lock to serialize credential replacement and revocation.

Attempt counters contain hashed scope keys, window start and attempt count. Cleanup retains expired session/token/counter records for the configured period, which must cover all rate windows, then deletes them without deleting users.

[Migration 0004](../../db/migrations/0004_auth_recovery_and_outbox.up.sql) adds purpose-bound `auth_tokens` with hash, expiry, consumption and invalidation timestamps. A partial unique index permits one unconsumed/uninvalidated token per account/purpose. Issuing a replacement invalidates the old token; consumption locks/rechecks the account and token atomically. Reset revokes all sessions without verifying the user. Change rotates the current session, revokes others and invalidates reset links.

`auth_mail_outbox` references each token once, with AES-GCM encrypted payload, delivery state, failure count and retry time. The key is deployment configuration outside PostgreSQL. Terminal rows clear payloads. Worker claims lock the user or Google flow before the outbox row, coordinating with token replacement/consumption. Provider delivery is at least once; single-use tokens remain authoritative even if a message is duplicated. Account membership links and sharing leases remain unimplemented. The [account contract](authentication-contract.md) separates current endpoints from future tasks.

[Migration 0005](../../db/migrations/0005_google_authentication.up.sql) adds `external_identities`, unique by provider/subject and provider/user, plus `auth_google_flows` with hashed credential/nonce, transport, audience snapshot, verified provider claims, state and absolute expiry. ID tokens and onboarding passwords are not stored. Google email challenge tokens use `auth_tokens.google_flow_id` instead of a user; a check enforces the owner/purpose combination and a partial unique index enforces one current challenge per flow. Flow deletion cascades through challenge/outbox rows. Recipient mail quotas include all purposes under a recipient advisory lock, even before account creation.

Google mutations lock flow, subject key, email key, user and then token/outbox records. Password signup shares the email key lock so concurrent account creation cannot bypass linking checks. Active account credentials stay fixed; claiming pending registrations replaces the password, revokes old sessions and invalidates recovery/verification links. Existing pending usernames remain immutable. Google flow/challenge consumption is single-use, and its deadline is checked again after lock waits before session issuance.

## Data Model

### routes

- `id`
- `code`
- `name`
- `description`
- `password_hash`
- `sharing_policy`
- `status`
- `max_tracking_members`
- `created_at`
- `closed_at`

### route_members

- `id`
- `route_id`
- `client_id`
- `display_name`
- `transport_mode`
- `is_owner`
- `status`
- `joined_at`
- `left_at`
- `color`

### path_segments

- `id`
- `route_id`
- `member_id`
- `started_at`
- `ended_at`
- `end_reason`

### position_points

- `id`
- `route_id`
- `member_id`
- `segment_id`
- `seq`
- `recorded_at`
- `client_recorded_at`
- `location` (`geography(Point, 4326)`)
- `latitude`
- `longitude`
- `accuracy_m`
- `altitude_m`
- `speed_mps`
- `heading_deg`
- `raw_payload`

### member_tokens

- `id`
- `member_id`
- `token_hash`
- `created_at`
- `revoked_at`

### owner_tokens

- `id`
- `route_id`
- `member_id`
- `token_hash`
- `created_at`
- `revoked_at`


## Storage and Derived Data

- Raw accepted points are the source of truth
- Derived data can be added later:
  - simplified paths
  - snapped paths
  - replay timelines

Never overwrite raw points with derived geometry.


## Persistence Rules

- Store accepted raw GPS readings as source of truth
- Preserve browser payload for accepted points
- Store:
  - server canonical timestamp
  - client timestamp if available
- Canonical ordering uses server receive time
- Brief reconnects within grace window keep the same path segment
- Prolonged disconnects end the segment


Sources: [core migration](../../db/migrations/0001_enable_postgis_and_core_tables.up.sql), [presence migration](../../db/migrations/0002_membership_presence_rules.up.sql), [repository](../../apps/api/internal/routes/postgres.go). The migrations own exact columns, constraints, and indexes; the list above is a conceptual overview.
