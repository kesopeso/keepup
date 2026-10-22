---
type: reference
status: active
---

# Mobile client boundaries

## Purpose

Describe the mobile client's current architecture, source ownership, and relationship to the shared backend. Commands and troubleshooting belong to the [Android workflow](../workflow/mobile-development.md); completed work and proposed next tasks belong to [delivery status](../planning/status.md#mobile-foundation).

## Owning facts

### Client approach

The mobile app uses React Native, Expo, and TypeScript in `apps/mobile`. Android is the first platform being developed and tested; iOS is intended later. An installed development build includes `expo-dev-client`, so native modules and configuration can be tested on the emulator and physical devices.

This is a separate mobile UI. The Next.js web screens are not automatically reused as native screens. The shared product rules and API contract are the integration boundary; no shared TypeScript application package has been extracted yet.

### Current source ownership

| Source | Responsibility |
|---|---|
| [App.tsx](../../apps/mobile/App.tsx), [mobile app](../../apps/mobile/src/MobileApp.tsx) | Restore saved access and route between creation, joining, snapshot, and connection-check screens |
| [Create screen](../../apps/mobile/src/screens/CreateRouteScreen.tsx), [creation session](../../apps/mobile/src/domain/create-session.ts), [creation tests](../../apps/mobile/tests/create.test.cjs) | Route form, create request, secure owner/member persistence, and saving retry without repeated creation |
| [Join screen](../../apps/mobile/src/screens/JoinRouteScreen.tsx) | Route code/link entry, access metadata, display name, transport mode, and conditional password |
| [Snapshot screen](../../apps/mobile/src/screens/RouteSnapshotScreen.tsx), [route drawer](../../apps/mobile/src/components/RouteDrawer.tsx) | Full-screen map layout, Peek/Full drawer, authenticated route details and members, sharing actions, refresh/retry, and archives |
| [Native map](../../apps/mobile/src/components/RouteMap.tsx), [snapshot geometry](../../apps/mobile/src/map/snapshot-geometry.ts), [tile provider](../../apps/mobile/src/map/tile-provider.ts) | Native MapLibre rendering, member focus, camera fitting, and basemap configuration |
| [Screen-off task](../../apps/mobile/src/location/screen-off-location.ts), [task lifecycle](../../apps/mobile/src/location/location-task.ts), [task tests](../../apps/mobile/tests/location-task.test.cjs) | Android foreground service, notification, task delivery, and serialized native cleanup |
| [Foreground sharing](../../apps/mobile/src/location/foreground-sharing.ts), [native location](../../apps/mobile/src/location/native-location.ts), [sharing tests](../../apps/mobile/tests/sharing.test.cjs) | Explicit sharing intent, permissions, cancellable GPS capture, command confirmation, and rejection recovery |
| [Viewing session](../../apps/mobile/src/live/viewing-session.ts), [live tests](../../apps/mobile/tests/viewing.test.cjs) | Authenticated sockets, automatic positions, lifecycle resync, reconnect, and foreground recovery |
| [UI components](../../apps/mobile/src/components/ui.tsx) | Scrollable keyboard-aware screens, labeled inputs, buttons, and error feedback |
| [Brand components](../../apps/mobile/src/components/Brand.tsx), [transport dropdown](../../apps/mobile/src/components/TransportSelect.tsx) | Shared logo and colored wordmark, brand spelling in screen copy, and accessible transport selection |
| [Route domain types](../../apps/mobile/src/domain/routes.ts), [join session](../../apps/mobile/src/domain/join-session.ts) | Code/link parsing, API DTOs, and membership-before-snapshot ordering |
| [Route API client](../../apps/mobile/src/api/routes.ts) | Create, access, join, and Bearer-authenticated snapshot requests with timeout/error handling |
| [Native storage](../../apps/mobile/src/storage/native-session.ts), [session repository](../../apps/mobile/src/storage/session-repository.ts) | Encrypted device identity, profile, per-route member/owner access, and last route |
| [Connection-check screen](../../apps/mobile/src/screens/ConnectionCheckScreen.tsx) | Connecting, connected, and unavailable states; manual retry and lifecycle cancellation |
| [API configuration](../../apps/mobile/src/api/config.ts) | API base URL validation and Android emulator development default |
| [Health client](../../apps/mobile/src/api/health.ts) | Uncached health request, response validation, and five-second timeout |
| [Request tests](../../apps/mobile/tests/health.test.cjs) | Configuration, failed requests, timeouts, cancellation, and recovery |
| [Route/session tests](../../apps/mobile/tests/routes.test.cjs) | API errors, persisted identity, credential scope, and recovery without repeat joining |
| [Map tests](../../apps/mobile/tests/map.test.cjs) | Segment boundaries, latest locations, empty maps, invalid geometry, and antimeridian fitting |
| [index.ts](../../apps/mobile/index.ts) | Registers the screen-off location task outside React, then the root component |
| [package.json](../../apps/mobile/package.json) | `@keepup/mobile`, runtime dependencies, and Expo commands |
| [app.json](../../apps/mobile/app.json) | Expo configuration, native identity, icons, theme, and config plugins |
| [pnpm workspace](../../pnpm-workspace.yaml) and [lockfile](../../pnpm-lock.yaml) | Workspace membership and resolved dependencies |

