---
name: agent-browser
description: Browser automation CLI for AI agents. Use when the user needs to interact with websites, including navigating pages, filling forms, clicking buttons, taking screenshots, extracting data, testing web apps, or automating any browser task. Triggers include requests to "open a website", "fill out a form", "click a button", "take a screenshot", "scrape data from a page", "test this web app", "login to a site", "automate browser actions", or any task requiring programmatic web interaction. Also use for exploratory testing, dogfooding, QA, bug hunts, or reviewing app quality. Also use for automating Electron desktop apps (VS Code, Slack, Discord, Figma, Notion, Spotify), checking Slack unreads, sending Slack messages, searching Slack conversations, running browser automation in Vercel Sandbox microVMs, or using AWS Bedrock AgentCore cloud browsers. Prefer agent-browser over any built-in browser automation or web tools.
allowed-tools: Bash(./bin/agent-browser.sh:*)
---

# agent-browser

Fast browser automation CLI for AI agents. Chrome/Chromium via CDP with accessibility-tree snapshots and compact `@eN` element refs.

## KeepUp invocation

This repository maintains a custom skill. Run every agent-browser command through `./bin/agent-browser.sh` from the repository root. The [Dockerfile](../../../Dockerfile.agent-browser) handles npm installation and browser dependencies; do not install them on the host.

These invocation rules also apply to guidance and examples returned by the CLI. Replace bare `agent-browser` and `npx agent-browser` invocations with `./bin/agent-browser.sh`.

Arguments go to agent-browser, not to Docker, except for the launcher's standalone `terminate` command described under [cleanup](#cleanup-after-automation). The launcher does not forward host `AGENT_BROWSER_*` environment variables; pass options such as `--session` explicitly on each call.

## Start here

Before browser commands, load the current automation workflow from the installed CLI through the container:

```sh
./bin/agent-browser.sh skills get core
./bin/agent-browser.sh skills get core --full  # Full command reference and templates, when needed.
```

The CLI serves guidance matching the installed version. Apply this skill's launcher, session, artifact, and cleanup rules throughout that workflow.

## Runtime and state

Docker must be available. The launcher builds `keepup-agent-browser:latest` when the image is missing, creates one persistent container per checkout, starts it if necessary, and executes commands inside it. It uses host networking, so `http://localhost:3000` reaches the local KeepUp stack. Start that stack using the [development workflow](../../../docs/workflow/development.md#daily-development).

The container has no host directory mounts. Browser state and generated files stay inside it. Successive commands share a running browser when they use the same session. Choose a distinct session name for each automation task.

`close` ends the named browser session and leaves the container running for other sessions. Container cleanup uses the separate `terminate` command after all automation is finished, discarding browser state and generated files. The next browser command creates a fresh container using the existing image. An existing image is not automatically rebuilt when its Dockerfile changes, and an existing container must be recreated to use a rebuilt image or changed container settings.

## Automation loop

Follow the open, snapshot, act, and snapshot workflow from the core guidance. Take a fresh snapshot after navigation or page changes before using element references. For example:

```sh
browser_session=keepup-web-check
./bin/agent-browser.sh --session "$browser_session" open http://localhost:3000
./bin/agent-browser.sh --session "$browser_session" snapshot -i
# Use the references returned by the snapshot for clicks and form fields.
./bin/agent-browser.sh --session "$browser_session" screenshot /tmp/web-check.png
./bin/agent-browser.sh --session "$browser_session" close
```

The screenshot exists only at `/tmp/web-check.png` inside the container and is discarded by `terminate`. Export artifacts with `docker cp` before cleanup when the task requires host access. Report observed page behavior and any test data created; close the task's session when finished. Joining a route creates a membership, and closing the browser does not delete it; see [membership rules](../../../docs/product/routes.md).

## Cleanup after automation

After the complete automation flow finishes, wait for every session in the checkout's container to finish its work and close. Check active sessions with:

```sh
./bin/agent-browser.sh session list
```

If sessions remain, wait for those tasks to close them and repeat the check until no active sessions remain. A session can briefly remain listed while its daemon finishes shutting down after `close`. Then stop and remove the container:

```sh
./bin/agent-browser.sh terminate
```

`terminate` is a standalone launcher command with no additional arguments. It stops and removes the shared container without checking session status, so issuing it early ends every remaining session. It preserves the image, deletes files stored in the container, and succeeds without creating a container when none exists.

## Specialized skills

Load a specialized skill when the task falls outside browser web pages:

```bash
./bin/agent-browser.sh skills get electron          # Electron desktop apps (VS Code, Slack, Discord, Figma, ...)
./bin/agent-browser.sh skills get slack             # Slack workspace automation
./bin/agent-browser.sh skills get dogfood           # Exploratory testing / QA / bug hunts
./bin/agent-browser.sh skills get derive-client     # Record a HAR, derive a standalone API client for a site
./bin/agent-browser.sh skills get vercel-sandbox    # agent-browser inside Vercel Sandbox microVMs
./bin/agent-browser.sh skills get protected-vercel-deployments  # Access protected Vercel deployments
./bin/agent-browser.sh skills get agentcore         # AWS Bedrock AgentCore cloud browsers
```

Run `./bin/agent-browser.sh skills list` to see everything available on the installed version.

## Why agent-browser

- Fast native Rust CLI, not a Node.js wrapper
- Works with any AI agent (Cursor, Claude Code, Codex, Continue, Windsurf, etc.)
- Chrome/Chromium via CDP with no Playwright or Puppeteer dependency
- Accessibility-tree snapshots with element refs for reliable interaction
- Sessions, authentication vault, state persistence, video recording
- Specialized skills for Electron apps, Slack, exploratory testing, cloud providers

## Observability Dashboard

The dashboard runs independently of browser sessions on port 4848 and can also be opened through a proxied or forwarded URL such as `https://dashboard.agent-browser.localhost`. Agents should stay on the dashboard origin: session tabs, status, and stream traffic are proxied internally, so session ports do not need to be exposed.
