---
type: reference
status: active
---

# Development workflow

All application tooling runs through Docker Compose. Do not run host pnpm, npm, Go, or other application toolchains.

## First-time setup

Clone the repository, install Docker Engine with the Compose plugin, and run these commands from the repository root. Docker supplies the web and API toolchains. Android Studio is only needed for mobile work.

```sh
make doctor
make setup
```

`make doctor` checks Docker access and the Compose configuration. `make setup` starts the proxy, web app, API, and PostGIS, then applies migrations through the existing manual migration helper. Open `http://localhost:3000`; the KeepUp create/join page should load. Check `http://localhost:3000/api/healthz` for an API response with `status: "ok"`. The first run builds images and downloads dependencies, so it takes longer than later starts. The local Compose database credentials are already configured; no `.env` file is required for this path.

For Android, continue with [first-time Android setup](mobile-development.md#first-time-android-setup). Developers working only on the web or API can stop here.

## Daily development

Start the existing stack with `docker compose up -d`, then open `http://localhost:3000`. Run `./bin/migrate.sh up` when a pull adds migrations. Mobile developers also start their emulator and run `make mobile` in a separate terminal to keep Metro serving the installed app. See the [Android daily workflow](mobile-development.md#daily-development) for rebuild rules.

## Web access from another device

Localhost development needs no environment file. To open the web app from another device using your computer's LAN address or a custom hostname, copy the [development environment example](../../apps/web/.env.development.local.example) from the repository root:

```sh
cp apps/web/.env.development.local.example apps/web/.env.development.local
```

Edit `KEEPUP_DEV_ORIGINS` in the copied file. Replace the example address with the server IP or hostname used in the testing device's browser URL. Multiple entries are comma-separated, for example `192.168.1.42,my-laptop.local`. Use hostnames or IPs without a scheme, port, or path. The file is ignored by Git, so each developer can keep their own values.

[Next.js configuration](../../apps/web/next.config.ts) splits the value on commas, trims entries, and removes empty entries before supplying `allowedDevOrigins`. Next.js loads the local file from the web app directory mounted by Compose. The `.example` file is a reference and is not loaded automatically. After editing the local file, restart the web service:

```sh
docker compose restart web
```

Open `http://YOUR_SERVER_HOST:3000` on the other device through the existing proxy. The device must be able to reach that address; custom hostnames also need to resolve on it. Update the local file and restart again if the browser hostname or IP changes. This setting allows Next.js development assets and hot reload from the listed hosts; native mobile API addresses are configured separately in the [Android workflow](mobile-development.md#api-connection).

HTTP access by IP supports development Start/Stop sharing using simulated positions. Browser ID generation falls back when the secure-context-only `crypto.randomUUID` method is unavailable; see [frontend boundaries](../system/frontend.md#current-frontend-foundation). `allowedDevOrigins` does not make an HTTP origin secure. Real browser geolocation requires HTTPS outside trusted local origins such as localhost; see [browser secure contexts](https://developer.mozilla.org/en-US/docs/Web/Security/Defenses/Secure_Contexts) and [geolocation requirements](https://developer.mozilla.org/en-US/docs/Web/API/Geolocation_API).

Hostname matching follows the [Next.js allowedDevOrigins documentation](https://nextjs.org/docs/app/api-reference/config/next-config-js/allowedDevOrigins).

## Onboarding verification

On a new Linux `x86_64` computer, use a fresh clone and follow [first-time setup](#first-time-setup), then [first-time Android setup](mobile-development.md#first-time-android-setup) if mobile work is needed. Confirm the web page and API health response, then join a route in the installed Android app and connect browser DevTools. Record any missing package, undocumented prompt, or failed command in this workflow before treating the guide as verified on a clean machine. Existing Docker images, SDK downloads, emulator state, and workspace dependencies can hide a missing step.

## Local Development Workflow

- `docker compose up` starts the main local stack: Nginx proxy, web, api, and postgres. Open `http://localhost:3000` through the proxy.
- One-off web pnpm commands can run through `bin/web-pnpm.sh`, which uses the dependency-free `web-helper` Compose service.
- Example: `./bin/web-pnpm.sh build`
- Android builds, emulator connections, Metro, and browser DevTools are owned by the [Android development workflow](mobile-development.md). Mobile commands use `bin/mobile-pnpm.sh` and the `mobile-helper` Compose service.
- Database migrations run through `bin/migrate.sh`, which wraps the tools-profile `migrate` Compose service, mounts `db/migrations`, and waits for the Postgres healthcheck.
- Examples: `./bin/migrate.sh up`, `./bin/migrate.sh down 1`, `./bin/migrate.sh down -all`


- API development pins Air to `v1.64.5`, which supports the Go 1.25 base image.
- API checks: `docker compose run --rm api go test ./...` and `make lint-api`.
- Database migrations are manual via `golang-migrate`; the API must never apply migrations at startup.
- Migration status: `./bin/migrate.sh version`.

## GPS validation

The API loads these optional settings at startup:

| Environment variable | Default | Meaning |
|---|---|---|
| `GPS_MAX_ACCURACY_M` | `100` | Maximum accepted horizontal accuracy radius in metres |
| `GPS_MAX_SPEED_MPS` | `400` | Maximum implied speed after subtracting accuracy radii |
| `GPS_MAX_AGE` | `2m` | Maximum sample age relative to server receive time |
| `GPS_MAX_FUTURE_SKEW` | `30s` | Maximum future client timestamp skew |

Numeric limits must be finite and greater than zero; durations must be greater than zero. Invalid settings fail startup. The current browser supplies the now-required accuracy and client timestamp fields. Existing accepted history is retained.

Run API tests with the race detector and opt-in PostGIS integration coverage:

```sh
docker compose run --rm \
  -e 'TEST_DATABASE_URL=postgres://keepup:keepup@postgres:5432/keepup?sslmode=disable' \
  api go test -race ./...
```

Integration tests create and remove isolated schemas and apply existing migration SQL there. They do not change application tables. The test database user needs schema creation privileges and access to the PostGIS and pgcrypto extensions. Without `TEST_DATABASE_URL`, database integration tests skip; unit and protocol tests still run. No new production migration is needed.

## Dev and Deployment

- One monorepo
- Everything containerized
- Local development runs via `docker compose up`
- Services:
  - proxy (Nginx)
  - web
  - api
  - postgres/postgis
- Production MVP targets a single VPS with containers
- HTTPS required in production

## Deploy to the production server

Run [the deployment helper](../../bin/deploy.sh) with registry credentials (`username:password`) and an SSH connection (`username@host`):

```sh
./bin/deploy.sh 'registry-user:your-password' deploy@203.0.113.10
PLATFORM=linux/amd64 ./bin/deploy.sh 'registry-user:your-password' deploy@203.0.113.10
```

The script resolves the repository root from its own location and performs these steps:

1. Log in to `docker-registry.kesopeso.eu` as the supplied registry user using Docker's `--password-stdin`.
2. Build both production Dockerfiles, then push `keepup-web:latest` and `keepup-api:latest`. Optional `PLATFORM` selects the build architecture.
3. Copy all regular files (including hidden files, except `.gitkeep`) directly in `db/migrations/` to `/home/<ssh-username>/projects/active-sites/keepup.kesopeso.eu/db/migrations` as the supplied SSH user.
4. Run `/home/<ssh-username>/projects/active-sites/keepup.kesopeso.eu/restart` from its containing directory.

Registry credentials split at the first colon; the password may contain additional colons. Both parts must be nonempty. The SSH host may be an IPv4 address, a hostname, or an IPv6 address (bare or bracketed, for example `deploy@[2001:db8::10]`). The SSH username determines the remote `/home/<ssh-username>` path.

The remote migrations directory must already exist. Remote migration files are not deleted. Each step must succeed before the next runs; a failure does not roll back published images or copied files. Both builds finish before either push, but pushes are sequential: if the second push fails, the first image is already published. The remote restart file must be executable and owns the server-side restart procedure.

SSH/SCP use batch mode and strict host-key checking. Authorize the SSH key for the supplied SSH user and verify/trust the server host key before running the script.

> **Warning:** Passing the password as a command-line argument can leave it in your shell history and expose it in process arguments. This script accepts that tradeoff; `--password-stdin` keeps the password out of the Docker login command's arguments.

## Production web image

The [web Dockerfile](../../apps/web/Dockerfile) uses Node 24 Alpine, a frozen pnpm lockfile, and Next.js standalone output. Its final stage contains the traced server dependencies and static/public assets and runs as the non-root `node` user on port 3000. Build from the repository root so the workspace manifests are available.

Replace the registry and tag in these commands:

```sh
docker build -f apps/web/Dockerfile \
  -t registry.example.com/keepup/web:1.0.0 .
docker login registry.example.com
docker push registry.example.com/keepup/web:1.0.0
```

The image has no environment-specific API or WebSocket URL settings. Browser REST requests use `/api`; live connections derive `ws://` or `wss://` from the page origin and use `/ws`. The same image can be promoted across environments without rebuilding for a different hostname. Environment files are excluded from the build context.

## Production API image

The [API Dockerfile](../../apps/api/Dockerfile) builds a static binary with Go 1.25 Alpine and copies it into a `scratch` runtime with CA certificates for outbound TLS, including encrypted database connections. It runs as non-root UID/GID `65532`, defaults to `APP_ENV=production` and `APP_PORT=8080`, and starts the binary directly so shutdown signals reach the API.

Build from the repository root, replacing the registry and tag:

```sh
docker build -f apps/api/Dockerfile \
  -t registry.example.com/keepup/api:1.0.0 .
docker push registry.example.com/keepup/api:1.0.0
```

Supply `DATABASE_URL` at runtime; credentials are not baked into the image. WebSocket keepalives default to a `30s` ping interval and `10s` pong timeout, configurable with `WEBSOCKET_PING_INTERVAL` and `WEBSOCKET_PING_TIMEOUT`. Other optional timing and tracker-limit settings are defined in [API config](../../apps/api/internal/config/config.go). Attach the API to the application proxy's network as `api` on port 8080. The API serves HTTP behind the proxy. Apply database migrations separately before serving application traffic; the image contains neither migration tooling nor development tools. `/livez` and `/healthz` can be probed externally; the minimal runtime contains no shell or HTTP client. Development Compose continues to use `Dockerfile.dev`.

## Same-origin proxy

[Compose](../../docker-compose.yml) publishes Nginx on port 3000; web and API listen only inside the Compose network. The [Nginx configuration](../../apps/proxy/nginx.conf) routes requests as follows:

| Browser path | Container destination |
|---|---|
| `/api` or `/api/…` | `api:8080/…`, stripping `/api` |
| `/ws` (including query strings) | `api:8080/ws`, preserving WebSocket upgrades |
| All other paths | `web:3000`, including static assets and development hot reload |

Quiet live connections have a 24-hour proxy idle timeout. Backend REST paths remain unchanged; for example, browser `/api/routes` reaches Go `/routes` and `/api/healthz` reaches `/healthz`.

Production must provide the same routing contract behind HTTPS, with WebSocket upgrade support. The checked-in Compose file remains a development stack; when deploying the production web image, route Nginx's `web:3000` upstream to that container and `api:8080` to the separately deployed API. The production public `nginxproxy/nginx-proxy` terminates TLS using certificates managed by ACME companion. Mount [nginx.prod.conf](../../apps/proxy/nginx.prod.conf) at `/etc/nginx/conf.d/default.conf` in the application proxy; development Compose continues to mount `nginx.conf`. The production application proxy listens on HTTP port 80 and preserves the public proxy's `X-Forwarded-Proto` header instead of replacing it with its internal HTTP scheme. Keep this application proxy accessible only through the trusted public proxy on the container network, since it trusts that forwarded header. Certificates and HTTPS redirects belong to the public proxy; the `api:8080` and `web:3000` upstreams remain HTTP. Build for the deployment host's architecture (for example, add `--platform linux/amd64` when needed).

Proxy forwarding follows the [Nginx WebSocket documentation](https://nginx.org/en/docs/http/websocket.html).

Standalone packaging follows the [Next.js output documentation](https://nextjs.org/docs/app/api-reference/config/next-config-js/output).

## Project Agent Skills

- `icm-architect` is installed at `.agents/skills/icm-architect` from https://github.com/RinDig/icm-architect.
- The installation is scoped to this repository and includes the upstream skill, references, assets, and license.
- Invoke it with `$icm-architect` when working in this project.


Sources: [Compose services](../../docker-compose.yml), [Makefile](../../Makefile), [readiness check](../../bin/doctor.sh), [migration helper](../../bin/migrate.sh), [web helper](../../bin/web-pnpm.sh).
