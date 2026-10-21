---
type: record
status: accepted
---

# ADR 0001: Account authentication and route identity

Date: 2026-10-20. Accepted for implementation on 2026-10-21. Password/session and verification/recovery foundations are implemented behind a disabled feature flag; other parts remain scheduled in the task checklist.

## Context

KeepUp currently stores browser identity and route credentials in [identity-storage.ts](../../apps/web/lib/identity-storage.ts); native identity/access is described in [mobile boundaries](../system/mobile.md). Route memberships and credentials are device-oriented in the [data model](../system/data.md). The [live hub](../../apps/api/internal/live/hub.go) and [live protocol](../system/api-and-live.md) currently support member-connection restrictions that must change to permit several account viewers and one sharing device per route.

The [authentication PRD](../product/authentication-prd.md) owns account rules, activation, identity immutability, recovery, device behavior, and acceptance criteria. This record owns the technical approach. The [v1 contract](../system/authentication-contract.md) fixes transports, endpoints, defaults, and examples for implementation.

## Decision

### Account authority and data

Use the existing Go API and PostgreSQL as account/session authority. Add a dedicated authentication service and repository boundary; keep route authorization and tracking rules in route services. Google supplies identity proofs, not application permissions.

Use an opaque UUID user primary key with original-display identifiers plus normalized email/username lookup keys and database unique constraints. Apply the PRD's normalization consistently at registration, login, reset, and linking. Email is an attribute, not a foreign-key identity. No identifier-edit endpoints are planned.

Proposed records: users with password hash and verification state; external identities keyed uniquely by provider and subject; revocable sessions; purpose-bound verification/reset token hashes with expiry and consumption; account-linked route membership episodes; per-account/per-route sharing leases tied to a specific session and connection generation. Reference user IDs from membership/ownership, not mutable client-provided names. Enforce one current membership per account/route transactionally. Historical membership episodes may remain after leaving.

Store salted, adaptive password hashes using Argon2id, with deployment-tested cost parameters and a versioned hash format. Never store or log passwords, raw reset/session tokens, or Google credentials. This follows [OWASP password storage guidance](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html).

### Sessions and transport

Propose opaque random session credentials, stored hashed server-side, so revocation is authoritative. Web uses Secure, HttpOnly, SameSite cookies in production with CSRF protection for mutations and strict origin checks. Android uses platform-backed secure credential storage and authenticated API requests. Do not move account credentials into browser localStorage. Define rotation, expiry, and replay behavior in AUTH-01's contract; activity extends idle expiry, not revoked sessions. See [OWASP session guidance](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html).

Replace standalone member/owner token authority with account session plus server-side route membership/ownership checks. Legacy tokens cannot remain an authorization bypass after rollout. For WebSocket bootstrap, propose a short-lived, single-use route-scoped ticket issued by an authenticated endpoint, sent in the first frame rather than the URL. Bind it to session and route. Check revocation throughout the connection lifetime, close revoked sockets, and release affected sharing in the same service operation. Exact ticket lifetime is configurable and fixed during backend contract work.

### Google identity and linking

Validate Google credentials server-side with a maintained library: signature, issuer, audience allowlist for web/Android, expiry, and flow replay/nonce protections. OAuth redirect flows require state validation and PKCE where applicable. Key external identity by provider subject, not email. Never trust a client-supplied email or decoded-but-unverified token.

