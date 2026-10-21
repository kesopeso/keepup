---
type: plan
status: draft
---

# Authentication implementation tasks

## Handoff

No implementation has started. Product rules belong to the [PRD](../product/authentication-prd.md); architecture belongs to [ADR 0001](../adr/0001-account-authentication.md). Authentication and My routes visual reviews are complete. The PRD links portable design snapshots and records feedback. Do not infer architecture acceptance from UI review. Resolve explicitly proposed details before the affected implementation task. Execute application tools in Docker and refresh Graphify after application edits as required by [documentation discipline](../workflow/documentation.md).

Each task below is an independent change with its own validation. Web and Android depend on the shared contract, not on one another. The parent delivery groups remain AUTH-01 through AUTH-04 in the PRD.

## Resume point

Start at B1, not application implementation. Review the remaining proposed items below with the owner, finalize the shared contract and defaults, then implement B2 onward. All 12 tasks are pending. Web and Android can proceed independently against B1 fixtures, then integrate the implemented backend. No deployment, database reset, or commit has been performed by this planning task.

## AUTH-01: Backend

### B1: Define contracts and fixtures

- Depends on accepted PRD/ADR. Specify auth endpoints, cookie/native transports, Google proof exchange, pending state, redirects, validation, errors, session expiry/rotation, and single-use socket tickets.
- Specify account membership, owner authority, sender/viewer presence, sharing generations, route-list schema, filter semantics, pagination, and safe former-member summaries.
- Fix configurable defaults and boundaries: verification/reset TTLs, resend and attempt limits, ticket TTL, idle session policy, and mail retries. Record Google third-party email and pending-account claim behavior.
- Done when versioned request/response and live-event examples cover success, failure, expiry, revocation, and conflict; both client tasks can use deterministic fixtures. Document unresolved questions instead of inventing incompatible client rules.

### B2: Account persistence and password login

- Depends on B1. Add user IDs, case-insensitive unique email/username lookup keys, immutable display identifiers, password hashes, account state, session hashes and indexes.
- Implement signup, email-or-username login, pending restrictions, current-account lookup, session expiry/rotation, logout, and logout everywhere. Include origin/CSRF enforcement for browser mutations and native credential transport.
- Done when database-backed tests prove case collisions, concurrent signup, password boundaries, idle expiry, revocation, and inability to mutate identifiers. Secrets must not reach logs.

### B3: Verification, recovery, and delivery

- Depends on B2. Implement purpose-bound token hashes, atomic consumption/replacement, verification, resend, forgot/reset password, and authenticated password change.
- Add configurable provider interface, SMTP, fake delivery, delivery queue/retry strategy, templates, secret configuration, and rate limits. Handle scanners without consuming tokens on GET.
- Done when expiry/replay/replacement, mismatch validation, generic recovery responses, SMTP failure/retry, pending activation restrictions, and required session revocation are tested. Use fake mail in automated tests.

### B4: Google accounts

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

## Remaining review decisions

- Confirm exact username/password alphabet and maximum password length, Google third-party email ownership challenge, session/queue/lease design, and rejoin episode model from the PRD/ADR.
- Visual review of My routes is complete. Confirm proposed ordering, pagination, view-state persistence, and former-member access behavior before finalizing B1.
- After review, mark accepted requirements separately from implementation status; preserve any unresolved item as an explicit blocker to its task.
