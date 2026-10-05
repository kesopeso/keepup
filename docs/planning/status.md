---
type: plan
status: active
---

# Delivery status

## Immediate next step

Android route joining, saved membership, authenticated snapshots, and native snapshot maps are implemented. The proposed next task is live updates and native location sharing through the existing protocol; see the [Android implementation sequence](roadmap.md#phase-9-mobile-client-android-first).

The previous physical-phone outdoor review of the web client is complete. Native tracking still needs its own physical-device validation.

## Mobile foundation

The React Native/Expo app and local Android workflow are ready. The mobile app joins routes by code or pasted link, stores membership securely, restores the last route, and displays authenticated route/member snapshots with a native map, refresh, and retry. A separate connection screen checks shared API/database readiness. Implementation boundaries are owned by [mobile](../system/mobile.md); commands and debugging by the [Android workflow](../workflow/mobile-development.md). Live updates, native location sharing, and background tracking are not implemented. iOS is deferred until after Android.

Validation on 2026-10-05:

- Inspected the Android build output: Gradle reported `BUILD SUCCESSFUL in 2m 35s`, and Expo installed and opened the development APK on the Android 17/API 37 emulator.
- The owner confirmed the starter screen appeared and later confirmed browser React Native DevTools worked after the hostname and headless workflow corrections.
- ADB forwarding and the browser debugger endpoint were checked during diagnosis. These checks establish the development setup, not KeepUp API connectivity or native tracking reliability.

API connection validation on 2026-10-05:

- Mobile TypeScript checking and all eight request/configuration tests passed through Docker Compose. Tests cover healthy and degraded responses, incorrect successful responses, network recovery, request/body timeout, cancellation, and release configuration.
- The Android 17/API 37 emulator displayed Connected to KeepUp against the existing proxy/API/PostGIS stack. Stopping the API produced the unavailable state and Retry. Restarting the API and tapping Retry returned to connected. The Android accessibility hierarchy confirmed the checking button was disabled while the request ran.
- The temporary Metro session was stopped after verification and the backend was restored. The screen needs no native dependency or APK rebuild; start Metro with the existing launcher.

Android joining validation on 2026-10-05:

- Mobile TypeScript checking and all 20 tests passed through Docker Compose. The additional tests cover code/link parsing, HTTP error distinctions, authenticated snapshots, persistent identity, backend isolation, rejected joins, invalid-token cleanup, and retry after failed snapshot/storage operations without duplicate membership.
- Regenerated the Android project with the SecureStore plugin, successfully built the development APK with Expo Crypto and SecureStore, and installed it on the running Android 17/API 37 emulator.
- Emulator checks passed for invalid route codes, a pasted link to a disposable password-protected route, incorrect password, duplicate display name, a successful two-member snapshot, and force-stop/reopen with the same membership. Failed refresh retained the dated snapshot and offered Retry. The API was restored after the outage check.
- The actual mobile API/session code loaded the disposable route's closed archive and resumed saved access without another join. Deleting that disposable route produced invalid access and cleared its test session's membership and last-route credentials. This archive/deletion check used a separate test repository; it did not alter the owner's Android storage. The emulator remained available for the owner's concurrent testing.
- Metro is running for review. Other installations need a rebuilt APK for the added native modules; see the [Android workflow](../workflow/mobile-development.md#route-joining-and-saved-access).

Android snapshot map validation on 2026-10-05:

- Mobile TypeScript checking and all 25 tests passed through Docker Compose. Map tests cover separate path segments, latest timestamps, member colors/status, empty and single-point geometry, invalid samples, antimeridian paths/bounds, and malformed snapshot rejection.
- Regenerated the Android project, built the MapLibre development APK successfully, and installed it on the Android 17/API 37 emulator. The existing saved route displayed the native basemap and empty-location message.
- A temporary in-memory fixture verified colored paths/markers, visible segment gaps, manual pan, refresh without recentering, Fit group becoming hidden and inaccessible after fitting, member focus with written status/timestamp, and an empty archive. The fixture was removed and the normal app entry restored without backend or membership changes.
- An unavailable tile endpoint exposed MapLibre's separate native tile-error reporting. The map now turns those errors into readable feedback and Retry map. Restoring tile access and retrying recovered the basemap. The real provider was restored after the check.
- Documentation links and heading fragments resolve. Metro remains running for review. Native GPS sharing and live updates are still pending; this establishes snapshot rendering, not native tracking reliability.

## UI redesign

The user selected the full layout refresh in an earlier Lavish visual proposal, then requested implementation. The app now uses simpler create/join forms, a map-led route layout, sharing controls above collapsible members, explicit presence badges, and a route-details dialog. Behavior is owned by [map experience](../product/experience.md); implementation boundaries by [frontend](../system/frontend.md).

Validation on 2026-09-26:

- Production web build and all four existing live-state tests passed through Docker Compose.
- Chromium checks passed for create, password errors and protected join, live member insertion, start/stop sharing, clipboard sharing, duplicate connections, close/archive transitions, and code-confirmed deletion using disposable local routes.
- Isolated browser fixtures covered all five presence states, long names, restricted sharing, full tracking slots, stale recovery, member collapse, map resizing, and manual viewport preservation.
- Layout checks passed at 360px, 390px, 640px, 844px landscape, and 1280px. Automated accessibility checks found no violations on create, live route, or the populated member sheet. Nested dialogs restore focus and close confirmation initially focuses Cancel.

Validation reported by the owner on 2026-09-30:

- Physical-phone outdoor testing passed for map readability, location permissions, and native sharing.

## Implemented areas

| Area | Owning reference |
|---|---|
| Compose stack, API startup, health checks, manual migrations | [Development](../workflow/development.md), [backend](../system/backend.md) |
| Mobile starter, Android development build, Metro and browser debugging | [Mobile](../system/mobile.md), [Android workflow](../workflow/mobile-development.md) |
| Mobile API health check, timeout, unavailable state, and retry | [Mobile](../system/mobile.md#shared-backend-integration) |
| Android route joining, secure membership persistence, restored snapshots, and manual refresh | [Mobile](../system/mobile.md#saved-membership) |
| Android native snapshot map, saved paths and markers, member focus, and Fit group | [Mobile map](../system/mobile.md#native-snapshot-map) |
| Create/join, browser identity, authenticated snapshots | [Frontend](../system/frontend.md) |
| Route lifecycle REST operations and owner controls | [API](../system/api-and-live.md), [frontend](../system/frontend.md) |
| MapLibre rendering, viewport control, snapshot and live path state | [Frontend](../system/frontend.md) |
| WebSocket authentication, sharing commands, position ingestion | [API and live protocol](../system/api-and-live.md) |
| Shared GPS validation and atomic rejection behavior | [Tracking](../product/tracking.md), [API](../system/api-and-live.md) |
| Saved segments/points, snapshot history | [Persistence](../system/data.md) |
| Presence transitions, duplicate connection rejection, stale recovery | [API](../system/api-and-live.md), [frontend](../system/frontend.md) |
| UI refresh, simpler entry forms, sharing, accessible dialogs, and presence styling | [Experience](../product/experience.md), [frontend](../system/frontend.md) |
| K4-10 logo in the site and route headers and platform icon exports | [Frontend](../system/frontend.md) |
| Stacked Share feedback cards for clipboard success and manual-copy fallback | [Frontend](../system/frontend.md) |
| Owner close/delete confirmations | [Frontend](../system/frontend.md) |

This consolidates the previous 23-item implementation status without making the plan a second behavior specification. It records existing project status; the documentation restructure did not run application tests.

## Specification versus implementation

The shared API currently rejects new memberships on closed routes, despite the [product requirement](../product/routes.md#route-access) to keep archives accessible by code/password. Existing members can load archive snapshots. The mobile join flow reports this existing backend limitation and does not create a new closed-route membership.

The shared API implements the [GPS validation rules](../product/tracking.md#gps-validation), including accuracy limits, required and ordered timestamps, and impossible-jump rejection. See [protocol and transaction behavior](../system/api-and-live.md#gps-validation).

Validation on 2026-10-03:

- Full API tests passed through Docker Compose with the race detector and PostGIS integration tests enabled.
- Isolated database schemas covered concurrent duplicate submissions, rejected-point side effects, persisted baselines across service instances, geographic antimeridian movement, segment resets, and concurrent stop/leave/close operations.
- WebSocket tests covered each GPS rejection code, malformed timestamps, continued sharing after rejection, and stale timers unaffected by rejected samples.
- Go vet and golangci-lint passed. No new database migration is required.



## Knowledge-base maintenance

The Markdown knowledge base is organized by ownership with folder contracts and stable entry paths. See the [migration record](icm-migration.md).
