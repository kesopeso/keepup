---
type: record
status: active
historical: true
---

# Validation history

Load only for dated evidence or previous review limits. These records preserve observations at the time of each check. Later records can supersede earlier pending tasks; use [current status](status.md) for today's progress and owning system references for current behavior. Test runs listed here were recorded previously, not rerun by the documentation audit.

## Web My routes W2 on 2026-10-22

- Checks ran on 2026-10-21 and 2026-10-22. Docker web tests passed, 28 tests, including additive filter encoding, cursor reset, deduplication, account-isolated retention/logout cleanup, shared account/route rotation serialization, pre-socket one-use ticket acquisition, reconnect cancellation, and native/cancel/copy/manual share fixtures. The production Next.js build and TypeScript check passed. The final application graph refreshed and verified; changed documentation links and whitespace checks passed.
- Dockerized Chromium exercised the production build against a real account-enabled API and isolated `w2_check` PostgreSQL schema migrated through 0007. Synthetic accounts were activated directly in that schema to avoid external mail. Real checks covered protected authentication return, immutable username create/join, wrong-password denial, restored current membership, explicit non-owner leave, all eight Active filter combinations with owned/joined/left fixtures, 20-row cursor pagination through 23 routes, archive transition, protected former-member archive rejoin, read-only archive history, and full server counts. Back navigation restored 23 rows and a 2200px list position, then revalidated loaded pages. Fresh navigation used default filters.
- Two independent browser sessions viewed the same membership concurrently. Explicit Start acquired one sender lease, eight synthetic browser GPS samples reached stored history, the other viewer showed the other-device restriction, and Stop released the lease. Real logout-all while sending removed the map and stopped the browser GPS watcher within the observed 500ms check. Dashboard authority polling also redirected after server revocation. UI logout erased retained summaries/return state; a same-browser account switch showed fresh filters and no previous account summaries.
- Browser-local network fixtures exercised offline cached summaries and failed-load retry. Offline route opening, create and join were disabled while Share stayed available. Clipboard success and blocked-clipboard manual-copy feedback used local browser stubs. Unit fixtures exercised native sharing and silent native cancellation; an operating-system share sheet was not tested. The common helper shares only a route URL. No external Google sign-in, email delivery, physical GPS or Android interoperability was exercised; those remain release acceptance work.
- Dashboard accessibility audits reported zero violations. Desktop and 390px phone layouts were reviewed, with no horizontal phone overflow. Final My routes metadata reported `no-referrer` and `noindex, nofollow`. Disposable browsers/services/schema were removed after acceptance; application tables were not migrated/reset and no deployment or feature-flag default changed. Current ownership is in [web boundaries](../system/frontend.md#my-routes-and-account-route-authority).

## Android authentication A1 on 2026-10-22

- Work began on 2026-10-21 in the separate `issue-a1` worktree. Docker mobile TypeScript passed, and the complete mobile regression suite passed 98 tests. The A1 tests cover native headers/cookie exclusion, timeout/body consumption, rate limits, link/destination allowlists, backend-scoped storage/expiry, pending restoration, verification elsewhere, serialized rotation, secure-write/deletion/read recovery, polling during unsaved rotation, nonblocking routine refresh, revocation, Google context restoration/exchange/onboarding and ignored late picker callbacks.
- The Android x86_64 development APK compiled with the local Expo Credential Manager module and installed without clearing existing app data. An explicit `keepup` scheme preserves the Expo development-client scheme alongside verified HTTPS filters. Prebuild and Gradle must run sequentially. An initial concurrent prebuild/build lost generated autolinking input; the subsequent sequential builds passed.
- The API on port 14000 used only disposable schema `a1_check`, manually migrated through 0007. Native transport/controller integration exercised real B2–B4 signup/pending restoration, case-insensitive login, generic recovery, logout, Google nonce start and invalid proof, a prepared post-proof ready context, explicit exchange/onboarding completion, password change and old-token rejection. No external email or Google sign-in occurred.
- The Android 17/API 37 emulator exercised real signup, masked fields and on-screen keyboard entry, focused confirmation-mismatch feedback, pending SecureStore restoration after force-stop and APK update, a retained route destination and unavailable-route fallback after activation. Another native client confirmed a prepared verification token without receiving a session; the existing emulator session observed activation automatically.
- Emulator settings/password change retained typed fields through repeated five-second authority reads. A separate native session received 401 after the password change. Active SecureStore access restored after force-stop, while reset fields/link tokens did not restore. Explicit native reset submission distinguished invalid and expired replacement links, then a valid prepared reset returned to login without activation/sign-in. The reset warning appeared directly below the title. The reset password then authenticated with a differently cased username. Server expiry returned the app to login. All-device logout and process restart retained no account access.
- The native Google system picker was cancelled with Android Back; KeepUp displayed cancellation and discarded its flow. No Google account was selected. Unit/API fixture checks cover proof-following contexts, while actual provider proof, Google configuration/signing acceptance and real email delivery remain R1.
- App-link checks explicitly targeted the Android component with a disposable approved HTTPS host. They establish native handling and same-app replacement, not production domain association. The module clears the in-process Activity Intent; Android/browser launch records remain platform-owned. Production `assetlinks.json`, release-signing dispatch, physical-phone password-manager fill and device lifecycle review remain R1. Autofill hints are set; an actual password-manager fill was not claimed.
- Login, signup, pending, account and reset screenshots were reviewed without exposed passwords, tokens or proofs. A1 leaves native My routes and route/live authority to A2. Authentication remains disabled by default, and no application tables were migrated/reset or deployment performed.

## Web authentication W1 on 2026-10-21

- Docker web regression tests passed, 22 tests, including browser transport/CSRF headers, Unicode password boundaries, destination allowlisting/expiry, access rechecks, serialized cookie rotation, Google SDK load retry, stale-response fencing, hidden-tab revocation and offline session behavior. The production Next.js build and its TypeScript check passed.
- Dockerized Chromium exercised the production web build against a separate account-enabled API and a disposable PostgreSQL schema migrated through 0007. Real API checks covered signup confirmation/focus, pending signup and reload persistence, mixed-case username login, verification in another browser without signing it in, destination retention and unavailable-route fallback, password change retaining the current session while revoking another, generic recovery, reset revocation, consumed/expired links including replacement links opened in the same tab, Google context restoration/onboarding and username collisions, and cross-tab logout everywhere. Application tables were not migrated or reset.
- Google onboarding and third-party email challenges used prepared post-proof database fixtures, then real B4 context, completion, confirmation and exchange endpoints. A local Identity Services stub verified nonce binding, popup-button initialization, cancellation and ignored late callbacks. Offline and Retry-After UI checks used browser-local request stubs. These checks did not authenticate with Google or send external email. SMTP was configured only against an unreachable loopback endpoint in the disposable API.
- Accessibility audits reported zero violations on login, verification, reset, account settings and Google onboarding. Account settings and phone forms fit a 390px viewport without horizontal overflow; inputs expose password-manager autocomplete attributes and confirmation errors focus the relevant field.
- With the owner's explicit approval to save disposable session cookies, Chromium was closed and relaunched from a temporary browser-state file. A pending session returned `200` with the same username and retained `/account` destination. After explicit email verification, a second close/relaunch restored the verified account settings screen with empty password fields. After logout, replaying the previously saved state returned `401` and redirected to sign-in. The state file remained inside the disposable browser container, which was removed after the check; the separate test API, web server, proxy and schema were also removed. No application tables or external mail were used. Real Google, external mail delivery, Android interoperability and full route/My routes integration remain later acceptance work. Current source ownership is in [frontend boundaries](../system/frontend.md#account-authentication).

## Docker browser automation validation

Previously recorded checks, with the original test date unrecorded:

- The image build and browser launch were checked. Separate launcher calls opened a test page, returned its interactive button, and retained its title. A local KeepUp route was joined and its member list read through the Dockerized browser.
- These checks establish the launcher and basic automation flow, not full web application coverage. They were not rerun when this evidence was moved here.

Sources: [launcher](../../bin/agent-browser.sh), [image](../../Dockerfile.agent-browser). Current usage instructions belong to the custom [agent-browser skill](../../.agents/skills/agent-browser/SKILL.md).

## Mobile validation

### Android build workflow, 2026-10-08

Added `make mobile-build` on 2026-10-08 to regenerate, build, and install the Android development APK before starting Metro. The Metro launcher now checks for a single ready device, an installed development client, and support for the current development scheme. Shell syntax, the Make command sequence, and seven simulated ADB outcomes passed validation; the installed emulator app passed the live compatibility check. A complete rebuild was not run for this tooling change. See [daily development](../workflow/mobile-development.md#daily-development).

### Shared mobile branding, 2026-10-08

On 2026-10-08, the owner approved the mobile logo preview in Lavish. Implemented the shared logo header and lowercase white/green wordmark across startup, joining, route viewing, archives, and connection checks, plus the transport dropdown. Docker TypeScript checking and all 68 mobile tests passed. Device review of these UI changes remains unverified; no native dependency or APK rebuild is required for a development build. See [mobile boundaries](../system/mobile.md#current-source-ownership).

### Validation on 2026-10-05

- Inspected the Android build output: Gradle reported `BUILD SUCCESSFUL in 2m 35s`, and Expo installed and opened the development APK on the Android 17/API 37 emulator.
- The owner confirmed the starter screen appeared and later confirmed browser React Native DevTools worked after the hostname and headless workflow corrections.
- ADB forwarding and the browser debugger endpoint were checked during diagnosis. These checks establish the development setup, not KeepUp API connectivity or native tracking reliability.

### API connection validation on 2026-10-05

- Mobile TypeScript checking and all eight request/configuration tests passed through Docker Compose. Tests cover healthy and degraded responses, incorrect successful responses, network recovery, request/body timeout, cancellation, and release configuration.
- The Android 17/API 37 emulator displayed Connected to KeepUp against the existing proxy/API/PostGIS stack. Stopping the API produced the unavailable state and Retry. Restarting the API and tapping Retry returned to connected. The Android accessibility hierarchy confirmed the checking button was disabled while the request ran.
- The temporary Metro session was stopped after verification and the backend was restored. The screen needs no native dependency or APK rebuild; start Metro with the existing launcher.

### Android joining validation on 2026-10-05

- Mobile TypeScript checking and all 20 tests passed through Docker Compose. The additional tests cover code/link parsing, HTTP error distinctions, authenticated snapshots, persistent identity, backend isolation, rejected joins, invalid-token cleanup, and retry after failed snapshot/storage operations without duplicate membership.
- Regenerated the Android project with the SecureStore plugin, successfully built the development APK with Expo Crypto and SecureStore, and installed it on the running Android 17/API 37 emulator.
- Emulator checks passed for invalid route codes, a pasted link to a disposable password-protected route, incorrect password, duplicate display name, a successful two-member snapshot, and force-stop/reopen with the same membership. Failed refresh retained the dated snapshot and offered Retry. The API was restored after the outage check.
- The actual mobile API/session code loaded the disposable route's closed archive and resumed saved access without another join. Deleting that disposable route produced invalid access and cleared its test session's membership and last-route credentials. This archive/deletion check used a separate test repository; it did not alter the owner's Android storage. The emulator remained available for the owner's concurrent testing.
- Metro is running for review. Other installations need a rebuilt APK for the added native modules; see the [Android workflow](../workflow/mobile-development.md#route-joining-and-saved-access).

### Android snapshot map validation on 2026-10-05

- Mobile TypeScript checking and all 25 tests passed through Docker Compose. Map tests cover separate path segments, latest timestamps, member colors/status, empty and single-point geometry, invalid samples, antimeridian paths/bounds, and malformed snapshot rejection.
- Regenerated the Android project, built the MapLibre development APK successfully, and installed it on the Android 17/API 37 emulator. The existing saved route displayed the native basemap and empty-location message.
- A temporary in-memory fixture verified colored paths/markers, visible segment gaps, manual pan, refresh without recentering, Fit group becoming hidden and inaccessible after fitting, member focus with written status/timestamp, and an empty archive. The fixture was removed and the normal app entry restored without backend or membership changes.
- An unavailable tile endpoint exposed MapLibre's separate native tile-error reporting. The map now turns those errors into readable feedback and Retry map. Restoring tile access and retrying recovered the basemap. The real provider was restored after the check.
- Documentation links and heading fragments resolve. Metro remains running for review. Native GPS sharing and live updates are still pending; this establishes snapshot rendering, not native tracking reliability.

### Android route creation validation on 2026-10-08

- Added the native creation form with required route/display names, saved transport selection, optional description/password, and both sharing policies. Successful creation saves member and owner access in backend-scoped SecureStore and opens the native route screen without starting GPS.
- Docker TypeScript checking and all 79 mobile tests passed. Eleven creation tests cover validation, response rejection, password handling, backend scope, restored owner access, concurrent submissions, credential/last-route write failures, cancellation, request recovery, snapshot failures, and invalid-access cleanup.
- The Android 17/API 37 emulator displayed required-name feedback and created a password-protected, owner-only-sharing route with its description and saved Bicycle selection. A temporary runtime SecureStore failure after the response showed the created code and disabled navigation/inputs. Retry saved access and opened the route; the observed create POST count remained one.
- Force-stop/reopen restored the protected route without a password prompt and displayed the same membership as You and Owner, spectating. Deleted the disposable routes with their returned owner credentials, verified deleted-access cleanup, and restored the emulator's original saved route. Runtime fault injection and temporary cleanup files were removed.
- This change uses existing native modules and needs no development APK rebuild. Physical-device review of the creation form remains unverified. The existing create endpoint has no idempotency support; lost responses and app termination before recovering failed credential storage remain access-recovery limits.



### Live viewing validation on 2026-10-06

- Mobile TypeScript checking and all 36 tests passed through Docker. Eleven live-viewing tests cover token authentication, position validation, segment separation, sequence ordering/deduplication, snapshot/event races, membership resync, reconnection, obsolete sockets, foreground recovery, archive closure, failures, duplicate connections, and authentication timeout.
- The actual mobile session ran against the local API with a disposable route and two real WebSocket clients. Location/status updates, join/leave events, metadata edits, foreground catch-up, and socket reconnection with missed positions passed.
- The Android 17/API 37 emulator displayed Live updates connected and rendered real accepted locations and paths without Refresh. Route edits appeared automatically. Returning from the background recovered an edit made while disconnected. A manually panned map retained Fit group after resync. Closing the route displayed its archive and stopped live viewing.
- The disposable route was deleted; Android's invalid-access flow cleared its saved membership. The original saved route was restored. Metro and the emulator remain available for review.
- This task adds JavaScript only and needs no APK rebuild. It does not capture device location or establish background tracking reliability.

### Foreground sharing validation on 2026-10-06

- TypeScript checking and all 53 mobile tests passed through Docker. New coverage includes payload metadata, permission denial, occupied slots, command IDs and acknowledgement/rejection/timeout, Stop cleanup, GPS feedback, network interruption, background recovery, delayed native subscriptions, permission-dialog lifecycle, and cancellation of initial GPS requests.
- Installed Expo Location and upgraded Expo to `57.0.27` to satisfy the current Expo dependency check. Compatibility checking passed. Generated and built the native Android APK successfully, installed it on the emulator, and verified that its generated manifest contains coarse/fine location permissions without background location or foreground-service permissions.
- Android runtime permission denial displayed retry guidance. Granting foreground precise access started sharing. Native emulator GPS samples reached the API with accuracy and client timestamps. Backgrounding retained the accepted-point count despite new emulator fixes and marked the member stale. Returning showed the recovery choices. Resume restored tracking; Stop changed the member to spectating and additional fixes produced no new accepted points.
- The actual mobile sharing and viewing controllers ran against the real backend with a disposable membership. An inaccurate sample produced rejection feedback; a later valid sample cleared it. Confirmed Stop released tracking and removed capture.
- Physical hardware was not connected. Outdoor precise/approximate permission behavior, movement, network recovery, and Stop remain required before marking foreground sharing fully validated. Screen-off tracking was still unimplemented at this checkpoint; see the following validation record.

### Screen-off sharing validation on 2026-10-07

- TypeScript checking, Expo dependency compatibility, and all 68 mobile tests passed through Docker. Added coverage includes screen-lock connection retention, native task ownership and serialized cleanup, delayed/offline samples, stopped/invalid/archived sessions, and recovery when JavaScript timers or socket close events do not fire.
- Regenerated Android and successfully built and installed the APK with Expo TaskManager. Native verification exposed the job scheduler's `RECEIVE_BOOT_COMPLETED` requirement; app configuration now includes it alongside foreground-service/location and notification permissions. Background location permission remains disabled.
- Notification denial prevented sharing and displayed settings guidance. With notification permission, Android reported an active location foreground service with an ongoing notification. A fresh app runtime delivered new accepted points while the device reported `Asleep`.
- Emulator recovery exposed obsolete API presence timers. WebSocket regression tests reproduced a recovered stale member being marked offline. The fix cancels recovered-session timers and prior disconnect timers; the full API race/PostGIS suite, Go vet, and golangci-lint passed.
- With the screen asleep, a 50-second emulator network outage left accepted history unchanged at 55 points and marked the member stale. Restoring connectivity recovered tracking and accepted new points without waking the activity. Tapping the ongoing notification returned to the route controls. Stop returned the member to spectating, removed the foreground notification, and left the accepted-point count at 64 after another locked-screen fix. Restart accepted new points. Closing the route while locked removed the service and notification; returning displayed its archive.
- Deleted the disposable route and cleared its Android saved access through the invalid-membership flow. Restored the original saved route. Metro, the emulator, and the backend remain available.
- Physical hardware was not connected. Outdoor GPS reliability, longer screen-lock periods, manufacturer battery restrictions, and measured battery use remain pending. Follow the [screen-off review](../workflow/mobile-development.md#screen-off-sharing-review) and record the device and observations before closing this checkpoint.

### Physical-phone development-build validation reported by the owner on 2026-10-07

- The owner confirmed that everything works on the Samsung phone connected by USB after installing the development build and configuring the reachable computer API address.
- The phone was described as a Galaxy S21; ADB reports model `SM_G996B`, the Galaxy S21+. The emulator-only `x86_64` APK could not install; rebuilding for the selected physical device resolved the architecture mismatch. Expo device selection uses the device name or the interactive `--device` picker, rather than the ADB serial.
- This records the owner's successful connected development test. No individual outdoor, screen-lock duration, network-recovery, or battery measurements were reported, and the Android version was not recorded.
- Next, use a standalone production/release ARM build with the production HTTPS API. Perform the [foreground review](../workflow/mobile-development.md#foreground-sharing-review) and [screen-off review](../workflow/mobile-development.md#screen-off-sharing-review) outdoors without USB, Metro, or a debugger. Record device/Android version, permissions, battery restrictions, duration, battery change, and any update gaps before closing the tracking checkpoints.

### Standalone Android release build prepared on 2026-10-07

- Docker Gradle `:app:assembleRelease -PreactNativeArchitectures=arm64-v8a` succeeded. The APK is `apps/mobile/android/app/build/outputs/apk/release/app-release.apk`, approximately 37.7 MiB, with bundled JavaScript and `https://keepup.kesopeso.eu/api` verified in the bundle. The production health endpoint returned `status: "ok"`.
- APK signature verification and mobile TypeScript checking passed. Native libraries contain only `arm64-v8a`; package/version are `eu.kesopeso.keepup`, `1.0.0`, version code 1. SHA-256 is `005c0d26d8fccd21aab016cedc9ac352aa88381b40eaa4237d31f2544cbe2186`.
- The generated release configuration uses the development signing key. This standalone release build is for the owner's outdoor validation; dedicated release signing remains required for public distribution. See the [build and installation workflow](../workflow/mobile-development.md#standalone-android-release-build-for-outdoor-testing).
- Rebuilt successfully after applying the KeepUp display name and K4-10 launcher/fallback/themed icons. APK metadata confirms the KeepUp label; generated adaptive icon resources use the branded foreground and `#173023` background. Signature, embedded production URL, and ARM64 libraries were reverified.
- Added the Expo description and `expo-splash-screen` launch screen using the KeepUp logo on `#173023` in light/dark modes. Expo compatibility and TypeScript checks passed; native regeneration and the ARM64 release rebuild succeeded. Generated Android splash theme, logo, and both background color resources were inspected; APK signature and production API URL were reverified.
- Outdoor testing was pending at this build checkpoint; see the subsequent owner-reported result below. Launcher and launch-screen appearance were not separately reported.

### Physical-phone outdoor validation reported by the owner on 2026-10-07

- The owner reported that outdoor testing was successful after preparation of the standalone production-connected release APK. This completes the planned physical-review checkpoints for foreground and screen-off tracking on the owner's report.
- The test used the owner's Samsung phone from the preceding installation workflow. Android version, elapsed screen-lock duration, individual network-recovery/Stop/restart results, battery restrictions, and measured battery change were not supplied; no per-scenario measurements are inferred from the overall success report.
- The owner selected Android route creation as the next implementation task, followed by owner controls to edit, close, and delete routes. These remain planned, not implemented.

### Android route map layout implemented on 2026-10-08

- The selected Lavish option A is implemented with a full-screen map and a drawer that rests only at Peek or Full. Layout and camera behavior are owned by [mobile map boundaries](../system/mobile.md#native-snapshot-map).
- TypeScript and the existing 79 mobile regression tests passed through Docker. Android emulator review covered vertical map pan, zoom buttons, handle dragging and snapping, independent member scrolling, member focus returning to Peek, and manual-camera preservation after a position fixture. Permission/settings errors, recovery choices, reconnection with Stop available, and archives were reviewed with in-memory fixtures without changing backend data. Enlarged system text was reviewed at 1.5 times the default size. Automated synthetic pinch and double-tap input did not establish zoom behavior, but the owner confirmed pinch zoom works on the emulator on 2026-10-08. The zoom buttons were removed in favor of two-finger pinch. Physical-phone gesture verification remains pending.

## UI redesign

The user selected the full layout refresh in an earlier Lavish visual proposal, then requested implementation. The app now uses simpler create/join forms, a map-led route layout, sharing controls above collapsible members, explicit presence badges, and a route-details dialog. Behavior is owned by [map experience](../product/experience.md); implementation boundaries by [frontend](../system/frontend.md).

Validation on 2026-09-26:

- Production web build and all four existing live-state tests passed through Docker Compose.
- Chromium checks passed for create, password errors and protected join, live member insertion, start/stop sharing, clipboard sharing, duplicate connections, close/archive transitions, and code-confirmed deletion using disposable local routes.
- Isolated browser fixtures covered all five presence states, long names, restricted sharing, full tracking slots, stale recovery, member collapse, map resizing, and manual viewport preservation.
- Layout checks passed at 360px, 390px, 640px, 844px landscape, and 1280px. Automated accessibility checks found no violations on create, live route, or the populated member sheet. Nested dialogs restore focus and close confirmation initially focuses Cancel.

Validation reported by the owner on 2026-09-30:

- Physical-phone outdoor testing passed for map readability, location permissions, and native sharing.

## Web HTTP sharing validation

Validated on 2026-10-09 after adding the [browser ID helper](../../apps/web/lib/browser-id.ts) for browser identity and sharing-command request IDs:

- Docker production web build, including TypeScript checking, and all 10 web tests passed. Regression tests cover missing `randomUUID`, distinct UUID v4 request IDs, saved identity preservation, and the existing fallback when Web Crypto is absent.
- Chrome used an isolated test context on an HTTP IP origin. Confirmed `isSecureContext` was false, `crypto.randomUUID` was undefined, and `getRandomValues` was available. Restarted the local web service to load the existing development-origin setting before testing.
- Created a disposable route through the web UI. Start and Stop sharing generated distinct request IDs, received matching server acknowledgements, and changed the UI and authenticated snapshot between tracking and spectating. The API stored 21 simulated positions; the count remained 21 after Stop. No browser console or captured runtime errors occurred during the completed check.
- Deleted the disposable route, confirmed its access endpoint returned 404, cleared its isolated saved route access, and closed the test tab. Existing browser memberships were not used. This verifies development sharing with simulated GPS, not real browser geolocation over HTTP.

## Web development clipboard fallback validation

Validated on 2026-10-12 for the [development-only Share fallback](../system/frontend.md#current-frontend-foundation):

- All 10 web tests and the Docker production build, including TypeScript checking, passed. The production client output contains no legacy copy command or unsafe-fallback notice.
- On an HTTP IP origin in desktop Chrome, clicked Share and pasted the copied text to confirm the exact route URL. Verified the amber warning, manual-copy behavior when legacy copy reports failure, and silent native-share cancellation without attempting copy. A simulated native-share rejection also reached the successful copy fallback. Removed the temporary test input and deleted the disposable route.
- The owner tested the fallback on mobile Chrome and confirmed it works.

## GPS validation

Validation on 2026-10-03:

- Full API tests passed through Docker Compose with the race detector and PostGIS integration tests enabled.
- Isolated database schemas covered concurrent duplicate submissions, rejected-point side effects, persisted baselines across service instances, geographic antimeridian movement, segment resets, and concurrent stop/leave/close operations.
- WebSocket tests covered each GPS rejection code, malformed timestamps, continued sharing after rejection, and stale timers unaffected by rejected samples.
- Go vet and golangci-lint passed. No new database migration is required.

## Android account routes A2 on 2026-10-22

A2 implements native My routes and account route/live integration. Checks used Docker mobile helpers and a separate `keepup-a2` API/PostGIS stack with an in-memory disposable database through migration 0007. Synthetic accounts were activated directly in that database. Mail pointed to a non-delivering local endpoint; no real Google proof, external email, application-data reset or deployment was involved.

- Docker TypeScript and 111 mobile tests passed. Added checks cover serialized route requests after credential rotation, native PATCH/DELETE transport, wrong-password versus expired-account handling, all filter combinations, cursor/page deduplication, public share links/cancellation/fallback, ticket-before-socket ordering, connection capabilities, acknowledged sender state, generation attachment/discard, obsolete sender cleanup and cancellation of late ticket responses. Explicit cancellation/timeout races also cover native fetch/body implementations that ignore abort, preventing cancelled route loads from blocking later queued requests. Existing legacy route, map, storage, foreground and screen-off tests still pass.
- Real B5/B6 integration through the native controller/transport and viewing/sharing controllers covered 20-to-23 pagination, full counts and all eight category combinations. Wrong-password joins failed without discarding the account. Current membership restoration retained its episode; protected former-member archive rejoin required the password, created a new episode and remained read-only.
- Independent real native sessions watched the same account membership concurrently. Exactly one sender held that route; another session could send on a separate route. Synthetic GPS was accepted. Obsolete-generation packets were rejected. Remote logout-all invalidated both senders, removed their simulated native watchers, cleared account authority and prevented further positions. Screen-off controller continuity and generation disposal/reconnect behavior also have deterministic tests.
- The Android 17/API 37 emulator loaded A2 JavaScript through Metro in the existing A1 development APK. No native dependency changed and no new APK was built for A2. Native login opened My routes; opening a route connected through Android's real WebSocket implementation as a viewer with no foreground location service. The adapter suppresses React Native's automatic Origin header, which the native account transport rejects.
- Explicit Start created the Android foreground location service and accepted synthetic emulator GPS. With the screen asleep, new synthetic positions were accepted. A real password reset using a purpose-token fixture in the disposable database released the sender lease, stopped foreground location sharing and returned the app to sign-in without protected route history. The reset link fixture tested the real reset/revocation path, not email delivery.
- Native Load more loaded all 23 routes. Opening a route and pressing Android Back restored the same visible final rows. Native Share opened the Android share sheet with only the configured HTTPS route URL; dismissal returned without an error. During a real disposable-API outage, retained rows disabled Open while Share remained enabled and opened the same system sheet with a public link. Shared link generation covers active, archived, former-member and cached rows in tests; an OS share failure uses the selectable-link fallback.
- Restoring the disposable API recovered list loading. Signing out and switching to the other synthetic account cleared the previous account's loaded pages/scroll and restored default filters; that account saw its own empty Active list. Native creation opened the new owner route as a viewer; confirming Close returned to a revalidated list and the archive opened read-only. Confirming Delete removed that archive and returned to the list.

The native list layout was inspected on the emulator. Real Google/email, verified production app-link dispatch, password-manager fill, larger-text and physical-device account-mode power/network behavior remain R1. Earlier outdoor legacy tracking evidence does not establish account-mode device reliability. Application graph freshness, local documentation links and whitespace checks accompany this change.
