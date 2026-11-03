---
type: reference
status: active
---

# Account release acceptance

Account configuration and acceptance procedure. [Task status](../planning/authentication-tasks.md#r1-configuration-and-cross-client-acceptance) owns completion; [validation history](../planning/validation-history.md#r1-local-acceptance-on-2026-10-28) records evidence. The owner treats the current HTTPS deployment as disposable staging and will reset it for day zero. The separate R2 rollout task was [discarded](../planning/authentication-tasks.md#r2-fresh-start-rollout-and-documentation); configuration and manual migration requirements below still apply.

## Local account configuration

Copy the root [environment example](../../.env.example) to `.env` if no local file exists. Preserve existing values. Set `AUTH_ENABLED=true`, exact browser origins in `AUTH_ALLOWED_ORIGINS`, and the browser origin in `AUTH_PUBLIC_URL`. Generate `AUTH_OUTBOX_KEY` with `openssl rand -base64 32`, store it privately and keep it stable. Use `EMAIL_PROVIDER=log` for the selected local acceptance mode. No SMTP configuration is needed. [Email configuration](authentication-email.md) owns queue behavior and settings.

Compose forwards these account settings, session idle duration, trusted proxy CIDRs, SMTP settings and Google public client IDs into their owning API/web services. Other optional settings still require a Compose override. An empty Google configuration leaves password authentication available. Root `.env` does not configure Expo; use the separate [mobile environment example](../../apps/mobile/.env.example).

Before starting account mode, apply migrations through 0007 manually to the intended database. For acceptance, use an isolated Compose project and a disposable database, with separate proxy ports and no existing database-volume mounts. Do not point acceptance/reset commands at application or production data. The API never migrates automatically. Recreate services after environment changes; `restart` alone does not apply new container environment values.

For persistent local testing on `http://localhost:3000`, the optional [account Compose override](../../docker-compose.auth.yml) replaces the PostgreSQL mount with a separate `r1_postgres_data` volume and removes its published database port. Select it in the private root `.env` before starting services:

```dotenv
COMPOSE_FILE=docker-compose.yml:docker-compose.auth.yml
```

Then run `docker compose up -d postgres`, `./bin/migrate.sh up`, and `docker compose up -d api web proxy` in that order. The override does not enable authentication itself; the account settings above still apply. It preserves the legacy `postgres_data` volume. Existing credentials and routes in the selected account database persist across ordinary restarts. To switch back, stop this stack first, remove `COMPOSE_FILE` from `.env`, disable account mode in the API/mobile environments, and recreate the original services. Do not use volume deletion as a switching step.

Use mobile API base `http://10.0.2.2:3000/api` for the standard Android emulator. Keep its Google server client ID equal to the web client ID; the native API allowlist includes that audience and the Android client ID. Native link parsing can use the intended HTTPS deployment origin while REST uses local development HTTP, but the local association endpoint cannot establish association with the HTTPS production domain. Metro uses the separate mobile environment file. Existing local mobile settings can be backed up privately before changing it.

## Google setup

Create or select a Google Cloud project and configure OAuth branding/audience and any required test users. Create a Web application OAuth client. Register the actual browser origins, including `http://localhost` and the port-specific localhost origin for local testing, following [Google's setup guide](https://developers.google.com/identity/gsi/web/guides/get-google-api-clientid). KeepUp uses JavaScript ID-token exchange, so it has no OAuth code callback to register.

Set the web ID as `AUTH_GOOGLE_WEB_CLIENT_ID` for Next.js and include it in API `AUTH_GOOGLE_WEB_CLIENT_IDS`. Use the server/web ID for Android `EXPO_PUBLIC_GOOGLE_SERVER_CLIENT_ID` and include that audience in `AUTH_GOOGLE_NATIVE_CLIENT_IDS`. Register an Android OAuth client for `eu.kesopeso.keepup` and the intended signing certificate's SHA-1 fingerprint in the same project. Include approved native presenter IDs as required by the token's `azp` claim. No client secret belongs in the app. [Google configuration](authentication-google.md) owns allowlist behavior; Android uses the [Credential Manager server client ID](https://developer.android.com/identity/sign-in/credential-manager-siwg-implementation).

## HTTPS links and signing

Choose the owned HTTPS test origin before configuring Android links. Set API `AUTH_PUBLIC_URL` and the exact browser allowlist, then mobile `EXPO_PUBLIC_AUTH_LINK_ORIGIN` and its reachable `EXPO_PUBLIC_API_URL`. Release native sessions require HTTPS. Set `EXPO_PUBLIC_AUTH_ENABLED=true` only against that account-enabled backend.

Next.js serves `/.well-known/assetlinks.json` dynamically. Set `ANDROID_APP_LINK_SHA256_FINGERPRINTS` on the web server to comma-separated, colon-delimited SHA-256 fingerprints of the APK signing certificates approved for this domain. Empty configuration returns `[]` and grants no association; malformed fingerprints return 503. Only package `eu.kesopeso.keepup` is granted link handling. Use the actual installed app signer, including the Play App Signing certificate for Play-distributed APKs. See [Android website associations](https://developer.android.com/training/app-links/configure-assetlinks).

Rebuild Android after changing its link origin, then verify domain association and actual browser-to-app dispatch using the [native review procedure](mobile-development.md#account-authentication-review). Opening an Activity with ADB does not establish verified domain association. Production release signing remains a separate prerequisite.

## Standalone Android acceptance build

The [outdoor build](mobile-development.md#standalone-android-release-build-for-outdoor-testing) does not explicitly enable account mode. For R1, replace the example HTTPS origin and Google web/server client ID below with the configured test deployment values. The same environment must reach both Expo native generation and JavaScript bundling. `EXPO_NO_DOTENV=1` excludes local development overrides; rerunning Gradle tasks avoids reusing a bundle built with earlier public settings.

```sh
docker compose run --rm --no-deps \
  -e EXPO_PUBLIC_AUTH_ENABLED=true \
  -e EXPO_PUBLIC_API_URL=https://YOUR_TEST_HOST/api \
  -e EXPO_PUBLIC_AUTH_LINK_ORIGIN=https://YOUR_TEST_HOST \
  -e EXPO_PUBLIC_GOOGLE_SERVER_CLIENT_ID=YOUR_WEB_CLIENT_ID.apps.googleusercontent.com \
  -e NODE_ENV=production \
  -e EXPO_NO_DOTENV=1 \
  mobile-helper sh -c 'pnpm --filter @keepup/mobile exec expo prebuild --platform android --no-install && cd apps/mobile/android && ./gradlew :app:assembleRelease -PreactNativeArchitectures=arm64-v8a --rerun-tasks'
```

This produces `apps/mobile/android/app/build/outputs/apk/release/app-release.apk` for the ARM64 test phone. The generated release variant currently uses a debug signing key, as described in the outdoor-build procedure. Read the actual APK certificate before registering Google or website association fingerprints:

```sh
docker compose run --rm --no-deps mobile-helper \
  /opt/android-sdk/build-tools/36.0.0/apksigner verify --print-certs \
  apps/mobile/android/app/build/outputs/apk/release/app-release.apk
```

Google Android client registration uses the SHA-1 digest. `ANDROID_APP_LINK_SHA256_FINGERPRINTS` uses the SHA-256 digest with a colon between each byte. Recheck the signer after native regeneration or signing changes.

Install the APK using the [phone installation procedure](mobile-development.md#standalone-android-release-build-for-outdoor-testing). With the phone online and the association file publicly reachable over HTTPS, run these commands from the host on Android 12 or later. Replace `PHONE_SERIAL` and `YOUR_TEST_HOST`:

```sh
~/Android/Sdk/platform-tools/adb -s PHONE_SERIAL shell pm set-app-links --package eu.kesopeso.keepup 0 all
~/Android/Sdk/platform-tools/adb -s PHONE_SERIAL shell pm verify-app-links --re-verify eu.kesopeso.keepup
```

Allow verification to finish, then inspect the result and dispatch a link without specifying an Activity or package:

```sh
~/Android/Sdk/platform-tools/adb -s PHONE_SERIAL shell pm get-app-links eu.kesopeso.keepup
~/Android/Sdk/platform-tools/adb -s PHONE_SERIAL shell am start -a android.intent.action.VIEW -c android.intent.category.BROWSABLE -d https://YOUR_TEST_HOST/auth/verify
```

Require `YOUR_TEST_HOST: verified` and actual KeepUp dispatch. A token-free verification URL checks dispatch only; repeat with a real disposable verification/reset link for acceptance. Do not force-approve the domain to obtain a passing result. See [Android's verification procedure](https://developer.android.com/training/app-links/verify-applinks).

## Acceptance checkpoints

Use disposable accounts/routes and record which requests use real providers, local logs, mocked proofs, synthetic GPS or physical devices.

- Signup queues an inspectable email; a separate browser can explicitly verify without receiving a session. The originating client observes activation and resumes its destination.
- Password login and session restoration work across web/Android. A route created in one client restores membership and ownership in the other without starting tracking.
- Both clients can watch the same route. One account/route has one sender; the second viewer cannot take over. Independent routes can have independent senders. Network recovery requires explicit sharing restart where the account contract requires it.
- Reset and logout-all revoke the other client, close its live socket and remove protected data. A physical Android device must also stop capture and its foreground service.
- Real Google sign-in, cancellation, account linking and onboarding work on web and the intended Android build. Invalid/expired/replaced email links fail without creating sessions.
- Physical Android review covers screen-off sharing, network loss/recovery, notification shutdown, permission loss and password-manager autofill. Record device, OS, build/signing identity, elapsed duration and observed limits.

Log delivery does not prove real inbox delivery. SMTP acceptance remains deferred by the current provider choice; configure and check an owned inbox before claiming external email delivery. R1 remains incomplete while Google, verified-domain or required physical-device acceptance is outstanding.
