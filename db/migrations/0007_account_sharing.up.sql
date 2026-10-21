ALTER TABLE route_members ADD COLUMN live_absent_since timestamptz;
ALTER TABLE auth_sessions ADD COLUMN revoked_reason text CHECK (revoked_reason IN ('logout','logout_all','password_reset','password_changed'));

CREATE TABLE account_live_connections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES auth_sessions(id) ON DELETE CASCADE,
    route_id uuid NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES route_members(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX account_live_connections_member ON account_live_connections(member_id);
CREATE INDEX account_live_connections_session ON account_live_connections(session_id);

CREATE TABLE account_sharing_leases (
    route_id uuid NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    member_id uuid NOT NULL UNIQUE REFERENCES route_members(id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES auth_sessions(id) ON DELETE CASCADE,
    connection_id uuid NOT NULL,
    generation uuid NOT NULL DEFAULT gen_random_uuid(),
    last_position_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    stale_at timestamptz,
    PRIMARY KEY (route_id, user_id)
);
CREATE INDEX account_sharing_leases_session ON account_sharing_leases(session_id);
