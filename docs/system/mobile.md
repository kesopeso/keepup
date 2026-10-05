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
| [App.tsx](../../apps/mobile/App.tsx) | Renders the API connection-check screen |
| [Connection-check screen](../../apps/mobile/src/screens/ConnectionCheckScreen.tsx) | Connecting, connected, and unavailable states; manual retry and lifecycle cancellation |
| [API configuration](../../apps/mobile/src/api/config.ts) | API base URL validation and Android emulator development default |
| [Health client](../../apps/mobile/src/api/health.ts) | Uncached health request, response validation, and five-second timeout |
| [Request tests](../../apps/mobile/tests/health.test.cjs) | Configuration, failed requests, timeouts, cancellation, and recovery |
| [index.ts](../../apps/mobile/index.ts) | Registers the root React component |
| [package.json](../../apps/mobile/package.json) | `@keepup/mobile`, runtime dependencies, and Expo commands |
| [app.json](../../apps/mobile/app.json) | Expo configuration, native identity, icons, theme, and config plugins |
| [pnpm workspace](../../pnpm-workspace.yaml) and [lockfile](../../pnpm-lock.yaml) | Workspace membership and resolved dependencies |

The Android application ID is `eu.kesopeso.keepup`. The display name and Expo slug remain `mobile`; KeepUp branding has not been applied to the starter. `expo-system-ui` applies the configured light interface style on Android.

### Shared backend integration

The app checks `GET /api/healthz` through the existing proxy when its connection screen mounts. The API checks database reachability, so a successful connection indicates API and database readiness at the time of that request. The client requires a successful HTTP response with JSON `status: "ok"`; an HTML page or unrelated response cannot appear connected.

Checks time out after five seconds, including reading the response body. The screen reports network failures, timeout, server unavailability, invalid responses, or configuration failure. Retry starts a fresh check and stays disabled while checking. Check again is available after success. Unmounting or replacing a check aborts its request and prevents old results from updating the screen. There is no automatic polling; Connected records the latest completed check.

API addresses come from `EXPO_PUBLIC_API_URL`, with an Android emulator default only in development. Release builds require an explicit address. Setup is owned by the [Android workflow](../workflow/mobile-development.md#api-connection).

Future mobile route operations will use the same [REST and WebSocket contract](api-and-live.md) as the web client. Route permissions, member state, persistence, and [GPS validation](../product/tracking.md#gps-validation) remain server responsibilities. The existing [product references](../product/CONTEXT.md) own user-facing route and tracking rules.

The app has no route screens, token storage, live connection, map, or location service yet.

Native location capture, permission handling, background execution, and reconnection will need mobile implementations. Screen-off tracking is an early validation target, not an implemented guarantee. Android and iOS will each need platform-specific lifecycle and permission checks.

### Native project generation

Expo prebuild generates `apps/mobile/android` from app configuration, installed dependencies, and config plugins. The generated Android and iOS directories are ignored by the mobile `.gitignore`. Keep durable configuration in `app.json` and config plugins so regeneration preserves it.

Native dependency or configuration changes require regeneration and a new APK. JavaScript/TypeScript edits normally use Metro and Fast Refresh. See the [build and restart commands](../workflow/mobile-development.md#build-and-install).

## Sources and connections

- [Mobile source](../../apps/mobile/), [app configuration](../../apps/mobile/app.json), and [ignore rules](../../apps/mobile/.gitignore)
- [Web boundaries](frontend.md), [API and live protocol](api-and-live.md), and [Android workflow](../workflow/mobile-development.md)
- [Android implementation sequence](../planning/roadmap.md#phase-9-mobile-client-android-first) and [deferred iOS work](../planning/backlog.md#planned-later)

## Change impact

Follow the [change-impact guide](change-impact.md) for mobile screen, API, and native build changes. Check the shared product and protocol references before adding client behavior. Native permission and transport code will need new source links here when implemented.

## Validation

Current evidence and its limits are recorded in [mobile delivery status](../planning/status.md#mobile-foundation). Review mobile changes against that status so a working development build is not reported as a finished route-sharing client.
