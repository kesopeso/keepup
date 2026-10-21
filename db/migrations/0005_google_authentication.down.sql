-- Discard only the transient Google challenges before restoring B3 constraints.
DELETE FROM auth_tokens WHERE purpose='google_email';
DROP INDEX auth_tokens_google_current_idx;
ALTER TABLE auth_tokens DROP CONSTRAINT auth_tokens_owner_check;
ALTER TABLE auth_tokens DROP CONSTRAINT auth_tokens_purpose_check;
ALTER TABLE auth_tokens ADD CONSTRAINT auth_tokens_purpose_check CHECK (purpose IN ('verification','reset'));
ALTER TABLE auth_tokens DROP COLUMN google_flow_id;
ALTER TABLE auth_tokens ALTER COLUMN user_id SET NOT NULL;
DROP TABLE auth_google_flows;
DROP TABLE external_identities;
