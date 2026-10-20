---
type: contract
status: active
---

# Workflow shelf

Own development and knowledge-maintenance instructions.

## Inputs

Choose a workflow and read only its relevant section:

| Task | Read |
|---|---|
| Setup, web/API checks, migrations, LAN access, deployment | [Development](development.md) |
| Android build, Metro, debugging, native review | [Android development](mobile-development.md) |
| Code exploration after documentation routing | [Code navigation](documentation.md#code-navigation) |
| Scoped context loading and automatic ICM maintenance | [Documentation policy](documentation.md) |

## Contributor entry

Start with [agent routing](../../AGENTS.md) and the [context map](../CONTEXT.md). Use the chosen workflow, the matching [change-impact row](../system/change-impact.md) before editing code, and [current status](../planning/status.md) only to select or assess current work.

## Process

Follow the chosen reference's source links for the command or configuration being changed. Load Compose, helpers, proxy configuration, or native app configuration only when that workflow depends on them. Verify commands against source before editing the owning reference.

## Outputs

Update the relevant workflow reference and its entry links.

## Human check

Check that commands exist, tooling stays in Docker, and migrations remain manual.
