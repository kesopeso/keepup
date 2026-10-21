CREATE TABLE external_identities (
    provider TEXT NOT NULL CHECK (provider = 'google'),
    subject TEXT NOT NULL CHECK (length(subject) BETWEEN 1 AND 255),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider,subject),
    UNIQUE (provider,user_id)
);

CREATE TABLE auth_google_flows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash TEXT NOT NULL UNIQUE,
    nonce_hash TEXT NOT NULL UNIQUE,
    transport TEXT NOT NULL CHECK (transport IN ('web','native')),
    audiences TEXT[] NOT NULL CHECK (cardinality(audiences)>0),
    status TEXT NOT NULL DEFAULT 'awaiting_proof'
        CHECK (status IN ('awaiting_proof','email_verification_required','ready','onboarding','consumed')),
    subject TEXT,
    email TEXT,
    email_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    CHECK ((status='awaiting_proof') = (subject IS NULL)),
    CHECK ((subject IS NULL) = (email IS NULL)),
    CHECK ((email IS NULL) = (email_key IS NULL)),
    CHECK (email_key = lower(email COLLATE "C"))
);
CREATE INDEX auth_google_flows_expiry_idx ON auth_google_flows(expires_at);

ALTER TABLE auth_tokens ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE auth_tokens ADD COLUMN google_flow_id UUID REFERENCES auth_google_flows(id) ON DELETE CASCADE;
ALTER TABLE auth_tokens DROP CONSTRAINT auth_tokens_purpose_check;
ALTER TABLE auth_tokens ADD CONSTRAINT auth_tokens_purpose_check
    CHECK (purpose IN ('verification','reset','google_email'));
ALTER TABLE auth_tokens ADD CONSTRAINT auth_tokens_owner_check CHECK (
    (purpose='google_email' AND google_flow_id IS NOT NULL AND user_id IS NULL) OR
    (purpose IN ('verification','reset') AND user_id IS NOT NULL AND google_flow_id IS NULL)
);
CREATE UNIQUE INDEX auth_tokens_google_current_idx ON auth_tokens(google_flow_id)
    WHERE consumed_at IS NULL AND invalidated_at IS NULL;
