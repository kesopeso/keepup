---
type: plan
status: specified
---

# Account authentication PRD

## Purpose and review status

Replace anonymous device identity with persistent accounts shared by web and Android. Product requirements and the remaining proposed defaults were accepted for implementation on 2026-10-21. [ADR 0001](../adr/0001-account-authentication.md) and the [shared contract](../system/authentication-contract.md) define the approach. B2–B6 supply disabled-by-default account/Google authentication, account route authorization, My routes and socket bootstrap APIs; the complete product is not yet implemented. The existing [membership rules](routes.md) describe legacy mode used by current clients while the flag is false. Multiple viewers, exclusive sharing and immediate live cleanup are implemented; web authentication/account settings are implemented; web route integration and native clients remain pending.

## Outcomes and scope

Users sign in once, see their chosen username, and recover joined routes and ownership on another device. Every route participant must have an active account, including archive viewers. The public landing page and authentication/recovery pages remain accessible without login. Route passwords and route sharing policies still apply after account authentication.

Support email/password and Google on web and Android. All accounts have a password, including accounts created through Google. No anonymous account, membership, ownership, or historical route data needs migration. A fresh-start deployment will invalidate legacy credentials; this planning task does not delete any data.

Email and username changes are out of scope with no planned implementation. Account deletion is deferred to a separate task. Other identity providers, MFA, device takeover, and iOS are outside this release.

## Identity and activation

- A permanent internal user ID is the database primary key. Email and username each have a database-enforced, case-insensitive unique constraint.
- Email and username are immutable. Preserve username casing for all display, while accepting any casing at login. Email comparison does not remove dots or plus suffixes.
- Username length is 3–24 characters, using ASCII `A–Z`, `a–z`, `0–9`, `-`, `_`. This keeps normalization consistent across clients and the database.
- Passwords have 8–128 Unicode characters, one ASCII digit, and a punctuation/symbol character. Whitespace does not count as the special character. No mandatory uppercase or lowercase rule. Allow password-manager paste/autofill without trimming, normalization, or truncation.
- Email/password signup, new Google account setup, and password reset include a Confirm password field. Require an exact match before submission and show an inline mismatch error. Confirmation is a client-side input check, not a separately stored credential.
- Email/password registration creates a pending account. Pending accounts may verify, resend, recover access, and log out, but cannot create, join, view, or track routes.
- Verification activates the account. Its default lifetime is 24 hours and is configurable. Verification links are single-use; resending replaces the previous link. Sending and attempts are rate-limited with configurable limits.
- Opening a verification link activates the account but does not sign in the device opening it. The original authenticated pending signup session can observe activation and resume the intended destination. Without that session, login is required.

## Google signup and login

New Google users complete a form with username and password. The email comes from the validated Google identity and is not editable. Do not create a usable account until the form succeeds; username collisions retain the pending flow and show a field error.

An existing password account can sign in through Google using the same proven email without entering its KeepUp password. Preserve its username and password. This is linking, not a second account. Future Google logins resolve the linked provider subject to the existing user ID.

