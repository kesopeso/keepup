---
type: plan
status: planned
---

# Remove the authentication feature flags

Local issue. Priority: next implementation task. Application changes have not started.

## Purpose

Make account authentication the only supported application mode. The owner has adopted accounts for the day-zero staging deployment, so choosing anonymous route authority through an environment flag is no longer needed. Remove the flags rather than changing their defaults to true.

## Required behavior

- Remove API `AUTH_ENABLED`, its configuration field and the conditional startup path. Always initialize account services, mount authentication endpoints and use account route/WebSocket authority, including the mail/cleanup and live workers.
- Remove Android `EXPO_PUBLIC_AUTH_ENABLED`. Always launch the account application. Remove the legacy app entry and fallback branches that become unused, retaining shared map, route and location components used by account mode.
- Remove the web fallback to anonymous identity when `/auth/me` reports account authentication unavailable. Missing or expired sessions must lead to account sign-in; unavailable or misconfigured servers must show a recoverable error without granting legacy access.
- Remove obsolete legacy-mode selection from runtime wiring and update affected tests. Keep shared implementation needed by account mode; this issue does not require dropping legacy database columns or rewriting historical migrations.
- Remove the flags from Compose, environment examples, build instructions and active documentation. Update current setup instructions so a fresh development stack supplies the mandatory account origins, public URL and outbox key. Preserve dated validation records as historical evidence.
- Keep Google enablement optional through its client-ID configuration. Preserve the selected log-email provider and existing session, origin/CSRF, verification, route, sharing and revocation rules.

## Acceptance criteria

- With both flags absent, the API, web and Android use account mode. Old environment entries cannot restore anonymous mode, even if set to false.
- Missing mandatory account configuration fails API startup clearly. No configuration failure starts a legacy API instead.
- Unauthenticated route access requires sign-in; legacy member/owner tokens and legacy WebSocket frames remain rejected. Public landing, authentication/recovery pages and health endpoints retain their intended access.
- Existing account signup/login, Google login when configured, My routes, cross-client ownership/membership, live sharing and revocation behavior remain intact. Updating Android does not restore legacy saved memberships or start location capture automatically.
- Fresh local setup and a standalone Android build no longer require an authentication enablement flag. Migrations remain manual through 0007; no database reset or staging deployment is part of this implementation.
- Relevant Docker API/configuration and database integration checks, web tests/build, mobile tests/typecheck and an Android build pass. Review startup without either flag and with obsolete false values. Refresh the application graph and update owning documentation after implementation.

## Sources and change impact

Start with [system change impact](../system/change-impact.md), [web account boundaries](../system/frontend.md#account-authentication), [native account boundaries](../system/mobile.md#account-authentication) and the [account contract](../system/authentication-contract.md). Use Graphify before exploring callers and dependencies.

Primary sources are [API configuration](../../apps/api/internal/config/auth.go), [API startup](../../apps/api/cmd/api/main.go), [HTTP authority selection](../../apps/api/internal/httpapi/server.go), [web route gate](../../apps/web/app/components/route-account.tsx), [Android entry](../../apps/mobile/App.tsx), [Compose](../../docker-compose.yml), [root environment example](../../.env.example) and [mobile environment example](../../apps/mobile/.env.example).

Update [account setup and release instructions](../workflow/authentication-release.md), [development workflow](../workflow/development.md), [Android workflow](../workflow/mobile-development.md), affected product/system references and planning status. Keep account deletion, real inbox delivery, dedicated release signing and iOS as separate work.

## Validation

Issue scope was checked against current startup, configuration and client authority selection. This is planned work, not an implementation or a new test result. The earlier [R2 decision](authentication-tasks.md#r2-fresh-start-rollout-and-documentation) remains discarded; this issue is a separate code cleanup task.
