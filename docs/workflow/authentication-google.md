---
type: reference
status: active
---

# Google authentication configuration

B4 implements Google ID-token verification, account linking and username/password onboarding in the API. W1 implements the web button, SDK integration, onboarding and email-link pages; native integration remains A1 and real Google acceptance remains R1. The [account contract](../system/authentication-contract.md#google-proof-and-onboarding) owns the request/response flow.

## Enablement

Apply migration [0005](../../db/migrations/0005_google_authentication.up.sql) after 0003/0004 using the manual migration helper. The API does not migrate at startup. Authentication still requires `AUTH_ENABLED=true` and the [account/email settings](authentication-email.md). Google remains unavailable for a transport until its audience list is configured. Password authentication can run without Google configuration.

| Environment variable | Default | Meaning |
|---|---|---|
| `AUTH_GOOGLE_WEB_CLIENT_IDS` | empty | Comma-separated approved browser ID-token audiences, maximum 20 |
| `AUTH_GOOGLE_NATIVE_CLIENT_IDS` | empty | Comma-separated approved native ID-token audiences, maximum 20 |
| `AUTH_GOOGLE_FLOW_TTL` | `10m` | Positive duration; absolute flow deadline, never extended by polling or resend |
| `AUTH_GOOGLE_REQUEST_TIMEOUT` | `10s` | Positive duration; proof-verification and key-fetch timeout |
| `AUTH_GOOGLE_WEB_CLIENT_ID` | empty | Web server runtime setting, one public client ID from the API's web allowlist; `/auth/config` supplies it to the browser without rebuilding the image |

Set the singular client ID on the Next.js server and include it in the API's plural `AUTH_GOOGLE_WEB_CLIENT_IDS` allowlist. In development, Next.js can read it from `apps/web/.env.development.local`; restart the web service after changing it. Production supplies it as a container environment variable. Configure the browser origin in Google's OAuth client settings. This is a public identifier, not a client secret. An absent value keeps the password forms usable and shows an unavailable message when Google is chosen. Web implementation details belong to [frontend boundaries](../system/frontend.md#account-authentication).

Use actual OAuth client IDs ending in `.apps.googleusercontent.com`, not a project number, client secret or Android package name. Native SDKs may request an ID token whose audience is a backend/web client ID; configure the IDs actually issued for that integration. An `azp` claim, when present, must also name an approved client from these lists. Include the approved native presenter ID as appropriate. Each flow snapshots its transport's audience list at creation. Removing configuration disables new calls for that transport; outstanding flows otherwise retain their original audience binding until expiry.

Startup constructs `go-oidc` verification against Google's fixed HTTPS JWKS endpoint. Public keys are cached and refreshed by the library. There is no arbitrary issuer/JWKS endpoint environment override, token introspection endpoint, or client secret for this ID-token flow. Automated tests use locally signed proofs and a local mock key endpoint, without calling Google.

## Client responsibilities

Start a flow and pass its returned nonce to the Google SDK. Submit the resulting ID token through the protected JSON proof endpoint. Web receives a separate HttpOnly `__Host-keepup_google` cookie in production, or `keepup_google` in development; JavaScript receives only the nonce and expiry. Native receives `flowToken`, holds it in secure transient storage and uses it as the bearer credential for subsequent Google flow calls. This credential is distinct from `sessionToken` and cannot access account or route authority.

The flow supports ID-token proofs, not an OAuth authorization-code callback. If a later client adds a redirect/code flow, its state and PKCE validation must be implemented before use. Clients discard the transient flow on cancellation and start again when needed; abandoned flows expire and cleanup removes them after retention. On a successful session exchange, the browser flow cookie is cleared and native clients must discard their flow credential. Keep onboarding passwords out of persistent storage.

Third-party Google emails receive a fresh KeepUp challenge even if Google marks the email verified. The link uses `/auth/google/verify#token=...`. W1 renders explicit confirmation, removes the fragment after capture and avoids analytics on that page. A1 must provide the same protections. POST confirmation changes only the original flow to `ready`, with no session for the device opening the link. The initiating client polls context and explicitly exchanges it to continue. A Gmail address or verified hosted-domain account can establish initial ownership without this challenge. Once linked, the provider subject identifies the account even if its Google email changes. Stored KeepUp email and username remain immutable.

## Persistence and limits

Flows store hashes of their credential and nonce, allowed audiences, transport, status, verified provider subject/email and expiry. They never store ID tokens or onboarding passwords. External identities are unique by provider/subject and provider/user. Transactions serialize flow consumption, account creation and linking; password signup shares the email lock. An existing active account keeps its password and identifiers. Claiming a pending password registration requires a fresh password, retains its existing username, revokes old pending sessions and invalidates old verification/reset links. Onboarding context exposes that fixed username so clients can show it as read-only.

Google start/proof/complete/exchange share the configured login-attempt limit per source address, under a separate Google counter scope. Email confirmation uses the existing confirmation limits. All verification, reset and Google challenge messages share the recipient mail quota, including flows without an account. Google resend cooldown also spans separate flows for the same email, preventing new contexts from bypassing the cooldown. Challenge expiry is bounded by both the flow deadline and verification TTL. Resend replaces only the initiating flow's challenge. The existing encrypted outbox, TLS provider and retry policy deliver these messages; the worker locks the flow instead of a user for Google challenges. Cleanup removes expired flows and their challenge/outbox records after retention.

Sources: [configuration](../../apps/api/internal/config/google.go), [verifier](../../apps/api/internal/auth/google_verifier.go), [flow transactions](../../apps/api/internal/auth/google_postgres.go), [email challenges](../../apps/api/internal/auth/google_email.go), [HTTP transport](../../apps/api/internal/httpapi/google.go). Google's [backend verification guidance](https://developers.google.com/identity/gsi/web/guides/verify-google-id-token) defines issuer, audience, subject and authoritative-email checks; the [go-oidc API](https://pkg.go.dev/github.com/coreos/go-oidc/v3/oidc) owns cryptographic verification and key caching.