Automatic initial email linking requires current ownership evidence. Google documents authoritative email for Gmail and verified hosted-domain accounts. A third-party Google account email may require another ownership challenge even when `email_verified` is true. Require a fresh KeepUp email challenge in that case, then link without demanding the existing password. Source: [Google backend authentication](https://developers.google.com/identity/sign-in/web/backend-auth). This exception is called out in the PRD for review.

Create user, password credential, and Google identity atomically after signup completion. Serialize concurrent password/Google registration and linking with database constraints and retry-safe transactions. Never overwrite an existing user's password from a Google onboarding form. In particular, if a pending password registration is claimed via authoritative Google, require a fresh password setup and revoke old pending sessions so an earlier registrant cannot retain access. An already active password account keeps its password and username.

### Sharing and presence

Separate account membership, viewer connections, and the sharing lease. A lease is unique for `(route_id, user_id)` and identifies its authorized session/connection generation. Acquire it atomically after policy, capacity, and session checks. Validate lease ownership on every position submission and sharing command. User ID alone cannot authorize a sender.

Permit multiple viewers without duplicating members. Viewer connectivity and sharing connectivity need separate presence accounting: another viewer cannot keep a dead sender active. Retain the existing configurable presence grace periods, but apply sharing timeout to the sender. Stop, offline timeout, session revocation, leave, close, or delete releases the appropriate lease and closes the path segment. Late packets and old timers are fenced by lease generation. A reconnect must revalidate or reacquire authorization and cannot take over another device's lease.

A transaction must serialize lease acquisition with member state, route capacity, and lifecycle changes. The current single-process live hub can remain initially; persistent lease state and startup recovery must prevent stale ownership after API restarts. Distributed socket fanout remains outside this decision, but PostgreSQL must enforce exclusivity rather than relying only on process-local checks.

### Verification, recovery, and email

Generate high-entropy, purpose-bound, expiring, single-use tokens; store hashes and consume atomically. Replacement invalidates old links. Do not consume verification/reset tokens merely from a GET request by an email scanner; use a confirmation action. Redact tokens from logs and avoid referrer leakage. Verification changes account state without issuing a session to the device opening the link. Signup holds only a restricted pending session until activation.

A mail interface accepts a rendered verification/reset message with recipient and stable ID. SMTP and fake test implementations share it; third-party providers plug into the same boundary. Validate selection/configuration at startup, require appropriate TLS, and keep secrets outside source control. Transactional outbox delivery uses retry/backoff and expiry checks. Queue links are encrypted with AES-256-GCM under a separate deployment key and bound to their token row IDs; purpose records store hashes. Terminal states erase ciphertext. Bounded delivery holds the account lock to serialize with token replacement/consumption. SMTP remains at least once after an acceptance/commit failure, so duplicate email is possible while tokens remain single-use. Do not retry superseded verification/reset messages. Delivery failure leaves the account pending and permits a controlled resend.

Authentication limits, verification/reset TTLs, resend cooldowns, and SMTP settings belong to deployment configuration. The [v1 contract](../system/authentication-contract.md) fixes engineering defaults, session rotation and retry behavior; [email configuration](../workflow/authentication-email.md) defines provider settings. B2/B3 test these boundaries. Real provider authentication and delivery remain R1 checks.

### Navigation and release

Store intended destinations separately from credentials, validate against local routes, and recheck authorization after activation/login. Never redirect to arbitrary user-supplied external URLs. Web and Android use the same account contract with platform-specific session transport and navigation handling.

Use the PRD's fresh-start release task. No historical account conversion is needed; database deletion and rollout are explicit deployment operations. Independent client tasks share contract fixtures and require coordinated production cutover.

## Alternatives considered

- Email primary key: rejected in favor of stable internal identity and separate uniqueness. Identifiers remain immutable by product rule.
- Managed authentication service: viable, but not selected; the agreed existing Go/PostgreSQL ownership supports configurable delivery and direct route/session revocation.
- Self-contained long-lived JWTs as sole authority: not selected because immediate logout/reset enforcement and socket revocation need server-side state anyway.
- Existing route tokens as independent authority: rejected because they would bypass account activation and session revocation.
- One live connection per account/route: rejected because the product requires additional devices to watch.
- Device takeover: rejected for this release. Only stop or confirmed offline timeout frees the sharing lease.

## Consequences and affected boundaries

The application owns password recovery, delivery reliability, abuse controls, and session security. Google adds configuration for both clients and a linking race to test. Database uniqueness and transactional sharing are mandatory, not just client UI checks.

Affected implementation routes: [route service](../../apps/api/internal/routes/service.go), [repository](../../apps/api/internal/routes/postgres.go), [HTTP/live handlers](../../apps/api/internal/httpapi/server.go), [migrations](../../db/migrations/), [web boundaries](../system/frontend.md), [native boundaries](../system/mobile.md), and proxy/cookie configuration. Existing map rendering and GPS quality rules remain applicable, while participant identity and authorization change.

## Validation and acceptance

Use the PRD acceptance criteria and delivery tasks. Test concurrent linking/registration, normalized uniqueness, token replay, CSRF/origin enforcement, live revocation, sender/viewer disconnect independence, simultaneous lease acquisition, and late position rejection. B2/B3 validate password accounts, normalized uniqueness, expiry, rotation, revocation, browser/native transports, purpose-bound token consumption/replacement, signup/queue atomicity, recovery quotas, encryption, worker coordination, retries and local TLS SMTP behavior. Google-linking, account-bound routes and live leases remain unimplemented; acceptance of this architecture does not mark them delivered. Real provider delivery remains R1.
