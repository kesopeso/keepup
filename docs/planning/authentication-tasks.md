---
type: plan
status: active
---

# Authentication implementation tasks

## Handoff

Implementation is authorized with the accepted defaults. Product rules belong to the [PRD](../product/authentication-prd.md); architecture belongs to [ADR 0001](../adr/0001-account-authentication.md). Authentication and My routes visual reviews are complete. The PRD links portable design snapshots and records feedback. The [v1 contract and fixtures](../system/authentication-contract.md) support independent client work. Execute application tools in Docker and refresh Graphify after application edits as required by [documentation discipline](../workflow/documentation.md).

Each task below is an independent change with its own validation. Web and Android depend on the shared contract, not on one another. The parent delivery groups remain AUTH-01 through AUTH-04 in the PRD.

## Resume point

B1–B6, W1/W2 and A1/A2 implementation are complete. Continue with R1 configuration and cross-client/device acceptance. Browser-restart restoration is verified; real Google and email delivery remain R1. `AUTH_ENABLED` remains false by default; enabling it replaces route/socket authority as well as mounting account endpoints. Checks apply migrations through 0007 only to disposable databases/schemas; no application tables were migrated/reset and nothing was deployed. Web and Android both support account route authority.

B2 implements pending signup, normalized uniqueness and immutable identifiers, Argon2id, email/username login, hashed opaque sessions, idle extension, explicit atomic rotation, current/all-session revocation, configurable attempt limits, and protected browser/native transports. B3 adds purpose-bound token consumption/replacement, pending activation, reset/change revocation, encrypted transactional outbox, SMTP/fake providers, quotas, trusted-proxy support, and cleanup. B4 adds Google proof/onboarding/linking; B5 applies active-account authority to routes, dashboard lists and live bootstrap. B6 adds independent viewers, exclusive sharing generations and immediate live revocation cleanup. Race-enabled PostgreSQL and local TLS SMTP tests cover these boundaries. Web and native link/form UI are implemented; actual SMTP credentials/proxy CIDRs/real delivery remain R1.

## AUTH-01: Backend

### B1: Define contracts and fixtures

State: complete. Endpoints, transports, policies, route summaries, live events and deterministic success/failure fixtures are in the shared contract. B2–B6 implement the backend; client integration remains separate.

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

State: complete for backend account authority. Source: [recovery service](../../apps/api/internal/auth/recovery.go), [token transactions](../../apps/api/internal/auth/recovery_postgres.go), [queue worker](../../apps/api/internal/auth/outbox.go), [providers](../../apps/api/internal/maildelivery/mail.go), and [migration 0004](../../db/migrations/0004_auth_recovery_and_outbox.up.sql). [Configuration](../workflow/authentication-email.md) covers required secrets and settings. No real email was sent. B5/B6 implement account-bound route/live cleanup; web and native confirmation matching and email-link UI are implemented.

- Depends on B2. Implement purpose-bound token hashes, atomic consumption/replacement, verification, resend, forgot/reset password, and authenticated password change.
- Add configurable provider interface, SMTP, fake delivery, delivery queue/retry strategy, templates, secret configuration, and rate limits. Handle scanners without consuming tokens on GET.
- Done when expiry/replay/replacement, mismatch validation, generic recovery responses, SMTP failure/retry, pending activation restrictions, and required session revocation are tested. Use fake mail in automated tests.

### B4: Google accounts

State: complete for the backend ID-token flow. Source: [verifier](../../apps/api/internal/auth/google_verifier.go), [flow/account transactions](../../apps/api/internal/auth/google_postgres.go), [challenge queueing](../../apps/api/internal/auth/google_email.go), [HTTP endpoints](../../apps/api/internal/httpapi/google.go), and [migration 0005](../../db/migrations/0005_google_authentication.up.sql). [Google configuration](../workflow/authentication-google.md) covers audiences, nonce transport, optional enablement and client responsibilities. Tests use real signatures with a local key endpoint plus mocked identity-provider claims against isolated PostgreSQL schemas. Web SDK/buttons/link pages are implemented in W1; A1 implements native Credential Manager and account forms. Real Google acceptance remains R1. OAuth code callbacks are not implemented.

- Depends on B2/B3. Validate Google proofs for approved web/native clients, map provider subjects, add username/password onboarding, and link existing accounts without replacing their password.
- Cover authoritative email, third-party email challenge, cancelled/expired onboarding, provider collision, and pending-account claim protection.
- Done when mocked identity-provider tests reject bad issuer/audience/signature/expiry/replay and concurrent signup/linking cannot duplicate or hijack accounts. Real Google smoke checks belong to R1.

### B5: Account route authorization and My routes API

