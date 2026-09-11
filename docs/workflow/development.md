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

## Same-origin proxy

[Compose](../../docker-compose.yml) publishes Nginx on port 3000; web and API listen only inside the Compose network. The [Nginx configuration](../../apps/proxy/nginx.conf) routes requests as follows:

| Browser path | Container destination |
|---|---|
| `/api` or `/api/…` | `api:8080/…`, stripping `/api` |
| `/ws` (including query strings) | `api:8080/ws`, preserving WebSocket upgrades |
| All other paths | `web:3000`, including static assets and development hot reload |

Quiet live connections have a 24-hour proxy idle timeout. Backend REST paths remain unchanged; for example, browser `/api/routes` reaches Go `/routes` and `/api/healthz` reaches `/healthz`.

Production must provide the same routing contract behind HTTPS, with WebSocket upgrade support. The checked-in Compose file remains a development stack; when deploying the production web image, route Nginx's `web:3000` upstream to that container and `api:8080` to the separately deployed API. Configure TLS at the public proxy (and preserve the original scheme in forwarded headers if TLS terminates upstream). Build for the deployment host's architecture (for example, add `--platform linux/amd64` when needed).

Proxy forwarding follows the [Nginx WebSocket documentation](https://nginx.org/en/docs/http/websocket.html).

Standalone packaging follows the [Next.js output documentation](https://nextjs.org/docs/app/api-reference/config/next-config-js/output).

## Project Agent Skills

- `icm-architect` is installed at `.agents/skills/icm-architect` from https://github.com/RinDig/icm-architect.
- The installation is scoped to this repository and includes the upstream skill, references, assets, and license.
- Invoke it with `$icm-architect` when working in this project.


Sources: [Compose services](../../docker-compose.yml), [Makefile](../../Makefile), [migration helper](../../bin/migrate.sh), [web helper](../../bin/web-pnpm.sh).
