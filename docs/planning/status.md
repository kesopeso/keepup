---
type: plan
status: active
---

# Delivery status

Current progress and material validation limits. Read only for task selection, delivery questions, or implementation gaps. Dated checks live in [validation history](validation-history.md) and are loaded only when evidence is needed.

## Immediate next step

Account authentication B1–B6, W1/W2 web account/route integration and A1/A2 Android account/route integration are implemented. The [task handoff](authentication-tasks.md) continues with R1 configuration and cross-client/device acceptance. Authentication remains disabled by default. W2 has real backend browser checks in a disposable schema, 28 passing web regression tests, a production build, responsive review and zero dashboard accessibility violations. W1 browser-restart restoration remains verified. A1/A2 native evidence and limits are recorded below. Real Google/email and physical-device account-mode release checks remain R1.

Account mode requires verified sessions and implements multiple viewers, exclusive sharing generations, synchronous revocation cleanup and startup recovery. Its process-local live hub requires one API runtime per database/schema. Backend tests use isolated databases/schemas through migration 0007, locally signed Google proofs and local TLS SMTP. No application tables were migrated/reset and nothing was deployed.

Account-mode Android owner edit/close/delete controls are included in A2. Equivalent legacy-shell controls remain deferred during the account transition. See the [Android implementation sequence](roadmap.md#phase-9-mobile-client-android-first).

## Mobile foundation

Android creation, joining, secure owner/member storage, restored snapshots, native maps, live viewing, foreground GPS sharing, and screen-off tracking are implemented. The app includes connection checking, shared branded headers, and the full-screen map with a Peek/Full drawer. Source ownership is in [mobile boundaries](../system/mobile.md); commands are in the [Android workflow](../workflow/mobile-development.md). iOS is deferred.

The owner reported successful outdoor testing of foreground and screen-off tracking with the standalone production-connected Android build on 2026-10-07. Those physical-review checkpoints are complete on that report. Android version, elapsed duration, per-scenario network recovery, and battery measurements were not supplied; no broader device reliability is inferred.

Route creation and the drawer layout have Docker TypeScript/regression checks and emulator review recorded on 2026-10-08. The owner confirmed emulator pinch zoom; physical-phone review of creation and the later drawer/gesture changes remains unverified. Dated evidence is separate for [creation](validation-history.md#android-route-creation-validation-on-2026-10-08) and [drawer layout](validation-history.md#android-route-map-layout-implemented-on-2026-10-08).

A1 has Docker TypeScript/regression checks, a compiled Android APK, real B2–B4 native requests, and emulator account/link/restart/cancellation checks. A2 adds native My routes, account memberships, lifecycle controls and ticket/generation live authority, with 111 passing mobile tests and real B5/B6 and emulator checks. Account mode remains opt-in. Real Google/email, verified domain association, physical-device password-manager/autofill and account-mode power/network acceptance remain release gates. See [native account boundaries](../system/mobile.md#account-routes-and-live-authority) and [A2 evidence](validation-history.md#android-account-routes-a2-on-2026-10-22).

## Android implementation checkpoints

| Task | State | Remaining limit / next acceptance |
|---|---|---|
| Live viewing | Complete; emulator validated | Device-specific behavior requires review when affected |
| Foreground location sharing | Complete; owner-reported outdoor validation | Detailed device, duration, and network-recovery measurements not supplied |
| Screen-off tracking | Complete; owner-reported outdoor validation | Detailed battery, manufacturer restrictions, and screen-lock duration not supplied |
| Android route creation | Complete; emulator validated | Physical-phone form review remains unverified |
| Native map and Peek/Full drawer | Complete; emulator reviewed | Physical-phone gesture review remains unverified |
| Account authentication/settings | Implemented; emulator and B2–B4 checked | Real Google/email, domain association and physical-device autofill remain R1 |
| Account My routes/live integration | Implemented; B5/B6 and emulator checked | Physical-device account-mode screen-off/network/power acceptance remains R1 |
| Android owner controls | Implemented in account mode | Legacy shell still lacks owner controls; physical-device account-mode review remains R1 |

## UI redesign

Web create/join forms, route map, sharing controls, member presence, dialogs, and brand assets are implemented. Browser layout/accessibility checks and owner-reported physical-phone testing are recorded in [UI validation](validation-history.md#ui-redesign). The owning facts live in [frontend boundaries](../system/frontend.md) and [experience](../product/experience.md).

The selected web landing design and landing/create navigation rollout are implemented; see [frontend boundaries](../system/frontend.md#current-frontend-foundation).

## Web HTTP sharing validation

Development Start/Stop sharing on HTTP IP origins is validated with simulated GPS. Real browser geolocation still requires a secure origin. See [dated HTTP checks](validation-history.md#web-http-sharing-validation) and [current frontend behavior](../system/frontend.md#current-frontend-foundation).

## Web development clipboard fallback validation

The development-only legacy copy fallback has desktop browser validation and owner-reported mobile Chrome success. The recorded production build excludes that fallback. See [dated clipboard checks](validation-history.md#web-development-clipboard-fallback-validation); [frontend boundaries](../system/frontend.md) own the behavior.

## Implemented areas

| Area | Owning reference |
|---|---|
| Compose tooling, API startup, health, manual migrations, deployment | [Development](../workflow/development.md), [backend](../system/backend.md) |
| Web landing, creation/joining, identity, snapshots, owner close/delete, maps, sharing, branding | [Frontend](../system/frontend.md) |
| Android creation/joining, saved access, native maps, live viewing, sharing | [Mobile](../system/mobile.md) |
| REST lifecycle, snapshots, WebSocket authentication, presence, GPS validation | [API](../system/api-and-live.md) |
| Stored segments and accepted points | [Persistence](../system/data.md) |

## Specification versus implementation

Legacy mode rejects new memberships on closed routes despite the [archive access requirement](../product/routes.md#route-access). Existing members can read archive snapshots. The current legacy Android client reports this limitation. Account mode supports explicit archive rejoin, including password gates, and W2 uses it. Android owner edit/close/delete controls remain unimplemented.

Metadata editing exists in the API but lacks current client controls. W2 exposes account membership leave; legacy web and current Android clients do not; see [web boundaries](../system/frontend.md#current-frontend-foundation) and [mobile saved access](../system/mobile.md#saved-membership). This gap does not change the selected next task.

Shared GPS validation is implemented. API race/PostGIS integration tests, Go vet, and lint passed in the recorded 2026-10-03 checks; presence recovery also has recorded 2026-10-07 verification. These are prior results, not fresh validation of the current checkout. See [GPS evidence](validation-history.md#gps-validation) and [mobile evidence](validation-history.md#mobile-validation).

Observability has structured logs and GPS rejection counters; the full metrics set remains desired work in [backend boundaries](../system/backend.md#observability). Dedicated Android release signing remains required for public distribution; see [release builds](../workflow/mobile-development.md#standalone-android-release-build-for-outdoor-testing).

## Knowledge-base maintenance

Use the [context map](../CONTEXT.md) and [documentation policy](../workflow/documentation.md). Maintain current owning references during relevant repository changes with icm-architect; dated history is separate and optional context.