State: complete for the staged account-mode backend. Source: [account route service](../../apps/api/internal/routes/accounts.go), [My routes queries/cursors](../../apps/api/internal/routes/account_list.go), [ticket/live authority](../../apps/api/internal/routes/account_live.go), [HTTP routes](../../apps/api/internal/httpapi/account_routes.go), and [migration 0006](../../db/migrations/0006_account_routes.up.sql). Tests cover account isolation, pending/expired/revoked sessions, legacy-mode separation, concurrent/idempotent join, leave/rejoin path history, archived access, all filter combinations, signed cursor scope/tie-breaking, ticket replay and real HTTP/WebSocket transport. Rollback refuses silent demotion of account-owned routes. No application database migration/reset occurred. Client integration is owned by W2/A2. B6 adds multiple viewers, sender generations/leases, and synchronous revocation/socket cleanup.

- Depends on B2 and B1's route contract. Replace independent member/owner token authority; scope route REST and live authorization to active account sessions and membership/ownership.
- Persist membership episodes; restore current access across devices; use username for display; support leave/rejoin without erasing old history.
- Implement Active/Archived list, additive My/Joined/Left filters, counts, sorting, pagination, deduplication, and former-member-safe responses. Rejoining must recheck access/password and move categories.
- Done when API tests cover ownership isolation, pending accounts, stale legacy credentials, repeated join, former-member privacy, all filter combinations, pagination without duplicates, deleted routes, and archive access.

### B6: Multiple viewers and exclusive sharing

State: complete for the staged account-mode backend. Source: [leases and tickets](../../apps/api/internal/routes/account_live.go), [presence/deadlines/recovery](../../apps/api/internal/routes/account_presence.go), [account sockets](../../apps/api/internal/httpapi/account_connections.go), [revocation hook](../../apps/api/internal/auth/revocation.go), and [migration 0007](../../db/migrations/0007_account_sharing.up.sql). Race-enabled API/database/HTTP/WebSocket tests, vet and lint pass. Tests cover simultaneous same-account starts and last-slot contention, different-route senders, viewer/sender independence, accepted-GPS-only renewal, stale recovery, reconnect/generation fencing, lifecycle/expiry release, startup recovery/singleton enforcement, immediate lock-loss fencing, and real WebSocket generation/privacy/capabilities. Logout, logout-all, reset and password change synchronously release sharing and close affected sockets; cleanup failure rolls back revocation. Delayed notifications cannot close fresh logins created after commit. The account live runtime requires one API process per database/schema while the hub is process-local. Client integration is implemented in W2/A2; application rollout migrations and deployment remain pending.

- Depends on B5 and B2/B3 revocation hooks. Permit multiple account viewers; acquire a transactional sharing lease per account/route and validate sender generation for every position.
- Separate sender timeout from viewer presence. Release on stop/offline/revocation/leave/close/delete; fence late packets, stale timers, and reconnects. Implement restart recovery.
- Done when concurrent Start yields one winner; different routes allow concurrent senders; viewers cannot keep a dead sender active; no takeover is possible; session revocation closes affected sockets and sharing. Preserve existing GPS validation and slot limits.

## AUTH-02: Web

### W1: Authentication and account settings

