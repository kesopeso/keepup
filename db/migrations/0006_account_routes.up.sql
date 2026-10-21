-- Legacy rows remain separate until the explicitly authorized fresh-start rollout.
ALTER TABLE routes ADD COLUMN owner_user_id UUID REFERENCES users(id);
CREATE INDEX routes_owner_user_idx ON routes(owner_user_id) WHERE owner_user_id IS NOT NULL;
ALTER TABLE route_members ADD COLUMN user_id UUID REFERENCES users(id);
ALTER TABLE route_members ADD CONSTRAINT route_members_account_left_check
    CHECK (user_id IS NULL OR ((left_at IS NOT NULL) = (status='left')));
CREATE UNIQUE INDEX route_members_current_account_idx ON route_members(route_id,user_id)
    WHERE user_id IS NOT NULL AND left_at IS NULL;
CREATE INDEX route_members_account_history_idx ON route_members(user_id,route_id,joined_at DESC)
    WHERE user_id IS NOT NULL;

CREATE TABLE auth_socket_tickets (
    token_hash TEXT PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES auth_sessions(id) ON DELETE CASCADE,
    route_id UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    member_id UUID NOT NULL REFERENCES route_members(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);
CREATE INDEX auth_socket_tickets_expiry_idx ON auth_socket_tickets(expires_at);