The Android application ID is `eu.kesopeso.keepup`. The native display name is `KeepUp` and the Expo slug is `keepup`. Mobile launcher and fallback icons use the established K4-10 Waypoint K identity from `assets/brand`. Android uses a transparent foreground padded for adaptive icon masks, a solid `#173023` background, and a monochrome themed icon. Icon exports live in `apps/mobile/assets`; native regeneration applies the name and assets from `app.json`. React Native screens use `assets/header-logo.png`, exported from the web mark with transparent corners outside its rounded box, beside a lowercase `keepup` wordmark, with `keep` in white and `up` in green. The shared header covers startup, joining, live routes, and archives; the connection screen uses the same 32 px wordmark and 40 px logo and aligns its content at the top like the other mobile screens. The white/green brand coloring applies only to brand headers and app titles. Brand mentions in body copy, descriptions, controls, and errors stay lowercase and inherit the surrounding text color. Joining uses an accessible transport dropdown with all seven existing modes and preserves the saved selection. These changes use existing React Native components and the bundled icon, without new native dependencies. `expo-system-ui` applies the configured light system interface style; screens set their own background and status-bar appearance.

### App description and launch screen

Expo configuration includes the description "Share your live location and follow your group along the route." This project metadata is separate from a future Google Play store listing.

The `expo-splash-screen` config plugin renders the branded `assets/splash-icon.png` on the KeepUp dark green `#173023` background in light and dark modes. The splash image uses contain sizing with a configured width of 240. Android native regeneration applies these settings; review the standalone release APK because development-client splash behavior can differ. The app uses the module's default splash lifecycle without a custom loading delay.

### Account authentication

A1 adds an opt-in native account UI. `EXPO_PUBLIC_AUTH_ENABLED=true` selects [AccountApp](../../apps/mobile/src/auth/AccountApp.tsx) instead of the legacy route shell. The default remains false. A2 adds My routes and account route/live integration. Account mode never reads legacy device identity or saved membership credentials.

The [controller](../../apps/mobile/src/auth/controller.ts) serializes requests and credential writes. React receives public user/context state, without session or flow credentials. [Native transport](../../apps/mobile/src/auth/api.ts) sends `X-KeepUp-Client: native`, omits cookies and browser headers, and uses the session bearer or the separate Google flow bearer for the appropriate endpoints. Request and response-body consumption share a fifteen-second timeout. Password change saves its replacement credential before the next authenticated read. Failed secure writes retain the issued response for Retry saving access without repeating signup or rotation; polling pauses during this recovery.

[Native storage](../../apps/mobile/src/auth/native.ts) uses Expo SecureStore, scoped by a SHA-256 digest of the API base. Session, Google flow and navigation intent have separate keys. Flow expiry never extends, and destination records expire after 24 hours. Passwords, confirmation inputs, Google ID proofs and email-link tokens never enter persistence. Signup/reset/onboarding check matching passwords and the shared composition rules before submitting. Screens include visibility controls, autofill hints, keyboard navigation, focused inline errors, rate-limit feedback, offline retry, pending activation, recovery, settings and current/all-device logout. Email and username are read-only.