State: complete for W1. Source and behavior are in [web account boundaries](../system/frontend.md#account-authentication). Docker web tests and production build pass. Browser checks integrate password/session, verification/recovery and prepared post-proof Google contexts with the real backend in a disposable schema. SDK cancellation uses a local stub. [Dated evidence](validation-history.md#web-authentication-w1-on-2026-10-21) distinguishes real API checks, fixtures and remaining limits. Pending and verified sessions, retained destinations, and rejection of revoked saved cookies are verified across Chromium process restarts using owner-authorized disposable browser state. Real Google/email acceptance remains R1. W2 owns account route authority, navigation and `/my-routes`.

- Depends on B1 fixtures, accepted auth UI. Implement email/Google signup, confirmation fields, login, verification/resend, recovery/reset, password change, and logout actions with accessible validation and autofill.
- Preserve approved destinations across redirects, verification, restarts, and expired sessions; protect against external redirects. Handle pending accounts and fixed identifiers.
- Done when responsive browser checks cover keyboard use, matching/mismatching confirmation, Google cancellation, expired links, verification elsewhere, cookie restart, cross-tab revocation, and destination recovery. Integrate B2–B4 before marking complete.

### W2: My routes and account route integration

State: complete. [Web boundaries](../system/frontend.md#my-routes-and-account-route-authority) own the implementation. Docker regression tests, production build, real account API/browser checks and responsive/accessibility review passed. [Dated evidence](validation-history.md#web-my-routes-w2-on-2026-10-22) separates real API checks from synthetic GPS/share/offline fixtures and remaining native/provider release gates. Authentication remains disabled by default.

- Depends on B1 fixtures, W1, and the reviewed My routes UI. Build list, tabs, all three filters, pagination, retained view state, actions, and loading/empty/offline/error states. Give every route a Share action using the existing web route-screen share flow and feedback; test native share, cancellation, copy and manual fallbacks.
- Remove editable route aliases, restore account access, support concurrent watching, and show other-device sharing restriction. Account menu links to W1 settings. Create/Join stay explicit actions; opening a list item never starts tracking.
- Done when reviewed desktop/phone layouts and B5/B6 integration cover all filters, left-route rejoin, archive gates, back navigation, data isolation after account switch, logout cleanup, and multi-device sharing.

## AUTH-03: Android

### A1: Authentication and account settings

State: implementation complete. [Native account boundaries](../system/mobile.md#account-authentication) own the sources. Docker TypeScript and 98 mobile tests passed at A1, and the Android development APK compiled with the native Credential Manager module. Emulator/real B2–B4 checks cover password/session restoration, confirmation, pending activation, links, reset/change, native Google cancellation and prepared post-proof contexts. [Dated evidence](validation-history.md#android-authentication-a1-on-2026-10-22) distinguishes emulator behavior, API fixtures and remaining Google/email/domain/device acceptance. An actual password-manager fill still needs release-device acceptance; hints and keyboard entry are implemented. A2 now implements native route entry/My routes.

- Depends on B1 fixtures, accepted auth UI; no web implementation dependency. Add secure session storage, Google native/system-browser flow, app-link handling, all account forms and settings, and pending activation.
- Preserve destination through browser/app transitions and process recreation without retaining passwords. Configure client IDs and approved links through deployment settings.
- Done when emulator tests cover keyboard/autofill, confirmation mismatch, app restart, Google cancellation, invalid links, verification elsewhere, expiry, and revocation. Integrate B2–B4 before completion; real-device checks are explicit release gates.

### A2: My routes and native route integration

State: implementation complete. [Native account routes](../system/mobile.md#account-routes-and-live-authority) own the list, forms, lifecycle controls, ticket sockets and sender-generation integration. Docker TypeScript/regression checks, real B5/B6 requests and emulator native-socket/service checks are recorded in [dated evidence](validation-history.md#android-account-routes-a2-on-2026-10-22). Physical-device account-mode screen-off/network/power acceptance remains R1.

- Depends on B1 fixtures, A1, and the reviewed My routes UI. Implement native list/tabs/filters, pagination, back-state restoration, account menu, and all empty/loading/offline/error states. Give every route a Share action using the existing native route-screen share flow and feedback; test active, archived, left, and cached offline links.
- Replace device identity and restore account memberships; allow concurrent watchers; stop native location service on revoked access and reject obsolete sender generations.
- Done when B5/B6 integration covers rejoin/archive authorization, account switching, screen-off sharing, remote logout/reset, network recovery, and independent routes on multiple devices. Verify no automatic tracking start on login or list navigation.

## AUTH-04: Release

### R1: Configuration and cross-client acceptance

- Depends on B2–B6, W1/W2, A2. Configure Google web/Android clients, redirect/app links, trusted origins, session/email settings, and SMTP secrets. Verify Gmail SMTP compatibility if chosen; implement the selected authentication mode rather than assuming ordinary account-password login works.
- Test actual delivery to owned test inboxes, Google login on both clients, same-account route restoration, verification opened elsewhere, simultaneous sharing, reset/logout revocation, and background-service shutdown.
- Done when acceptance results and remaining device limits are recorded, with no credentials in fixtures or docs.

### R2: Fresh-start rollout and documentation

- Depends on R1. Rehearse database reset/migrations in a disposable environment, legacy credential rejection, client local-state invalidation, coordinated API/client deployment, and rollback/maintenance behavior.
- Obtain explicit authorization for the concrete production reset/deployment. This plan authorizes no production deletion.
- Update owning product/system references, status and roadmap to implemented behavior, and record evidence. Follow up account deletion separately; identifier editing and iOS remain out of scope.
- Done when rollout is verified, required checks pass, application graph is fresh, and the next-session handoff accurately states completed and pending tasks.

## Release boundaries

- Owner accepted credential/default proposals and authorized implementation on 2026-10-21. The contract fixes the engineering defaults for independent tasks.
- R1/R2 remain pending. W1/W2 and A1/A2 web/native account and route implementation are complete. Complete release acceptance before exposing account mode; pending signup never grants usable route authority.
- Production data reset and deployment still require explicit authorization for the concrete rollout.