Google-authoritative verified email activates the account without a KeepUp verification message. For Google accounts with a third-party email that Google does not authoritatively manage, fresh KeepUp email verification is required before first linking or activation, based on the [Google backend guidance](https://developers.google.com/identity/sign-in/web/backend-auth). It does not require the existing KeepUp password. Once linked, the verified Google subject identifies the user even if Google's profile email later changes; the KeepUp email remains fixed.

Handle Google cancellation, unavailable service, invalid credentials, expired signup context, and a provider identity already linked to another account without silently overwriting identity or password.

## Login, sessions, and recovery

- Password login accepts either email or username, case-insensitively. Password comparison is exact.
- Sessions survive browser/app restarts and expire after 30 days of inactivity. Activity is authenticated API use or authenticated live participation, not merely launching a process. The backend owns this configurable timeout.
- Preserve every intended protected destination through login, signup, Google redirects, and verification. Persist navigation intent, never passwords. Only allow approved in-app destinations. After authentication, recheck access, route existence, route password requirements, and route state.
- With no destination, open My routes. Existing members regain their membership and owner authority without rejoining or entering the route password again.
- Forgotten-password links default to one hour, configurable, and are single-use. Issuing a new reset link invalidates older reset links. Reset success revokes all sessions and stops their sharing, then returns to login. Resetting a password alone does not verify a pending account.
- Password changes require the current password. Retain and rotate the current session, revoke other sessions, and stop their sharing. The current device may continue its own sharing.
- Logout revokes the current session and stops sharing associated with it. Logout everywhere revokes all sessions and stops all sharing. Neither action leaves route memberships or removes ownership.
- Use generic login/recovery responses where necessary to avoid exposing whether an email is registered. Show actionable field validation, expired-link, resend-delay, offline, and delivery-failure states without exposing credentials.

## Account membership and concurrent devices

Use the account username as the display name. Do not ask for a route alias during create/join. Transport selection and protected-route passwords retain their existing roles. Each account has one current participation identity per route; devices do not add duplicate member rows or consume extra tracking slots.

One account may create multiple routes. Multiple devices may watch the same route. For a given account and route, only one device may share location. Different devices may share concurrently on different routes. Existing sharing-policy and tracker-limit checks still apply.

A second device cannot take over. Show: "Another device is sharing on this route. Stop sharing there before starting here." The current device must stop, or its sharing connection must be declared offline by the server's presence timeout. A watching device must not keep a disconnected sharing device's lease alive. On reconnect, an old sender must not resume automatically or submit points after another device acquired sharing.

Leaving, closing, deleting, or losing session authorization releases affected sharing rights. Preserve existing route lifecycle rules, including owners being unable to leave an active route. Rejoining after an explicit leave creates a new membership episode under the same account, preserving old path history; this is distinct from restoring a still-valid membership on another device.

## My routes

Provide separate Active and Archived views. In both, show three additive checkboxes: My routes and Joined routes selected by default; Left routes unselected. My routes means owned routes, Joined routes means current non-owner memberships, and Left routes means former memberships without a current membership. Classify each route once, with current ownership/membership taking precedence over historical participation. Selecting Left routes alone shows former routes. Clearing all three shows a filter-specific empty state with a Reset filters action.

Mark owners with an Owner badge and former memberships with a Left badge. A left route remains discoverable but does not restore revoked membership access. Rejoin an active route through the normal join flow, including its password if required. For a closed left route, View archive first goes through the required access/join checks. Deleted routes disappear. Rejoining moves the route back into the current category and never duplicates a list item.

Include Create route and Join route actions. Every displayed route has a Share button on web and Android, including active, archived, and left routes. Reuse the corresponding platform route-screen sharing behavior and feedback, including native sharing and existing copy/manual-copy fallbacks. Share only the route URL, never passwords or account credentials. A cached route link can still be shared offline; sharing a link does not restore membership or bypass recipient authentication. Load membership, ownership, lifecycle, and available actions from the server. Do not embed member locations, route passwords, or privileged route details in dashboard responses to former members.

Use a responsive list with route name, code, role, protection indicator, and created date for active routes or closed date for archives. Open route for current active members; View archive for closed routes; Rejoin for left active routes. List newest created active routes first and newest closed archives first, with route ID as a stable tie-breaker. Use server pagination with a Load more action, 20 routes per page. Filter before pagination. Counts reflect the selected filters and full result set, not just the current page.

Default to Active and the default filters on a fresh visit; preserve tab, filters, and loaded position when returning from a route within the same signed-in session. Clear view state on logout/account switch. Show distinct no-routes, no-filter-matches, loading, failed-load, and offline states. Retained offline summaries are read-only and do not grant route access. The web and Android layouts completed Lavish review; see the review record below.

## Authentication UI review

The owner reviewed the initial authentication mockups in Lavish on 2026-10-21. Applied feedback: Confirm password on both signup paths and password reset; the reset logout warning directly below the screen title; an account-creation icon and Google icon; an "or" divider between email signup and Google; and username support for both hyphens and underscores. Preserve these choices in web and Android implementation. The portable [authentication UI snapshot](authentication-ui.html) preserves the reviewed mockup without requiring Lavish. It is a design reference, not application code.

The My routes review also ended with the owner marking it finished. Preserve its responsive route list, Active/Archived tabs, three additive category filters with Left routes off by default, ownership/former-membership badges, create/join/account actions, and per-route Share on both platforms. Share must reuse the corresponding route-screen behavior and feedback. The portable [My routes UI snapshot](my-routes-ui.html) includes the final Share revision, even if a review window retained an older rendered state. Loading, no-routes, no-matches, offline, and retry states are included. Opening a route never starts sharing.

Visual review established the layouts. Subsequent implementation authorization and acceptance of defaults established credential validation, ordering, pagination, retained list state, and former-member access behavior. The [task handoff](../planning/authentication-tasks.md) records delivered and remaining work separately.

## Email delivery

Authentication depends on an email delivery interface, not a particular provider. Implement SMTP first, configured with host, port, credentials, sender, and TLS mode. Third-party API providers can implement the same interface. Select the implementation explicitly; SMTP settings can select SMTP when no provider override is configured. Incomplete/invalid configurations must fail validation, not silently drop messages.

Verification/reset lifetimes, resend cooldowns, and rate limits are configurable. SMTP credentials remain deployment secrets. Gmail SMTP is a possible deployment choice, subject to its account authentication requirements and delivery limits; provider setup and a real delivery test are separate from this proposal. No email is sent by this planning work.

## Acceptance criteria

1. Concurrent registrations cannot create case-variant duplicates of email or username. Display retains the chosen username casing. Neither identifier has an update API.
2. Both login identifiers work in mixed case; invalid password formats fail signup/change/reset consistently on backend and clients. Both signup forms and password reset block mismatched password confirmation.
3. Pending accounts cannot use route REST or WebSocket capabilities. Valid verification activates once; expired, superseded, and replayed links fail safely. Opening on another device never signs it in.
4. Google signup requires username and password. Existing same-email accounts are linked under the ownership checks above without password replacement or duplicate users. Invalid token audience, issuer, expiry, or signature is rejected.
5. Password recovery, change, logout, and logout everywhere revoke the specified sessions and stop their affected live sharing. Revoked sessions cannot recover access through old route tokens or an already-open socket.
6. Login/signup/verification restore a permitted protected destination; external redirect attempts are rejected. Missing or forbidden destinations show a useful fallback to My routes.
7. Joining on another device restores the same current membership and ownership. Simultaneous Start requests yield exactly one sharing device for an account/route. Other devices continue watching.
8. Sharing on two different routes from two devices works. An unrelated viewer does not prevent timed-out sharing from being released. Late packets from the previous sender are rejected.
9. My routes shows both lifecycle views and three additive filters, with Left routes off by default. It marks ownership and former membership, avoids duplicates, rechecks access on rejoin/archive open, and handles empty, loading, offline, and failed requests.
10. SMTP and a fake provider satisfy the same interface. Delivery failure does not activate an account. Configurable expiry/rate-limit boundaries and link replacement are tested.
11. Web and Android support accessible forms, autofill, password visibility, loading/error states, persistent sessions, and their respective redirect/app-link flows. Cross-client tests verify the same account and sharing restrictions.

## Independent delivery tasks

The [implementation task checklist](../planning/authentication-tasks.md) splits these groups into independently verifiable changes with dependencies and completion criteria.

### AUTH-01: Shared backend and contract

Depends on accepted PRD/ADR. Own account schema, constraints, password hashing, session lifecycle, Google validation/linking, verification/reset, SMTP abstraction, route authorization, account memberships, My routes API, and exclusive per-route sharing. Publish request/response/error and live-event contracts plus deterministic tests and sample fixtures. Include concurrency, revocation, timeouts, malicious redirects, and provider failure cases. No client UI dependency.

### AUTH-02: Web integration

Depends on AUTH-01's agreed contract; UI work can use its fixtures before the backend is deployed. Own responsive authentication/verification/recovery/account settings, safe destination return, persistent cookie sessions, route username integration, concurrent viewer behavior, and My routes. Use the reviewed My routes UI snapshot and recorded feedback. Test browser restarts, cross-tab logout, Google return, invitation continuity, and multi-device sharing.

### AUTH-03: Android integration

Depends on AUTH-01's agreed contract, not AUTH-02's implementation. Own native Google integration, secure session storage, app/browser link return, auth/recovery/account settings, route username integration, My routes, and background-location shutdown on revoked access. Test app restart, cancelled Google login, verification opened elsewhere, session expiry, screen-off sharing, and cross-device contention. Use the reviewed My routes UI snapshot and recorded feedback.

### AUTH-04: Release and cross-client verification

Depends on AUTH-01 through AUTH-03. Configure Google client IDs/redirects, SMTP, trusted origins, and mobile links. Rehearse fresh-start database transition and legacy credential rejection in a disposable environment. Schedule the destructive production transition explicitly at rollout; do not wipe data as part of planning. Verify real email delivery, browser/mobile interoperability, and sharing revocation. Deploy the breaking API/client transition together or with a documented maintenance window; independent development does not imply old clients remain compatible.

## Review and evidence

Product rules above reflect planning and the owner's authorization to begin implementation with the proposed defaults. B2–B6 have database, password/session, transport, verification/recovery, queue, SMTP, Google proof/linking/onboarding, account route authorization, membership/history, safe My routes and socket bootstrap tests. B6 adds multiple-viewer, exclusive sender generation, timeout/reconnect, startup recovery and synchronous live-revocation checks. Web forms/Google SDK/link UI are implemented in W1; My routes, native integration and real Google/delivery checks remain pending. Lavish snapshots remain design references. Current source pointers and technical alternatives are in the ADR; delivery state belongs to the task handoff.