Foreground checks and five-second visible polling refresh pending activation and session/flow authority. Revocation clears the secure session; temporary network failures retain it. Process recreation reloads SecureStore, then checks the server. Protected destinations use an allowlist and recheck route access; unavailable routes fall back to My routes. A2 consumes retained create, account and route destinations after activation.

The [local Expo module](../../apps/mobile/modules/keepup-google/android/src/main/java/expo/modules/keepupgoogle/KeepUpGoogleModule.kt) uses Android Credential Manager's explicit Sign in with Google button flow, with the backend nonce and configured server client audience. Only the ID token is submitted as proof. Cancellation discards the secure flow, and a generation fence ignores late picker callbacks. Native Google contexts restore onboarding or third-party email challenges after recreation. Confirmation never signs in the link-opening device; the initiating context must exchange explicitly.

[Dynamic Expo configuration](../../apps/mobile/app.config.ts) generates verified HTTPS app-link filters from the approved origin. The native module captures incoming links and clears the in-process Activity Intent data before forwarding the URL to the UI. Link tokens stay only in the mounted confirmation/reset screen and require an explicit POST action. The app writes no link URL to storage or logs. Android and the invoking browser still control their own launch/history records, and production domain association requires the deployment's asset links. A separate custom development scheme preserves Expo development-client launch behavior; account parsing accepts only the configured HTTPS origin.

