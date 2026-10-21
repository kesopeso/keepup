CREATE TABLE auth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose TEXT NOT NULL CHECK (purpose IN ('verification', 'reset')),
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    invalidated_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX auth_tokens_current_idx ON auth_tokens(user_id,purpose)
    WHERE consumed_at IS NULL AND invalidated_at IS NULL;
CREATE INDEX auth_tokens_expiry_idx ON auth_tokens(expires_at);
CREATE INDEX auth_tokens_mail_limit_idx ON auth_tokens(user_id,created_at);

CREATE TABLE auth_mail_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_id UUID NOT NULL UNIQUE REFERENCES auth_tokens(id) ON DELETE CASCADE,
    payload BYTEA,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','failed','cancelled')),
    failures INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((status = 'pending') = (payload IS NOT NULL))
);
CREATE INDEX auth_mail_outbox_due_idx ON auth_mail_outbox(next_attempt_at) WHERE status='pending';
