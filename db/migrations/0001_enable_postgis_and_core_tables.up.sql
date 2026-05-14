CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT,
    password_hash TEXT,
    sharing_policy TEXT NOT NULL CHECK (sharing_policy IN ('everyone_can_share', 'joiners_can_view_only')),
    status TEXT NOT NULL CHECK (status IN ('active', 'closed')),
    max_tracking_members INTEGER NOT NULL CHECK (max_tracking_members > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ
);

CREATE TABLE route_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    client_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    transport_mode TEXT NOT NULL CHECK (
        transport_mode IN ('walking', 'bicycle', 'car', 'bus', 'train', 'boat', 'airplane')
    ),
    is_owner BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL CHECK (status IN ('tracking', 'spectating', 'stale', 'offline', 'left')),
    color TEXT NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX route_members_route_alias_unique_idx
    ON route_members (route_id, LOWER(display_name))
    WHERE status <> 'left';

CREATE TABLE path_segments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    member_id UUID NOT NULL REFERENCES route_members(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    end_reason TEXT CHECK (end_reason IN ('stopped', 'disconnected', 'left', 'route_closed'))
);

CREATE TABLE position_points (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    member_id UUID NOT NULL REFERENCES route_members(id) ON DELETE CASCADE,
    segment_id UUID NOT NULL REFERENCES path_segments(id) ON DELETE CASCADE,
    seq BIGINT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    client_recorded_at TIMESTAMPTZ,
    location geography(POINT, 4326) NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    accuracy_m DOUBLE PRECISION,
    altitude_m DOUBLE PRECISION,
    speed_mps DOUBLE PRECISION,
    heading_deg DOUBLE PRECISION,
    raw_payload JSONB NOT NULL DEFAULT '{}'::JSONB
);

CREATE UNIQUE INDEX position_points_segment_seq_unique_idx
    ON position_points (segment_id, seq);

CREATE INDEX position_points_route_recorded_at_idx
    ON position_points (route_id, recorded_at);

CREATE INDEX position_points_segment_recorded_at_idx
    ON position_points (segment_id, recorded_at);

CREATE INDEX position_points_location_gist_idx
    ON position_points USING GIST (location);

CREATE TABLE member_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id UUID NOT NULL REFERENCES route_members(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE TABLE owner_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    member_id UUID NOT NULL REFERENCES route_members(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);
