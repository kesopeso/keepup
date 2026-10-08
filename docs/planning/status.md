---
type: plan
status: active
---

# Delivery status

## Immediate next step

Implement Android owner controls to edit, close, and delete routes using the shared product rules and API. Android route creation is complete. See the [Android implementation sequence](roadmap.md#phase-9-mobile-client-android-first).

The owner reported successful outdoor testing of the standalone production-connected Android build on 2026-10-07. The foreground and screen-off tracking physical-review checkpoints are complete on that report; detailed duration, network-recovery, and battery measurements were not supplied.

## Mobile foundation

Added `make mobile-build` on 2026-10-08 to regenerate, build, and install the Android development APK before starting Metro. The Metro launcher now checks for a single ready device, an installed development client, and support for the current development scheme. Shell syntax, the Make command sequence, and seven simulated ADB outcomes passed validation; the installed emulator app passed the live compatibility check. A complete rebuild was not run for this tooling change. See [daily development](../workflow/mobile-development.md#daily-development).

On 2026-10-08, the owner approved the mobile logo preview in Lavish. Implemented the shared logo header and lowercase white/green wordmark across startup, joining, route viewing, archives, and connection checks, plus the transport dropdown. Docker TypeScript checking and all 68 mobile tests passed. Device review of these UI changes remains unverified; no native dependency or APK rebuild is required for a development build. See [mobile boundaries](../system/mobile.md#current-source-ownership).

The React Native/Expo app and local Android workflow are ready. The mobile app joins routes by code or pasted link, stores membership securely, restores the last route, and displays authenticated route/member snapshots with a native map, refresh, and retry. A separate connection screen checks shared API/database readiness. Implementation boundaries are owned by [mobile](../system/mobile.md); commands and debugging by the [Android workflow](../workflow/mobile-development.md). Live viewing and foreground native location sharing are implemented. The owner reported successful physical-phone outdoor validation of foreground and screen-off sharing on 2026-10-07. iOS is deferred until after Android.

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

Android route creation validation on 2026-10-08:

- Added the native creation form with required route/display names, saved transport selection, optional description/password, and both sharing policies. Successful creation saves member and owner access in backend-scoped SecureStore and opens the native route screen without starting GPS.
- Docker TypeScript checking and all 79 mobile tests passed. Eleven creation tests cover validation, response rejection, password handling, backend scope, restored owner access, concurrent submissions, credential/last-route write failures, cancellation, request recovery, snapshot failures, and invalid-access cleanup.
- The Android 17/API 37 emulator displayed required-name feedback and created a password-protected, owner-only-sharing route with its description and saved Bicycle selection. A temporary runtime SecureStore failure after the response showed the created code and disabled navigation/inputs. Retry saved access and opened the route; the observed create POST count remained one.
- Force-stop/reopen restored the protected route without a password prompt and displayed the same membership as You and Owner, spectating. Deleted the disposable routes with their returned owner credentials, verified deleted-access cleanup, and restored the emulator's original saved route. Runtime fault injection and temporary cleanup files were removed.
- This change uses existing native modules and needs no development APK rebuild. Physical-device review of the creation form remains unverified. The existing create endpoint has no idempotency support; lost responses and app termination before recovering failed credential storage remain access-recovery limits.

## Android implementation checkpoints

| Task | State | Acceptance |
|---|---|---|
| Live viewing | Complete | Automatic location, member, and route updates; reconnect catch-up; foreground recovery; manual viewport preservation; closed archives stop live connections |
| Foreground location sharing | Complete; owner-reported outdoor validation | Android permission handling, Start/Stop sharing, accuracy and timestamp submissions, rejection feedback, and physical-phone verification with the app open |
| Screen-off tracking | Complete; owner-reported outdoor validation | Android background tracking and notification; physical-phone verification of screen locking, network recovery, stop/restart, and battery behavior |
| Android route creation | Complete; emulator validated | Create a route using the shared product/API contract, retain owner/member access securely, and open its route screen |
| Android owner controls | Next | Owner-only editing, closing, and deletion with shared confirmation rules and correct archive/deleted-access handling |

Live viewing validation on 2026-10-06:

- Mobile TypeScript checking and all 36 tests passed through Docker. Eleven live-viewing tests cover token authentication, position validation, segment separation, sequence ordering/deduplication, snapshot/event races, membership resync, reconnection, obsolete sockets, foreground recovery, archive closure, failures, duplicate connections, and authentication timeout.
- The actual mobile session ran against the local API with a disposable route and two real WebSocket clients. Location/status updates, join/leave events, metadata edits, foreground catch-up, and socket reconnection with missed positions passed.
- The Android 17/API 37 emulator displayed Live updates connected and rendered real accepted locations and paths without Refresh. Route edits appeared automatically. Returning from the background recovered an edit made while disconnected. A manually panned map retained Fit group after resync. Closing the route displayed its archive and stopped live viewing.
- The disposable route was deleted; Android's invalid-access flow cleared its saved membership. The original saved route was restored. Metro and the emulator remain available for review.
- This task adds JavaScript only and needs no APK rebuild. It does not capture device location or establish background tracking reliability.

Foreground sharing validation on 2026-10-06:

- TypeScript checking and all 53 mobile tests passed through Docker. New coverage includes payload metadata, permission denial, occupied slots, command IDs and acknowledgement/rejection/timeout, Stop cleanup, GPS feedback, network interruption, background recovery, delayed native subscriptions, permission-dialog lifecycle, and cancellation of initial GPS requests.
- Installed Expo Location and upgraded Expo to `57.0.27` to satisfy the current Expo dependency check. Compatibility checking passed. Generated and built the native Android APK successfully, installed it on the emulator, and verified that its generated manifest contains coarse/fine location permissions without background location or foreground-service permissions.
- Android runtime permission denial displayed retry guidance. Granting foreground precise access started sharing. Native emulator GPS samples reached the API with accuracy and client timestamps. Backgrounding retained the accepted-point count despite new emulator fixes and marked the member stale. Returning showed the recovery choices. Resume restored tracking; Stop changed the member to spectating and additional fixes produced no new accepted points.
- The actual mobile sharing and viewing controllers ran against the real backend with a disposable membership. An inaccurate sample produced rejection feedback; a later valid sample cleared it. Confirmed Stop released tracking and removed capture.
- Physical hardware was not connected. Outdoor precise/approximate permission behavior, movement, network recovery, and Stop remain required before marking foreground sharing fully validated. Screen-off tracking was still unimplemented at this checkpoint; see the following validation record.

Screen-off sharing validation on 2026-10-07:

- TypeScript checking, Expo dependency compatibility, and all 68 mobile tests passed through Docker. Added coverage includes screen-lock connection retention, native task ownership and serialized cleanup, delayed/offline samples, stopped/invalid/archived sessions, and recovery when JavaScript timers or socket close events do not fire.
- Regenerated Android and successfully built and installed the APK with Expo TaskManager. Native verification exposed the job scheduler's `RECEIVE_BOOT_COMPLETED` requirement; app configuration now includes it alongside foreground-service/location and notification permissions. Background location permission remains disabled.
- Notification denial prevented sharing and displayed settings guidance. With notification permission, Android reported an active location foreground service with an ongoing notification. A fresh app runtime delivered new accepted points while the device reported `Asleep`.
- Emulator recovery exposed obsolete API presence timers. WebSocket regression tests reproduced a recovered stale member being marked offline. The fix cancels recovered-session timers and prior disconnect timers; the full API race/PostGIS suite, Go vet, and golangci-lint passed.
- With the screen asleep, a 50-second emulator network outage left accepted history unchanged at 55 points and marked the member stale. Restoring connectivity recovered tracking and accepted new points without waking the activity. Tapping the ongoing notification returned to the route controls. Stop returned the member to spectating, removed the foreground notification, and left the accepted-point count at 64 after another locked-screen fix. Restart accepted new points. Closing the route while locked removed the service and notification; returning displayed its archive.
- Deleted the disposable route and cleared its Android saved access through the invalid-membership flow. Restored the original saved route. Metro, the emulator, and the backend remain available.
- Physical hardware was not connected. Outdoor GPS reliability, longer screen-lock periods, manufacturer battery restrictions, and measured battery use remain pending. Follow the [screen-off review](../workflow/mobile-development.md#screen-off-sharing-review) and record the device and observations before closing this checkpoint.

Physical-phone development-build validation reported by the owner on 2026-10-07:

- The owner confirmed that everything works on the Samsung phone connected by USB after installing the development build and configuring the reachable computer API address.
- The phone was described as a Galaxy S21; ADB reports model `SM_G996B`, the Galaxy S21+. The emulator-only `x86_64` APK could not install; rebuilding for the selected physical device resolved the architecture mismatch. Expo device selection uses the device name or the interactive `--device` picker, rather than the ADB serial.
- This records the owner's successful connected development test. No individual outdoor, screen-lock duration, network-recovery, or battery measurements were reported, and the Android version was not recorded.
- Next, use a standalone production/release ARM build with the production HTTPS API. Perform the [foreground review](../workflow/mobile-development.md#foreground-sharing-review) and [screen-off review](../workflow/mobile-development.md#screen-off-sharing-review) outdoors without USB, Metro, or a debugger. Record device/Android version, permissions, battery restrictions, duration, battery change, and any update gaps before closing the tracking checkpoints.

Standalone Android release build prepared on 2026-10-07:

- Docker Gradle `:app:assembleRelease -PreactNativeArchitectures=arm64-v8a` succeeded. The APK is `apps/mobile/android/app/build/outputs/apk/release/app-release.apk`, approximately 37.7 MiB, with bundled JavaScript and `https://keepup.kesopeso.eu/api` verified in the bundle. The production health endpoint returned `status: "ok"`.
- APK signature verification and mobile TypeScript checking passed. Native libraries contain only `arm64-v8a`; package/version are `eu.kesopeso.keepup`, `1.0.0`, version code 1. SHA-256 is `005c0d26d8fccd21aab016cedc9ac352aa88381b40eaa4237d31f2544cbe2186`.
- The generated release configuration uses the development signing key. This standalone release build is for the owner's outdoor validation; dedicated release signing remains required for public distribution. See the [build and installation workflow](../workflow/mobile-development.md#standalone-android-release-build-for-outdoor-testing).
- Rebuilt successfully after applying the KeepUp display name and K4-10 launcher/fallback/themed icons. APK metadata confirms the KeepUp label; generated adaptive icon resources use the branded foreground and `#173023` background. Signature, embedded production URL, and ARM64 libraries were reverified.
- Added the Expo description and `expo-splash-screen` launch screen using the KeepUp logo on `#173023` in light/dark modes. Expo compatibility and TypeScript checks passed; native regeneration and the ARM64 release rebuild succeeded. Generated Android splash theme, logo, and both background color resources were inspected; APK signature and production API URL were reverified.
- Outdoor testing was pending at this build checkpoint; see the subsequent owner-reported result below. Launcher and launch-screen appearance were not separately reported.

Physical-phone outdoor validation reported by the owner on 2026-10-07:

- The owner reported that outdoor testing was successful after preparation of the standalone production-connected release APK. This completes the planned physical-review checkpoints for foreground and screen-off tracking on the owner's report.
- The test used the owner's Samsung phone from the preceding installation workflow. Android version, elapsed screen-lock duration, individual network-recovery/Stop/restart results, battery restrictions, and measured battery change were not supplied; no per-scenario measurements are inferred from the overall success report.
- The owner selected Android route creation as the next implementation task, followed by owner controls to edit, close, and delete routes. These remain planned, not implemented.

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
| Android screen-off tracking and ongoing notification | [Screen-off tracking](../system/mobile.md#screen-off-tracking) |
| Android foreground permission, Start/Stop sharing, native GPS metadata, and stale recovery | [Foreground sharing](../system/mobile.md#foreground-location-sharing) |
| Android live viewing, automatic positions and lifecycle updates, reconnect and foreground catch-up | [Mobile live viewing](../system/mobile.md#live-viewing) |
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
