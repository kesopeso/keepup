---
type: reference
status: active
---

# Authentication email and recovery configuration

B3 adds verification, password recovery/change, SMTP delivery and the transactional mail queue. The [account contract](../system/authentication-contract.md) owns endpoint behavior; the [task handoff](../planning/authentication-tasks.md) owns release state. Authentication remains disabled by default. This backend increment does not provide client link pages, Google sign-in, account-bound route access or live revocation yet.

## Enablement and secrets

Apply migrations 0003 and 0004 manually to the intended development database before setting `AUTH_ENABLED=true`. No application migrations or reset were performed by the implementation session. The API starts its mail/cleanup worker only when authentication is enabled; shutdown cancels and joins it before closing the database pool.

Supply `AUTH_ALLOWED_ORIGINS`, `AUTH_PUBLIC_URL`, `AUTH_OUTBOX_KEY`, and a delivery provider alongside the [base account settings](development.md#staged-account-api). Origins are exact scheme/host/port values without paths, query strings or credentials. HTTPS is required outside `APP_ENV=development`. The public origin is trusted deployment configuration; links never use an inbound Host header.

Generate 32 cryptographically random bytes in your secret manager, base64-encode them, and provide that value as `AUTH_OUTBOX_KEY`. Keep the key outside PostgreSQL and source control. It encrypts queued links with AES-256-GCM, authenticated against the token row ID. Purpose-token records hold SHA-256 digests; raw links are not stored in plaintext. Drain the queue before rotating the key, or queued ciphertext will fail closed and require a fresh resend. Terminal messages erase encrypted payloads immediately. No endpoint or log exposes queued links.

## Provider selection

If `AUTH_MAIL_PROVIDER` is absent and `SMTP_HOST` is present, the API selects SMTP. An explicit `smtp` also selects it. Unknown providers, partial SMTP settings, invalid keys and missing required settings fail startup. `fake` is allowed only with `APP_ENV=test` and records messages in memory for tests. It does not send mail or expose messages over HTTP.

The provider boundary is [maildelivery.Sender](../../apps/api/internal/maildelivery/mail.go). A third-party API implementation supplies `Send(context.Context, Message) error`, honors cancellation, and is registered explicitly in configuration and startup selection. Authentication logic and the outbox do not depend on SMTP.

| Setting | Default | Use |
|---|---|---|
| `AUTH_MAIL_PROVIDER` | auto-select SMTP from host | `smtp`, or test-only `fake` |
| `AUTH_PUBLIC_URL` | required when enabled | Public origin for `/auth/verify#token=...` and `/auth/reset-password#token=...` |
| `AUTH_OUTBOX_KEY` | required when enabled | Base64 encoding of a persistent 32-byte encryption key |
| `SMTP_HOST` | required for SMTP | Server hostname or IPv4 address |
| `SMTP_PORT` | `587` | 1–65535 |
| `SMTP_FROM` | required for SMTP | Bare sender address |
| `SMTP_USERNAME` / `SMTP_PASSWORD` | empty | Set both for SMTP PLAIN authentication over verified TLS, or neither for a trusted TLS relay |
| `SMTP_TLS_MODE` | `starttls` | Required STARTTLS, or `tls` for implicit TLS, typically port 465 |

TLS certificate verification is mandatory, with TLS 1.2 or later. STARTTLS absence is an error; credentials are never sent before encryption. Header injection is rejected. A stable Message-ID is retained across retries. Tests use a local TLS SMTP server and fake delivery; no real inbox delivery or Gmail account setup has been verified. Real provider credentials, authentication requirements and delivery limits belong to R1.

## Lifetimes, limits and worker settings

All durations use Go duration syntax, such as `24h`, `15m`, `60s`, and must be positive. Numeric limits are 1–10000. Confirmation/password fields follow the shared contract; password confirmation stays in clients and is not a second server credential.

| Setting | Default | Use |
|---|---|---|
| `AUTH_VERIFICATION_TTL` | `24h` | Email activation token lifetime |
| `AUTH_RESET_TTL` | `1h` | Password reset token lifetime |
| `AUTH_RESEND_COOLDOWN` | `1m` | Minimum interval per account and token purpose |
| `AUTH_MAIL_WINDOW` | `1h` | Mail quota window |
| `AUTH_MAIL_PER_ACCOUNT` | `5` | Verification/reset messages combined per account/window |
| `AUTH_MAIL_PER_SOURCE` | `20` | Mail requests per source/window, including unknown recovery identifiers |
| `AUTH_CONFIRM_WINDOW` | `15m` | Verification/reset consumption attempt window |
| `AUTH_CONFIRM_ATTEMPTS` | `20` | Attempts per source/window |
| `AUTH_RECOVERY_MIN_RESPONSE` | `200ms` | Minimum recovery response duration for known/unknown/suppressed requests |
| `AUTH_MAIL_RETRY_DELAYS` | `1m,5m,15m,1h,4h` | Backoff after each failed attempt; empty means no retries, maximum 20 entries |
| `AUTH_MAIL_POLL_INTERVAL` | `5s` | Poll interval; up to 50 messages per batch |
| `AUTH_MAIL_TIMEOUT` | `15s` | Bound delivery operations; also used by SMTP |
| `AUTH_CLEANUP_INTERVAL` | `1h` | Expired-record and encrypted-payload cleanup interval |
| `AUTH_RETENTION` | `168h` | Retain expired tokens/sessions/counters before deletion; must cover all rate windows |
| `AUTH_TRUSTED_PROXIES` | empty | Comma-separated CIDRs allowed to supply forwarded source addresses |

Mail requests share a per-source budget. Account quotas/cooldowns are serialized with the user lock. Forgot-password always returns the same accepted response for an absent account, an existing account or an account-specific mail cooldown/quota. Public source limits still return 429. The minimum response duration reduces routine timing differences; it is not a constant-time guarantee under overload. Login and current-password changes retain their separate attempt limits. Rate-limit responses include a conservative integer-seconds Retry-After for the applicable window/cooldown.

## Delivery and cleanup behavior

Signup writes account, pending session, verification token and encrypted outbox row in one transaction. Resending invalidates the prior token and cancels queued retries. Verification activates without creating a session for the device opening the link; a pending session can observe activation through `/auth/me`.

The worker locks the account before claiming an outbox row, using SKIP LOCKED to coordinate API processes. It holds that lock during bounded sending so token replacement/consumption cannot commit between validity checking and sending. Failed attempts retry on schedule; expired, consumed, superseded or undecryptable messages are not sent. Failure leaves the account pending and permits a later controlled resend.

Delivery is at least once. SMTP can accept a message before a database commit or acknowledgment fails, causing a duplicate email after retry. Single-use tokens still prevent replay. An email can expire while in transit. Terminal success/failure/cancellation clears the ciphertext. Cleanup removes expired token/session records and old counters after retention, preserves users, and wipes expired pending payloads.

A reset consumes its token and revokes all account sessions without verifying or signing in the account. Password change requires an active session/current password, rotates its credential while preserving session ID, revokes other sessions and invalidates outstanding reset links. B6 will connect these account revocations to live sharing/socket cleanup; existing anonymous route sockets are not account-bound yet.

## Proxy source addresses

Without trusted proxy CIDRs, the API ignores forwarded headers and uses the socket peer address. Behind the application proxy this groups clients under the proxy's budget. When a connecting peer is trusted, walk X-Forwarded-For right to left through trusted hops and use the first untrusted address. Malformed or oversized headers fall back to the socket peer.

Configure only actual trusted proxy networks reachable through the deployment's controlled ingress, including both application and public proxies where needed. Do not trust user-supplied leftmost addresses or expose the API directly while trusting arbitrary ingress. The checked-in Nginx proxies already append the connecting peer. Actual deployment CIDRs and real delivery remain R1 settings; no trust range was automatically enabled.

Source: [recovery configuration](../../apps/api/internal/config/recovery.go), [token transactions](../../apps/api/internal/auth/recovery_postgres.go), [outbox worker](../../apps/api/internal/auth/outbox.go), [SMTP provider](../../apps/api/internal/maildelivery/smtp.go).