Configuration, rebuilding and device review belong to the [Android account workflow](../workflow/mobile-development.md#account-authentication-review). Real Google, real email, verified domain association and physical-device checks remain release gates.

### Account routes and live authority

[AccountRoutes](../../apps/mobile/src/auth/AccountRoutes.tsx) owns the native My routes list, account-settings entry, create and code/link join forms. [Account route API](../../apps/mobile/src/auth/route-api.ts) sends account requests through the credential controller's queue, including PATCH/DELETE, so password rotation and route requests cannot race on an old credential. Bodies contain route/transport fields only. The immutable account username identifies the member. No account credential enters the legacy session repository.

The list has Active/Archived tabs, full filtered counts, additive Owned/Joined/Left categories, 20-item cursor pagination and route-ID deduplication. Fresh startup uses Active with Owned/Joined enabled and Left disabled. Returning from a route/settings retains filters, loaded pages and scroll in memory, revalidating the retained pages. Logout, revocation and account switching discard that cache. Loading, no routes, no matches, all categories off, retry and offline feedback are distinct. Retained offline summaries cannot open routes or start create/join. Every row retains Share, including archives and former memberships. List and map use the same native share helper, silent cancellation and selectable-link fallback. Public links contain only the code and use the approved link origin, falling back to the configured API origin and deployment prefix.

Opening a code first requests a snapshot to restore current account membership without another join. A 403 opens access metadata and transport/password entry. Former active and archived memberships require explicit rejoin, including password checks; archives remain read-only. Owner edit/close/delete and member leave controls follow server capabilities. Close/delete/leave require confirmation. Successful lifecycle actions return to My routes and revalidate it. These account controls do not change the legacy route shell's owner-control gap.

The shared [native route screen](../../apps/mobile/src/screens/RouteSnapshotScreen.tsx) keeps the existing map, drawer and foreground location service. Account [viewing sessions](../../apps/mobile/src/live/viewing-session.ts) fetch a fresh one-use ticket before opening the socket. Native upgrades send the native transport header; the first frame carries the ticket, never a legacy token or URL credential. Connection-specific viewer capabilities override REST capabilities while connected. Start acknowledgments establish the sender generation; positions and Stop include it. Concurrent viewers and other-device restrictions follow B6. Reconnect clears the generation, removes local GPS capture and requires an explicit Start after returning to a live viewer connection. Successful Start is the only way to start the native location service; login, process restoration and list navigation never start it.

Remote session revocation immediately invalidates viewing, hides history, stops the native watcher/service and clears secure account access. Forbidden/deleted memberships stop capture and offer route entry again. Foreground polling supplies an additional session check. Native task deliveries continue to wake the viewing session for stalled connections while explicit screen-off sharing is active. Physical-device account-mode screen-off, power-management and network-transition acceptance remain release checks.

### Live viewing

Active routes connect to the existing `/ws` endpoint and authenticate with the saved member token in the first message. The token never appears in the URL. Closed archives do not connect. The screen shows connecting, connected, reconnecting, paused, and invalid-access states.

Accepted position events update the existing segment directly. The client validates coordinates, timestamp, and sequence; orders points by segment sequence; and ignores duplicates. Membership, presence, metadata, and route-close events trigger a coalesced authenticated snapshot refresh. Positions arriving during a refresh are buffered and merged so a slower snapshot cannot erase new locations. A bounded buffer forces resynchronization if overwhelmed. No map remount is needed, so a manual viewport survives updates.

After subscription, the client fetches another snapshot to cover changes between the initial fetch and subscription. Unexpected disconnections retry with exponential backoff from one second to 30 seconds and fetch a snapshot to recover missed events. Authentication has a ten-second client timeout. Duplicate-member connections retry without replacing the server's existing connection. Invalid REST access clears saved membership through the existing session loader and stops retrying. Temporary failures retain the last map and history.

Spectators leaving the Android foreground close the socket and abort pending requests. Explicit Android sharing keeps the same connection and reconnect loop active while the screen is locked. Returning fetches current state without replacing a healthy connection. Leaving the screen cancels requests and timers and ignores old socket events. Closing a route refreshes its archive and stops live updates.

Live viewing and foreground location sharing use the same authenticated connection. Screen-off sharing uses the Android service described below.

### Foreground location sharing

Start sharing requests Android foreground permission and obtains a usable initial GPS fix before sending `start_sharing`. A cancellable native watch supplies the initial fix with a fifteen-second timeout. The Android permission dialog temporarily pauses the activity; the start flow allows the permission result, then waits for the foreground live connection before requesting GPS. Denial, blocked permission, disabled device Location, and unavailable/timed-out fixes have actionable messages. Blocked access offers Open app settings.

Sharing commands have request IDs and ten-second confirmation timeouts. Rejection messages explain route policy and occupied tracker slots. GPS watching starts after command acknowledgement and a snapshot confirming tracking or stale status. Expo Location requests high accuracy, a five-second minimum interval, and zero minimum distance so stationary members can maintain tracking presence. Samples include accuracy and UTC client measurement time; invalid optional altitude/speed/heading values are omitted. Duplicate local timestamps are dropped. The API remains responsible for quality and movement validation.

GPS rejection guidance persists until the viewer's own next accepted sample or confirmed Stop. Another member's accepted location cannot clear it. Native watcher failures stop capture and attempt to release the server tracking slot. Stop removes the watcher immediately, even if disconnected; when offline, the UI explains that spectator status still needs confirmation after reconnecting. Late callbacks and delayed watcher installations cannot restart capture.

Android retains its location service through transient network failures and drops disconnected samples. Sharing resumes after live reconnection and snapshot catch-up if the server still reports tracking or stale. Locking the screen preserves an established sharing intent. Backgrounding during the initial GPS request cancels it. Reopening after process termination offers Resume sharing or Continue as spectator for a tracking/stale membership. Neither action is sent before the live connection is ready. Changing routes while sharing first confirms Stop.

### Screen-off tracking

Android uses Expo Location with Expo TaskManager and a location foreground service. Start requests location permission and, on Android 13 or later, notification permission. Denying notifications prevents sharing and offers settings guidance. The service starts while the activity is visible after sharing is acknowledged. It requests high accuracy at a five-second minimum interval and zero distance threshold. Its ongoing KeepUp notification opens the app, where Stop ends capture. The screen explains screen-off sharing and battery/data use before Start.

The task is registered outside React in the entry module. A single in-memory owner receives callbacks. Native start/stop calls are serialized so delayed cleanup cannot stop a replacement session. Task delivery selects only the newest fix in a batch and drops fixes older than ten seconds. There is no application offline queue or replay. The server still validates every submitted point. The same authenticated WebSocket serves viewing and sharing; a background spectator has no connection.

Network loss leaves the service registered because Android restricts starting foreground services from the background. Samples are dropped until the connection authenticates and its snapshot confirms tracking/stale status. Fixes measured before reconnection are also discarded. Native location callbacks check reconnect and request/authentication deadlines because Android can pause JavaScript timers while locked. An active sharing connection with no server messages for 45 seconds is replaced to recover sockets left half-open by network changes. Stop, invalid access, a closed route, loss of tracking status, or leaving the route screen remove capture. Returning checks permission, device Location, and task registration. A failed service requires explicit Resume. Force-stop and process termination do not grant consent to restart: startup unregisters any old task, and callbacks without an owner unregister themselves. Removing the app from Recents uses `killServiceOnDestroy: true`.

The generated manifest includes foreground-service/location and notification permissions. `RECEIVE_BOOT_COMPLETED` is required by Expo TaskManager's persisted job scheduler; it does not authorize KeepUp to resume sharing after reboot. `ACCESS_BACKGROUND_LOCATION` is disabled because this service starts through an explicit foreground action. Android while-in-use permission covers this foreground-service model. iOS continues to use the foreground watcher and is unvalidated.

Physical-phone foreground and screen-off tracking checkpoints are complete on the owner's outdoor success report. Detailed duration, power-management, and battery measurements were not supplied; see [current validation limits](../planning/status.md#android-implementation-checkpoints). This does not establish reliability across Android manufacturers.

### Shared backend integration

The route behavior in this section and the saved-membership/creation sections below describes legacy mode. [Account routes](#account-routes-and-live-authority) use the separate account API and controller; map rendering is shared by both modes.

The connection-check screen is available from joining through Check server connection. It checks `GET /api/healthz` through the existing proxy when mounted. The API checks database reachability, so a successful connection indicates API and database readiness at the time of that request. The client requires a successful HTTP response with JSON `status: "ok"`; an HTML page or unrelated response cannot appear connected.

Checks time out after five seconds, including reading the response body. The screen reports network failures, timeout, server unavailability, invalid responses, or configuration failure. Retry starts a fresh check and stays disabled while checking. Check again is available after success. Unmounting or replacing a check aborts its request and prevents old results from updating the screen. There is no automatic polling; Connected records the latest completed check.

API addresses come from `EXPO_PUBLIC_API_URL`, with an Android emulator default only in development. Release builds require an explicit address. Setup is owned by the [Android workflow](../workflow/mobile-development.md#api-connection).

Mobile route operations use the same [REST contract](api-and-live.md) as the web client. The join screen accepts a six-character code or a pasted HTTP(S) link with `/routes/{code}`, normalizes the code to uppercase, and always requests the configured backend. A pasted link never changes the backend or receives the device's credentials. Automatic Android deep-link opening is not implemented.

Without saved access, Continue fetches route access metadata before collecting display name, one of the seven supported transport modes, and a password only when required. Join posts the profile and password, saves the returned membership, then opens the authenticated snapshot. Wrong passwords, duplicate names, missing routes, network failures, and unexpected responses show readable feedback with manual retry. Route requests have a ten-second timeout covering response parsing; concurrent form submissions are blocked.

The snapshot displays route name, description, code, active/closed state, sharing policy, members, transport modes, written presence status, and owner/self labels. Active routes open a live connection through the viewing session; Refresh requests another snapshot. Failed refreshes retain the last successful snapshot with its update time. Expired access or a deleted route clears saved route credentials and offers Join again. Existing saved members can view closed archives. The shared API currently rejects new members on closed routes, so the join screen explains that limitation; the product's broader closed-route access requirement remains a backend gap.

### Saved membership

Expo SecureStore persists a stable device UUID, display name and transport preference, a separate member token/id and optional owner token for each route, and the last opened route code. Expo Crypto generates the UUID and hashes the configured API base URL for the storage namespace. Credentials from different backends stay separate. The route password is never persisted. The SecureStore config plugin configures Android backup exclusions; tokens are encrypted using the platform's storage and keystore.

Startup restores the last route and requests its snapshot with the saved token instead of posting another join. Entering a previously joined code similarly resumes its membership without requiring a password. Join another route changes the screen and preserves membership; it does not leave the route or revoke access. There is no explicit Leave route action in the current mobile client.

Credentials are saved before fetching the initial snapshot, so a failed snapshot can be retried without creating another member. If saving a successful join fails, its returned credentials remain in memory and Retry saves them again instead of repeating POST. Device storage errors are surfaced. Unmounting aborts network work and old responses cannot update another screen. There is no automatic POST retry: the shared join endpoint is not idempotent, so a lost join response can leave a server membership without recoverable credentials.

Live viewing and foreground location sharing are implemented. Route creation is implemented; owner edit, close, and delete actions remain future mobile work.

Screen-off tracking is implemented, with Android permission/lifecycle emulator coverage and owner-reported outdoor physical-device success. Current review limits live in [delivery status](../planning/status.md#android-implementation-checkpoints). iOS remains unvalidated.

### Android route creation

Create a new route is available on the join screen. The keyboard-aware creation form collects route name, display name, and the saved transport preference. Expand Route settings for an optional description, optional password, and either shared location policy. Everyone can share is the default. The form checks required names, trims names/descriptions, and preserves password whitespace to match the shared API. Creating a route never starts location capture.

The creation session saves the profile before `POST /routes`, validates returned owner/member access, and stores both tokens together in the backend-scoped SecureStore membership record before opening the existing native route screen. Previous memberships remain available. Existing non-owner records without an owner token remain compatible. Startup restores the new last route, including owner access, without creating or joining again. Invalid/deleted access clears the whole route record.

Concurrent submissions share one request. A successful response remains in memory until credential and last-route writes succeed. If storage fails, the form shows the created code, locks inputs/navigation, and offers Retry saving access. Retry saves the same tokens without another POST. Snapshot failures use the existing route-screen retry and retain access. Passwords are never persisted and are cleared after successful creation or a returned creation response followed by a storage error.

There is no automatic create-request retry. The existing endpoint is not idempotent; losing its response can leave a created route without recoverable owner credentials. Closing the app before failed credential storage is recovered can also lose access held only in memory. Owner tokens are retained for the next implementation checkpoint; Android edit, close, and delete controls are not yet implemented.

### Native snapshot map

The Android route screen renders a full-screen native MapLibre map beneath a branded route header and bottom drawer. The map is a sibling of the drawer and has no parent ScrollView, so vertical pan and pinch gestures reach the native map. The drawer has exactly two resting positions, Peek and Full. Peek shows the membership/sharing state and Start, Stop, or recovery actions. Full exposes permission guidance, settings/error feedback, route details, members, refresh/retry, and changing routes in its own ScrollView. Only the handle captures drawer drags; tapping it or Android Back also switches positions. Peek measures its content height so wrapped text and recovery actions remain visible. Drawer animation respects reduced-motion settings. Permission errors open Full automatically. `@maplibre/maplibre-react-native` 11.5 supports the app's React Native architecture and supplies an Expo config plugin. The plugin is recorded in `app.json`; installing this dependency requires a new development APK. iOS remains untested.

Each saved path segment becomes a separate colored line. A member's latest saved point, selected by timestamp across segments, becomes a marker. Last-known markers remain available for stale, offline, spectating, and left members, matching the existing web renderer; they do not imply a live connection. System-font name pills render through native Marker views above saved positions, with a 152px cap, one-line ellipsis, 0.75-alpha charcoal backgrounds, and opaque text. Non-tracking pills include written presence. The [label placement helper](../../apps/mobile/src/map/member-labels.ts) hides names that overlap labels, circles, or header/drawer insets, prioritizing the selected member. Native camera projection and measured label sizes recompute visibility after position, camera, size, and drawer changes. Circles remain at their coordinates even when their names are hidden. Selecting a located member row returns the drawer to Peek. Tapping a circle, visible name pill, or a located member row fits that member's history and shows their name, written presence status, and localized last-location time. Members without coordinates show No saved location and cannot be focused.

The initial camera fits all saved geometry with padding for the header and Peek drawer. Opening Full keeps the map mounted and does not change its dimensions or camera. Two-finger pinch controls zoom; the map has no zoom buttons. Native pan and double-tap zoom remain enabled. A single location uses zoom 15, and the map caps zoom at 16. Bounds use the shortest longitude arc for routes crossing the antimeridian. Each segment's longitudes are unwrapped for drawing. Snapshot response validation rejects malformed segments, out-of-range coordinates, and invalid timestamps before native rendering. The geometry converter also breaks lines at invalid samples instead of connecting across gaps.

Manual map interaction disables automatic fitting. Refresh replaces geometry while preserving a manual viewport. Fit group restores automatic fitting and fades out when no fit action is available; the hidden control is excluded from interaction and accessibility. Camera and Fit group animations respect reduced-motion settings. Size and overlay-padding changes refit only in automatic mode; a manual camera is retained across live positions, refreshes, and drawer movement. Empty active routes explain when locations appear; empty archives report no saved locations. Map loading failures or a fifteen-second rendering timeout offer Retry map without blocking route/member controls.

MapLibre reports tile download failures through its native log handler rather than the style-loading callback. The mounted map owns that handler and releases it on unmount. Reported tile failures remain visible until Retry map rebuilds the map and fits the full route again, even if MapLibre reports a completed frame for missing tiles.

The basemap uses the same OpenStreetMap raster styling as the web client. Tile configuration lives separately from geometry. Native tile requests identify KeepUp through a scoped User-Agent header; MapLibre manages its normal HTTP cache. Visible OpenStreetMap attribution links to the copyright page. This slice does not add tile downloads or offline packs. Review the [tile usage policy](https://operations.osmfoundation.org/policies/tiles/) before changing provider behavior.

Active-route geometry updates through live viewing. Location capture belongs to the foreground sharing controller rather than the map renderer.

### Native project generation

Expo prebuild generates `apps/mobile/android` from app configuration, installed dependencies, and config plugins. The generated Android and iOS directories are ignored by the mobile `.gitignore`. Keep durable configuration in `app.json` and config plugins so regeneration preserves it.

Native dependency or configuration changes require regeneration and a new APK. JavaScript/TypeScript edits normally use Metro and Fast Refresh. See the [build and restart commands](../workflow/mobile-development.md#build-and-install).

## Sources and connections

- [Mobile source](../../apps/mobile/), [app configuration](../../apps/mobile/app.json), and [ignore rules](../../apps/mobile/.gitignore)
- [Web boundaries](frontend.md), [API and live protocol](api-and-live.md), and [Android workflow](../workflow/mobile-development.md)
- [Expo Location](https://docs.expo.dev/versions/latest/sdk/location/), [TaskManager](https://docs.expo.dev/versions/latest/sdk/task-manager/), and [Android while-in-use location services](https://codelabs.developers.google.com/codelabs/while-in-use-location)
- [Expo SecureStore](https://docs.expo.dev/versions/latest/sdk/securestore/) and [Expo Crypto](https://docs.expo.dev/versions/latest/sdk/crypto/)
- [MapLibre requirements](https://maplibre.org/maplibre-react-native/docs/setup/getting-started/) and [Expo setup](https://maplibre.org/maplibre-react-native/docs/setup/expo/)
- [Android implementation sequence](../planning/roadmap.md#phase-9-mobile-client-android-first) and [deferred iOS work](../planning/backlog.md#planned-later)

## Change impact

Follow the [change-impact guide](change-impact.md) for mobile screen, API, and native build changes. Check the shared product and protocol references before adding client behavior. Foreground location changes must follow the controller, native adapter, screen, live commands, tests, and app configuration links above.

## Validation

Current evidence and its limits are recorded in [mobile delivery status](../planning/status.md#mobile-foundation). Review mobile changes against that status so a working development build is not reported as a finished route-sharing client.
