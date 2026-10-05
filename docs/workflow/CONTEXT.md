---
type: contract
status: active
---

# Workflow shelf

Own development and knowledge-maintenance instructions.

## Inputs

[Development](development.md), [Android development](mobile-development.md), [documentation maintenance](documentation.md).

## Contributor entry

Start with [agent routing](../../AGENTS.md) and the [knowledge-base contract](../CONTEXT.md). Use the development and documentation references above, the [change-impact guide](../system/change-impact.md) before editing code, and the [implementation plan](../implementation-plan.md) to select current work.

## Process

Read [agent rules](../../AGENTS.md), [Compose](../../docker-compose.yml), [proxy configuration](../../apps/proxy/nginx.conf), [Makefile](../../Makefile), [readiness check](../../bin/doctor.sh), [migration helper](../../bin/migrate.sh), and [web helper](../../bin/web-pnpm.sh) for workflow changes. For Android, also read the [mobile helper](../../bin/mobile-pnpm.sh), [Metro launcher](../../bin/start-metro.sh), [Dockerfile](../../apps/mobile/Dockerfile.dev), and [app configuration](../../apps/mobile/app.json).

## Outputs

Update the relevant workflow reference and its entry links.

## Human check

Check that commands exist, tooling stays in Docker, and migrations remain manual.
