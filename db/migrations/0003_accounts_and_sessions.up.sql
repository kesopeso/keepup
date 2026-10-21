CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    email_key TEXT NOT NULL UNIQUE CHECK (email_key = lower(email COLLATE "C")),
    username TEXT NOT NULL CHECK (username ~ '^[A-Za-z0-9_-]{3,24}$'),
    username_key TEXT NOT NULL UNIQUE CHECK (username_key = lower(username COLLATE "C")),
    password_hash TEXT NOT NULL,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE FUNCTION keepup_immutable_identifiers() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.email IS DISTINCT FROM OLD.email OR NEW.username IS DISTINCT FROM OLD.username THEN
        RAISE EXCEPTION 'account identifiers are immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER users_immutable_identifiers BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION keepup_immutable_identifiers();

CREATE TABLE auth_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);
CREATE INDEX auth_sessions_user_idx ON auth_sessions(user_id) WHERE revoked_at IS NULL;
CREATE INDEX auth_sessions_expiry_idx ON auth_sessions(expires_at);

-- Shared fixed-window counters. Keys are hashes, not email/IP plaintext.
CREATE TABLE auth_attempts (
    key_hash TEXT PRIMARY KEY,
    window_started_at TIMESTAMPTZ NOT NULL,
    attempts INTEGER NOT NULL CHECK (attempts > 0)
);
