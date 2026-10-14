---
type: reference
status: active
---

# Browser automation

## Purpose

Define how agents automate browsers, inspect pages, take screenshots, and test the KeepUp web UI through Docker.

## Owning facts

### Invocation override

Use the repository's [agent-browser skill](../../.agents/skills/agent-browser/SKILL.md) for browser automation. Run every agent-browser command through `./bin/agent-browser.sh` from the repository root. This project rule overrides the skill's installation and invocation instructions, including examples returned by the CLI. Replace bare `agent-browser` and `npx agent-browser` invocations with the launcher; the Dockerfile handles npm installation and browser dependencies.

Read the installed skill, then load its current workflow through the container before browser commands:

```sh
./bin/agent-browser.sh skills get core
```

Load specialized guidance through the same launcher when needed, for example `./bin/agent-browser.sh skills get dogfood` for exploratory testing. Arguments go to agent-browser, not to Docker, except for the launcher's standalone `terminate` command described under [cleanup](#cleanup-after-automation). The launcher does not forward host `AGENT_BROWSER_*` environment variables; pass options such as `--session` explicitly on each call.

### Runtime and state

Docker must be available. The launcher builds `keepup-agent-browser:latest` when the image is missing, creates one persistent container per checkout, starts it if necessary, and executes commands inside it. It uses host networking, so `http://localhost:3000` reaches the local KeepUp stack. Start that stack using the [development workflow](development.md#daily-development).

The container has no host directory mounts. Browser state and generated files stay inside it. Successive commands share a running browser when they use the same session. Choose a distinct session name for each automation task.

`close` ends the named browser session and leaves the container running for other sessions. Container cleanup uses the separate `terminate` command after all automation is finished, discarding browser state and generated files. The next browser command creates a fresh container using the existing image. An existing image is not automatically rebuilt when its Dockerfile changes, and an existing container must be recreated to use a rebuilt image or changed container settings.

### Automation loop

Follow the skill's open, snapshot, act, and snapshot workflow. Take a fresh snapshot after navigation or page changes before using element references. For example:

```sh
browser_session=keepup-web-check
./bin/agent-browser.sh --session "$browser_session" open http://localhost:3000
./bin/agent-browser.sh --session "$browser_session" snapshot -i
# Use the references returned by the snapshot for clicks and form fields.
./bin/agent-browser.sh --session "$browser_session" screenshot /tmp/web-check.png
./bin/agent-browser.sh --session "$browser_session" close
```

The screenshot exists only at `/tmp/web-check.png` inside the container and is discarded by `terminate`. Export artifacts with `docker cp` before cleanup when the task requires host access. Report observed page behavior and any test data created; close the task's session when finished. Joining a route creates a membership, and closing the browser does not delete it; see [membership rules](../product/routes.md).

### Cleanup after automation

After the complete automation flow finishes, wait for every session in the checkout's container to finish its work and close. Check active sessions with:

```sh
./bin/agent-browser.sh session list
```

If sessions remain, wait for those tasks to close them and repeat the check until no active sessions remain. A session can briefly remain listed while its daemon finishes shutting down after `close`. Then stop and remove the container:

```sh
./bin/agent-browser.sh terminate
```

`terminate` is a standalone launcher command with no additional arguments. It stops and removes the shared container without checking session status, so issuing it early ends every remaining session. It preserves the image, deletes files stored in the container, and succeeds without creating a container when none exists.

## Sources and connections

[Launcher](../../bin/agent-browser.sh) owns image reuse, container lifecycle, argument forwarding, and networking. [Dockerfile](../../Dockerfile.agent-browser) owns Node, agent-browser, browser dependencies, and the executable path. The [skill](../../.agents/skills/agent-browser/SKILL.md) supplies the automation method; this page owns the project invocation override.

## Change impact

When changing the launcher, image, or skill integration, verify these commands and update this page. Keep routes in the [agent entry](../../AGENTS.md), [workflow contract](CONTEXT.md), and [change-impact guide](../system/change-impact.md) accurate.

## Validation

The image build and browser launch were checked. Separate launcher calls opened a test page, returned its interactive button, and retained its title. A local KeepUp route was joined and its member list read through the Dockerized browser. These checks establish the launcher and basic automation flow, not full web application coverage. Review future invocation changes against the source and repeat the open/snapshot check in one named session.
