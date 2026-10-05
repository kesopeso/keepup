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
./bin/start-metro.sh
```

The [Metro launcher](../../bin/start-metro.sh) runs the mobile helper with `expo start --dev-client --localhost --android --port 8081`. It prints the browser debugger link when Expo reports that Metro is ready, streams the normal logs, and preserves command failures. It resolves the repository root from its own location.

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

### Troubleshooting

- **Connection screen reports unavailable:** confirm `docker compose up -d` has started the backend stack, then tap Retry. Connected shows the last check result; use Check again to refresh it.
- **DevTools reports missing `libatk-1.0.so.0`:** the desktop debugger is trying to start inside Docker. The configured headless mode avoids this launcher; use the browser debugger on Ubuntu.
- **Browser DevTools immediately disconnects:** check that Metro uses `127.0.0.1:8081` and the browser debugger is opened at that same origin. Do not restore `REACT_NATIVE_PACKAGER_HOSTNAME=10.0.2.2`; it overrides Expo's localhost setting and causes browser WebSocket origin checks to fail.
- **No debugger target is available:** the installed app must connect to Metro first. Restart with the daily-development command and keep the emulator running.
- **Keyboard shortcuts do nothing:** headless mode disables them. Use startup flags such as `--android`.
- **ADB cannot find a device:** start the emulator in Android Studio and check that its host ADB server is running; the container uses that server.

## Sources and connections

- [Compose](../../docker-compose.yml), [mobile Dockerfile](../../apps/mobile/Dockerfile.dev), [mobile helper](../../bin/mobile-pnpm.sh), and [Metro launcher](../../bin/start-metro.sh)
- [Mobile manifest](../../apps/mobile/package.json), [app configuration](../../apps/mobile/app.json), and [workspace](../../pnpm-workspace.yaml)
- [Expo local builds](https://docs.expo.dev/guides/local-app-development/), [native generation](https://docs.expo.dev/workflow/continuous-native-generation/), and [Android emulator setup](https://docs.expo.dev/workflow/android-studio-emulator/)
- [Android emulator networking](https://developer.android.com/studio/run/emulator-networking-address) and [SDK management](https://developer.android.com/studio/intro/update#sdk-manager)

## Change impact

Update this page when the Docker image, SDK mount, ADB forwarding, Metro flags, or browser debugger workflow changes. Follow [mobile tooling changes](../system/change-impact.md) to the exact source files. Keep implementation and planning facts in their owning references.

## Validation

The owner verified the installed starter and browser debugger on Ubuntu. The connection-check screen was also verified against the running backend and through API stop/restart with manual retry. See [delivery status](../planning/status.md#mobile-foundation) for the dated evidence and scope. Before continuing development, review these commands against the actual Compose configuration. Later native tracking must be tested on physical hardware; the emulator setup does not establish battery use or screen-off GPS reliability.
