---
type: reference
status: active
---

# Persistence and history

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
