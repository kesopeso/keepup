---
type: plan
status: active
---

# Authentication implementation tasks

## Handoff

Implementation is authorized with the accepted defaults. Product rules belong to the [PRD](../product/authentication-prd.md); architecture belongs to [ADR 0001](../adr/0001-account-authentication.md). Authentication and My routes visual reviews are complete. The PRD links portable design snapshots and records feedback. The [v1 contract and fixtures](../system/authentication-contract.md) support independent client work. Execute application tools in Docker and refresh Graphify after application edits as required by [documentation discipline](../workflow/documentation.md).

Each task below is an independent change with its own validation. Web and Android depend on the shared contract, not on one another. The parent delivery groups remain AUTH-01 through AUTH-04 in the PRD.

## Resume point

B1/B2/B3/B4 are complete as staged backend increments. Continue with B5 account route authorization/My routes, then B6 live authority. Web and Android can proceed independently against B1 fixtures and integrate completed endpoints. The account API is disabled by default. Implementation checks apply migrations 0003–0005 only in disposable test schemas; this session did not migrate/reset application tables or deploy. Current routes and clients still use legacy identity until their integration tasks.

B2 implements pending signup, normalized uniqueness and immutable identifiers, Argon2id, email/username login, hashed opaque sessions, idle extension, explicit atomic rotation, current/all-session revocation, configurable attempt limits, and protected browser/native transports. B3 adds purpose-bound token consumption/replacement, pending activation, reset/change revocation, encrypted transactional outbox, SMTP/fake providers, quotas, trusted-proxy support, and cleanup. Race-enabled PostgreSQL and local TLS SMTP tests cover these boundaries. The active-account gate is ready for B5; existing route handlers do not use it yet. Live cleanup remains B6, link/form UI remains W1/A1, and actual SMTP credentials/proxy CIDRs/real delivery remain R1.

## AUTH-01: Backend

### B1: Define contracts and fixtures

State: complete. Target endpoints, transports, policies, route summaries, live events and deterministic success/failure fixtures are in the shared contract. Endpoints outside B2/B3/B4 remain specified, not implemented.

- Depends on accepted PRD/ADR. Specify auth endpoints, cookie/native transports, Google proof exchange, pending state, redirects, validation, errors, session expiry/rotation, and single-use socket tickets.
- Specify account membership, owner authority, sender/viewer presence, sharing generations, route-list schema, filter semantics, pagination, and safe former-member summaries.
- Fix configurable defaults and boundaries: verification/reset TTLs, resend and attempt limits, ticket TTL, idle session policy, and mail retries. Record Google third-party email and pending-account claim behavior.
- Done when versioned request/response and live-event examples cover success, failure, expiry, revocation, and conflict; both client tasks can use deterministic fixtures. Document unresolved questions instead of inventing incompatible client rules.

### B2: Account persistence and password login

State: complete for the staged account API. Source: [account service](../../apps/api/internal/auth/service.go), [HTTP transport](../../apps/api/internal/httpapi/auth.go), and [migration 0003](../../db/migrations/0003_accounts_and_sessions.up.sql). Route enforcement and socket revocation are B5/B6 dependencies, not implemented by this increment.

- Depends on B1. Add user IDs, case-insensitive unique email/username lookup keys, immutable display identifiers, password hashes, account state, session hashes and indexes.
- Implement signup, email-or-username login, pending restrictions, current-account lookup, session expiry/rotation, logout, and logout everywhere. Include origin/CSRF enforcement for browser mutations and native credential transport.
- Done when database-backed tests prove case collisions, concurrent signup, password boundaries, idle expiry, revocation, and inability to mutate identifiers. Secrets must not reach logs.

### B3: Verification, recovery, and delivery

State: complete for backend account authority. Source: [recovery service](../../apps/api/internal/auth/recovery.go), [token transactions](../../apps/api/internal/auth/recovery_postgres.go), [queue worker](../../apps/api/internal/auth/outbox.go), [providers](../../apps/api/internal/maildelivery/mail.go), and [migration 0004](../../db/migrations/0004_auth_recovery_and_outbox.up.sql). [Configuration](../workflow/authentication-email.md) covers required secrets and settings. No real email was sent. Required live sharing/socket cleanup waits for account-bound routes in B5/B6; confirmation matching and email-link UI wait for W1/A1.

- Depends on B2. Implement purpose-bound token hashes, atomic consumption/replacement, verification, resend, forgot/reset password, and authenticated password change.
- Add configurable provider interface, SMTP, fake delivery, delivery queue/retry strategy, templates, secret configuration, and rate limits. Handle scanners without consuming tokens on GET.
- Done when expiry/replay/replacement, mismatch validation, generic recovery responses, SMTP failure/retry, pending activation restrictions, and required session revocation are tested. Use fake mail in automated tests.

### B4: Google accounts

State: complete for the backend ID-token flow. Source: [verifier](../../apps/api/internal/auth/google_verifier.go), [flow/account transactions](../../apps/api/internal/auth/google_postgres.go), [challenge queueing](../../apps/api/internal/auth/google_email.go), [HTTP endpoints](../../apps/api/internal/httpapi/google.go), and [migration 0005](../../db/migrations/0005_google_authentication.up.sql). [Google configuration](../workflow/authentication-google.md) covers audiences, nonce transport, optional enablement and client responsibilities. Tests use real signatures with a local key endpoint plus mocked identity-provider claims against isolated PostgreSQL schemas. Client SDK/buttons/link pages, OAuth code callbacks and real Google acceptance are not included; W1/A1/R1 remain pending.

