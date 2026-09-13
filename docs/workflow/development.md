---
type: reference
status: active
---

# Development workflow

All application tooling runs through Docker Compose. Do not run host pnpm, npm, Go, or other application toolchains.

## Local Development Workflow

- `docker compose up` starts the main local stack: Nginx proxy, web, api, and postgres. Open `http://localhost:3000` through the proxy.
- One-off web pnpm commands can run through `bin/web-pnpm.sh`, which uses the dependency-free `web-helper` Compose service.
- Example: `./bin/web-pnpm.sh build`
- Database migrations run through `bin/migrate.sh`, which wraps the tools-profile `migrate` Compose service, mounts `db/migrations`, and waits for the Postgres healthcheck.
- Examples: `./bin/migrate.sh up`, `./bin/migrate.sh down 1`, `./bin/migrate.sh down -all`


- API development pins Air to `v1.64.5`, which supports the Go 1.25 base image.
- API checks: `docker compose run --rm api go test ./...` and `make lint-api`.
- Database migrations are manual via `golang-migrate`; the API must never apply migrations at startup.
- Migration status: `./bin/migrate.sh version`.

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

Run [the deployment helper](../../bin/deploy.sh) with the registry password and server IP:

```sh
./bin/deploy.sh 'your-password' 203.0.113.10
PLATFORM=linux/amd64 ./bin/deploy.sh 'your-password' 203.0.113.10
```

The script resolves the repository root from its own location and performs these steps:

1. Log in to `docker-registry.kesopeso.eu` as `kesopeso` using Docker's `--password-stdin`.
2. Build both production Dockerfiles, then push `keepup-web:latest` and `keepup-api:latest`. Optional `PLATFORM` selects the build architecture.
3. Copy all regular files (including hidden files, except `.gitkeep`) directly in `db/migrations/` to `/home/keso/projects/active-sites/keepup.kesopeso.eu/db/migrations` as SSH user `keso`.
4. Run `/home/keso/projects/active-sites/keepup.kesopeso.eu/restart` from its containing directory.

The remote migrations directory must already exist. Remote migration files are not deleted. Each step must succeed before the next runs; a failure does not roll back published images or copied files. Both builds finish before either push, but pushes are sequential: if the second push fails, the first image is already published. The remote restart file must be executable and owns the server-side restart procedure.

SSH/SCP use batch mode and strict host-key checking. Authorize the SSH key for `keso` and verify/trust the server host key before running the script.

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

Supply `DATABASE_URL` at runtime; credentials are not baked into the image. Optional timing and tracker-limit settings are defined in [API config](../../apps/api/internal/config/config.go). Attach the API to the application proxy's network as `api` on port 8080. The API serves HTTP behind the proxy. Apply database migrations separately before serving application traffic; the image contains neither migration tooling nor development tools. `/livez` and `/healthz` can be probed externally; the minimal runtime contains no shell or HTTP client. Development Compose continues to use `Dockerfile.dev`.

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


Sources: [Compose services](../../docker-compose.yml), [Makefile](../../Makefile), [migration helper](../../bin/migrate.sh), [web helper](../../bin/web-pnpm.sh).
