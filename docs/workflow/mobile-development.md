---
type: reference
status: active
---

# Android development on Ubuntu

## Purpose

Run and debug the mobile development build on an Ubuntu-hosted Android emulator. This page owns the local commands, tooling, ports, and troubleshooting. [Mobile boundaries](../system/mobile.md) own the app architecture; [delivery status](../planning/status.md#mobile-foundation) owns progress and validation evidence.

## Owning facts

### Moving parts

| Component | Where it runs | Job |
|---|---|---|
| Android Studio and emulator | Ubuntu desktop | Manage SDK packages and run the virtual phone |
| `mobile-helper` | Docker Compose, tools profile | Run pnpm/Expo and Java/Gradle to build the APK |
| ADB server | Ubuntu, started by Android Studio | Connect container ADB commands to the emulator on port `5037` |
| Metro | Inside `mobile-helper`, using host networking | Serve the app's JavaScript and refresh edits on port `8081` |
| React Native DevTools | Ubuntu Chrome/Chromium | Inspect the emulator's JavaScript runtime through Metro |
| Existing proxy/web/API/PostGIS services | Docker Compose | Serve KeepUp through the proxy on port `3000` |

Build and package commands run in Docker. Android Studio manages the SDK and emulator on the host. An Android build produces a development APK; Metro stays running to serve JavaScript to that installed app. Starting `mobile-helper` does not start the backend stack.

### Prerequisites

The current Dockerfile targets Ubuntu/Linux on `x86_64`: Node 24 Bookworm and OpenJDK 17, with an amd64 `JAVA_HOME`. Android native SDK binaries run in this Debian-based container.

Android Studio's SDK location must match `${HOME}/Android/Sdk`, which Compose mounts at `/opt/android-sdk`. Adjust the mount if the SDK is elsewhere. Host networking is used for this Linux workflow; it is not a verified macOS or Windows setup.

Use Android Studio's SDK Manager to install:

- Android SDK Platform 36, matching the installed React Native compile/target SDK.
- Android SDK Build-Tools `36.0.0`.
- Android SDK Platform-Tools and Command-line Tools.
- NDK `27.1.12297006`.

These versions come from the installed React Native Gradle version catalog. Recheck it on framework upgrades. The SDK mount is writable so Gradle can download missing components such as CMake after the relevant licenses are accepted. Downloads can persist in the host SDK.

For emulator acceleration, enable CPU virtualization and ensure the Ubuntu user can access `/dev/kvm`. Start an existing virtual phone using the play button in Android Studio's Device Manager, then leave it running. The build SDK and emulator OS are separate: the current API 36 build runs on the Android 17/API 37 emulator.

### First-time Android setup

Complete the [backend setup](development.md#first-time-setup) first. Install Android Studio, the SDK packages listed above, and an emulator image in Device Manager. Accept the Android SDK licenses and start a virtual phone. The supported host for these commands is Linux `x86_64` with the SDK at `${HOME}/Android/Sdk`.

```sh
make doctor-mobile
make mobile-setup
```

`make doctor-mobile` checks Docker, the Compose configuration, host architecture, required SDK packages, and whether ADB sees a running device. It prints a specific fix for each missing prerequisite. `make mobile-setup` builds the helper image, installs the workspace dependencies, checks Expo package compatibility, generates the native Android project, builds and installs the development APK, and starts Metro. Select the running emulator when Expo asks. The first build may take several minutes and download more Android components. Leave the Metro terminal running.

The app should open to Join a route or restore a saved route. Create a route in the web app at `http://localhost:3000`, then join it by code or pasted share link in Android. Its details, members, and native map should load. Open `http://127.0.0.1:8081/open-debugger` in a host browser after the app connects to Metro to check React Native DevTools. No mobile environment file is needed for the default Android emulator; physical devices need an [API address override](#api-connection).

The current app uses native SecureStore, Crypto, and MapLibre modules. A development APK built before those dependencies were added must be rebuilt with `make mobile-setup`; Metro alone cannot add native modules. Keep the app installed if you want to retain its saved memberships.

### Workspace and helper

`apps/mobile` is registered in `pnpm-workspace.yaml` as `@keepup/mobile`. The [mobile helper](../../bin/mobile-pnpm.sh) forwards arguments to `pnpm --filter @keepup/mobile` in `mobile-helper`, adding interactive Docker flags when stdin is a terminal.

Run commands from the repository root. For example:

```sh
docker compose build mobile-helper
./bin/mobile-pnpm.sh install
./bin/mobile-pnpm.sh exec expo install --check
```

The last command checks Expo dependency compatibility. Use `expo install` for Expo-compatible native library versions. Mobile React and React DOM currently match the Expo-required version; the web client resolves its own versions.

### Build and install

Keep the emulator running and use:

```sh
./bin/mobile-pnpm.sh exec expo run:android --device
```

Expo generates the Android project if absent, compiles it with Gradle, installs the development APK using ADB, and starts Metro. The initial build downloads dependencies and can take several minutes. The APK is generated at `apps/mobile/android/app/build/outputs/apk/debug/app-debug.apk`.

The container talks to Android Studio's existing ADB server using `ADB_SERVER_SOCKET=tcp:127.0.0.1:5037`. Keep the SDK Platform-Tools version shared between host and container, as configured by the SDK mount.

After changing native dependencies or app configuration, regenerate before rebuilding:

```sh
./bin/mobile-pnpm.sh exec expo prebuild --platform android --clean --no-install
./bin/mobile-pnpm.sh install
./bin/mobile-pnpm.sh exec expo run:android --device
```

`--clean` replaces the generated Android directory. Keep changes that must survive in app configuration or config plugins.

### Daily development

Once the APK is installed, JavaScript/TypeScript work needs only Metro:

```sh
make mobile
```

`make mobile` runs the mobile readiness check, then calls the [Metro launcher](../../bin/start-metro.sh). That launcher runs the mobile helper with `expo start --dev-client --localhost --android --port 8081`. It prints the browser debugger link when Expo reports that Metro is ready, streams the normal logs, and preserves command failures. It resolves the repository root from its own location.

`--android` opens the installed app and establishes ADB forwarding from the device's port `8081` to the host's port `8081`. Leave this terminal running; Ctrl+C stops Metro. Fast Refresh applies supported code edits without rebuilding the APK.

The configured `EXPO_UNSTABLE_HEADLESS=1` disables the Docker desktop DevTools launcher and Expo's interactive terminal shortcuts. Pressing `a` in this setup does nothing; use `--android` at startup. This is a version-specific Expo setting, so verify its behavior when upgrading.

### Addresses and browser debugging

| Address | Use |
|---|---|
| `http://127.0.0.1:8081` | Ubuntu's local Metro server; also reachable from the emulator through ADB forwarding |
| `http://127.0.0.1:8081/open-debugger` | Open React Native DevTools in Ubuntu Chrome/Chromium after the app connects |
| `http://10.0.2.2:3000/api` | Default development REST API base for the Android emulator, including the connection check |
| `ws://10.0.2.2:3000/ws` | Intended emulator WebSocket endpoint for future mobile integration |

The emulator's `10.0.2.2` address points to the Ubuntu host. `localhost` inside the emulator normally means the virtual phone; ADB forwarding makes the Metro port an explicit exception.

Start the separate backend stack with `docker compose up -d`. The mobile app calls its health endpoint on launch and manual retry. Backend routing remains owned by the [same-origin proxy contract](development.md#same-origin-proxy).

Compose sets `EXPO_DEBUG=1`, which enables the browser debugger redirect in the installed Expo CLI and emits verbose logs. Use `/open-debugger` to discover the current runtime rather than saving a session-specific inspector URL. Keep the emulator app and Metro running while debugging.

### API connection

The connection screen uses `http://10.0.2.2:3000/api` by default in development. It requires the proxy, API, and database to be running, separately from Metro. No APK rebuild is needed for this JavaScript screen.

To override the API address, copy [`.env.example`](../../apps/mobile/.env.example) to `apps/mobile/.env.local` and set `EXPO_PUBLIC_API_URL` to the public REST base, including `/api` when using the existing proxy. Restart Metro after changing environment settings. Physical devices need a reachable host address rather than the emulator's `10.0.2.2`. Release builds require an explicit API URL and should use the production HTTPS proxy. This Expo public variable is bundled in the app; it must not contain secrets.

Run mobile checks through Docker:

```sh
./bin/mobile-pnpm.sh typecheck
./bin/mobile-pnpm.sh test
```

The tests use the container's Node 24 built-in runner and TypeScript support, with no additional test dependency. They cover request failure, response validation, timeout through body consumption, cancellation, configuration, and retry recovery. Screen behavior is owned by [mobile boundaries](../system/mobile.md#shared-backend-integration).

### Route joining and saved access

The app starts with Join a route or restores the last saved membership. Create a route in the web app at `http://localhost:3000`, then enter its code or paste its `/routes/{code}` share link in Android. After Continue, choose a display name and transport mode, and enter the password if required. Successful joining opens route details and members; use Refresh for current snapshots. Reopening uses saved membership, without creating another member or asking for the password again.

This slice adds `expo-secure-store` and `expo-crypto`. An APK containing these modules is required; updating Metro alone cannot add native modules. Follow [build and install](#build-and-install) to regenerate and install after pulling this change. Keep the installed app's data when updating; uninstalling Android removes its saved tokens. The SecureStore plugin lives in `app.json`, so native regeneration preserves backup exclusions. [Saved membership](../system/mobile.md#saved-membership) owns storage and error behavior.

Route/session tests cover code/link parsing, password/name errors, Bearer authentication, backend isolation, persistent identity, invalid-token cleanup, and retry after snapshot or storage failures without repeating successful joins. WebSockets, location capture, and automatic Android link handling follow later.

### Native map review

The snapshot map adds `@maplibre/maplibre-react-native` and its Expo config plugin. Regenerate and rebuild using [build and install](#build-and-install); Metro refresh alone cannot install the native renderer. When Metro is already running, use `./bin/mobile-pnpm.sh exec expo run:android --no-bundler` for the build and installation. Keep Metro's localhost address for browser debugging.

Join a route with saved locations from the web client, then Refresh in Android to see its paths and last-known markers. Pan or zoom to reveal Fit group. Refresh must preserve that manual view, and Fit group must fit the whole history and hide again. Tap a marker or a located member row to focus their history and read the location timestamp. Empty routes display an explanatory message; closed archives use the same map. A failed map load offers Retry map while route controls remain usable.

The Android app still needs manual Refresh for new positions. It does not request location permission or send GPS data yet. Geometry tests cover separate segments, latest-point selection, missing/invalid coordinates, and antimeridian bounds. Basemap configuration and attribution are owned by the [native map reference](../system/mobile.md#native-snapshot-map).

### Troubleshooting

- **Connection screen reports unavailable:** confirm `docker compose up -d` has started the backend stack, then tap Retry. Connected shows the last check result; use Check again to refresh it.
- **DevTools reports missing `libatk-1.0.so.0`:** the desktop debugger is trying to start inside Docker. The configured headless mode avoids this launcher; use the browser debugger on Ubuntu.
- **Browser DevTools immediately disconnects:** check that Metro uses `127.0.0.1:8081` and the browser debugger is opened at that same origin. Do not restore `REACT_NATIVE_PACKAGER_HOSTNAME=10.0.2.2`; it overrides Expo's localhost setting and causes browser WebSocket origin checks to fail.
- **No debugger target is available:** the installed app must connect to Metro first. Restart with the daily-development command and keep the emulator running.
- **Keyboard shortcuts do nothing:** headless mode disables them. Use startup flags such as `--android`.
- **ADB cannot find a device:** start the emulator in Android Studio and check that its host ADB server is running; the container uses that server.

## Sources and connections

- [Compose](../../docker-compose.yml), [Makefile](../../Makefile), [readiness check](../../bin/doctor.sh), [mobile Dockerfile](../../apps/mobile/Dockerfile.dev), [mobile helper](../../bin/mobile-pnpm.sh), and [Metro launcher](../../bin/start-metro.sh)
- [Mobile manifest](../../apps/mobile/package.json), [app configuration](../../apps/mobile/app.json), and [workspace](../../pnpm-workspace.yaml)
- [Expo local builds](https://docs.expo.dev/guides/local-app-development/), [native generation](https://docs.expo.dev/workflow/continuous-native-generation/), and [Android emulator setup](https://docs.expo.dev/workflow/android-studio-emulator/)
- [Android emulator networking](https://developer.android.com/studio/run/emulator-networking-address) and [SDK management](https://developer.android.com/studio/intro/update#sdk-manager)

## Change impact

Update this page when the Docker image, SDK mount, ADB forwarding, Metro flags, or browser debugger workflow changes. Follow [mobile tooling changes](../system/change-impact.md) to the exact source files. Keep implementation and planning facts in their owning references.

## Validation

The owner verified the installed starter and browser debugger on Ubuntu. The connection-check screen was verified against the running backend and through API stop/restart with manual retry. Route joining, password/name errors, authenticated snapshots, and membership restoration were verified on the emulator; archive and deleted-route cleanup were also checked using the actual API/session client against a disposable route. See [delivery status](../planning/status.md#mobile-foundation) for the dated evidence and scope. Before continuing development, review these commands against the actual Compose configuration. Later native tracking must be tested on physical hardware; the emulator setup does not establish battery use or screen-off GPS reliability.