- Depends on B2/B3. Validate Google proofs for approved web/native clients, map provider subjects, add username/password onboarding, and link existing accounts without replacing their password.
- Cover authoritative email, third-party email challenge, cancelled/expired onboarding, provider collision, and pending-account claim protection.
- Done when mocked identity-provider tests reject bad issuer/audience/signature/expiry/replay and concurrent signup/linking cannot duplicate or hijack accounts. Real Google smoke checks belong to R1.

### B5: Account route authorization and My routes API

- Depends on B2 and B1's route contract. Replace independent member/owner token authority; scope route REST and live authorization to active account sessions and membership/ownership.
- Persist membership episodes; restore current access across devices; use username for display; support leave/rejoin without erasing old history.
- Implement Active/Archived list, additive My/Joined/Left filters, counts, sorting, pagination, deduplication, and former-member-safe responses. Rejoining must recheck access/password and move categories.
- Done when API tests cover ownership isolation, pending accounts, stale legacy credentials, repeated join, former-member privacy, all filter combinations, pagination without duplicates, deleted routes, and archive access.

### B6: Multiple viewers and exclusive sharing

- Depends on B5 and B2/B3 revocation hooks. Permit multiple account viewers; acquire a transactional sharing lease per account/route and validate sender generation for every position.
- Separate sender timeout from viewer presence. Release on stop/offline/revocation/leave/close/delete; fence late packets, stale timers, and reconnects. Implement restart recovery.
- Done when concurrent Start yields one winner; different routes allow concurrent senders; viewers cannot keep a dead sender active; no takeover is possible; session revocation closes affected sockets and sharing. Preserve existing GPS validation and slot limits.

## AUTH-02: Web

### W1: Authentication and account settings

- Depends on B1 fixtures, accepted auth UI. Implement email/Google signup, confirmation fields, login, verification/resend, recovery/reset, password change, and logout actions with accessible validation and autofill.
- Preserve approved destinations across redirects, verification, restarts, and expired sessions; protect against external redirects. Handle pending accounts and fixed identifiers.
- Done when responsive browser checks cover keyboard use, matching/mismatching confirmation, Google cancellation, expired links, verification elsewhere, cookie restart, cross-tab revocation, and destination recovery. Integrate B2–B4 before marking complete.

### W2: My routes and account route integration

- Depends on B1 fixtures, W1, and the reviewed My routes UI. Build list, tabs, all three filters, pagination, retained view state, actions, and loading/empty/offline/error states. Give every route a Share action using the existing web route-screen share flow and feedback; test native share, cancellation, copy and manual fallbacks.
- Remove editable route aliases, restore account access, support concurrent watching, and show other-device sharing restriction. Account menu links to W1 settings. Create/Join stay explicit actions; opening a list item never starts tracking.
- Done when reviewed desktop/phone layouts and B5/B6 integration cover all filters, left-route rejoin, archive gates, back navigation, data isolation after account switch, logout cleanup, and multi-device sharing.

## AUTH-03: Android

### A1: Authentication and account settings

- Depends on B1 fixtures, accepted auth UI; no web implementation dependency. Add secure session storage, Google native/system-browser flow, app-link handling, all account forms and settings, and pending activation.
- Preserve destination through browser/app transitions and process recreation without retaining passwords. Configure client IDs and approved links through deployment settings.
- Done when emulator tests cover keyboard/autofill, confirmation mismatch, app restart, Google cancellation, invalid links, verification elsewhere, expiry, and revocation. Integrate B2–B4 before completion; real-device checks are explicit release gates.

### A2: My routes and native route integration

- Depends on B1 fixtures, A1, and the reviewed My routes UI. Implement native list/tabs/filters, pagination, back-state restoration, account menu, and all empty/loading/offline/error states. Give every route a Share action using the existing native route-screen share flow and feedback; test active, archived, left, and cached offline links.
- Replace device identity and restore account memberships; allow concurrent watchers; stop native location service on revoked access and reject obsolete sender generations.
- Done when B5/B6 integration covers rejoin/archive authorization, account switching, screen-off sharing, remote logout/reset, network recovery, and independent routes on multiple devices. Verify no automatic tracking start on login or list navigation.

## AUTH-04: Release

### R1: Configuration and cross-client acceptance

- Depends on B2–B6, W1/W2, A1/A2. Configure Google web/Android clients, redirect/app links, trusted origins, session/email settings, and SMTP secrets. Verify Gmail SMTP compatibility if chosen; implement the selected authentication mode rather than assuming ordinary account-password login works.
- Test actual delivery to owned test inboxes, Google login on both clients, same-account route restoration, verification opened elsewhere, simultaneous sharing, reset/logout revocation, and background-service shutdown.
- Done when acceptance results and remaining device limits are recorded, with no credentials in fixtures or docs.

### R2: Fresh-start rollout and documentation

- Depends on R1. Rehearse database reset/migrations in a disposable environment, legacy credential rejection, client local-state invalidation, coordinated API/client deployment, and rollback/maintenance behavior.
- Obtain explicit authorization for the concrete production reset/deployment. This plan authorizes no production deletion.
- Update owning product/system references, status and roadmap to implemented behavior, and record evidence. Follow up account deletion separately; identifier editing and iOS remain out of scope.
- Done when rollout is verified, required checks pass, application graph is fresh, and the next-session handoff accurately states completed and pending tasks.

## Release boundaries

- Owner accepted credential/default proposals and authorized implementation on 2026-10-21. The contract fixes the engineering defaults for independent tasks.
- B5/B6, W1/W2, A1/A2 and R1/R2 remain pending. Do not expose the staged account API as a complete login system or treat pending signup as usable route authority.
- Production data reset and deployment still require explicit authorization for the concrete rollout.
