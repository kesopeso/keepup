---
name: graphify
description: Explore KeepUp application code relationships, callers, dependencies, and change impact after documentation routing; maintain the apps code graph after edits. Use targeted source search for files outside apps or questions the graph cannot answer.
---

# Graphify for KeepUp

Start with the relevant project docs for product rules, ownership, and documented paths or symbols. Use the graph for application-code relationships, then read the specific source sections needed to verify behavior or edit code. Source and configuration are authoritative; a missing edge is not proof of no dependency. Product or workflow questions already answered by docs need no graph query.

## Container execution

Run every executable command through `./bin/graphify.sh '<actual call>'` from the repository root, including Python and shell utilities. Do not invoke host Graphify/Python, bypass the wrapper with Docker, or install host dependencies. The wrapper builds the pinned image when missing and starts a disposable container with the writable checkout at `/workspace`. Use container `python`; do not discover or save a host interpreter path.

Pass one quoted shell program. Keep substitutions, pipelines, redirections, and dependent commands inside that argument. Escape embedded apostrophes for the host shell; quote user values for the inner shell too. For arbitrary query text, use the wrapper's positional arguments instead of interpolating it into shell code. Exports and variables last one call; only files under `/workspace` persist. Host environment variables and stdin are not forwarded. Native agent file tools map `/workspace/...` to the actual checkout; executable calls by subagents follow the same wrapper rule.

Keep the repository root as the scan anchor. `.graphifyignore` restricts indexing to `apps/`, and output stays in `graphify-out/`. Do not switch the root to `apps/`. Optional dependencies belong in the image and are not installed automatically.

## Routine navigation

Ensure freshness and run a bounded query in one container call:

```bash
./bin/graphify.sh 'python .agents/skills/graphify/scripts/index.py ensure && graphify query "JoinRouteScreen" --budget 1200'
```

Use documented symbols or paths first. For a named entity or two endpoints:

```bash
./bin/graphify.sh 'python .agents/skills/graphify/scripts/index.py ensure && graphify explain "JoinRouteScreen"'
```

```bash
./bin/graphify.sh 'python .agents/skills/graphify/scripts/index.py ensure && graphify path "NODE_A" "NODE_B"'
```

Start with a modest query budget, narrow noisy results, and expand only when the task needs more context. Duplicate names require source-path verification. Do not read the full graph, report, vocabulary, or source tree by default. If a direct query fails, consult [query guidance](references/query.md); do not rebuild a fresh graph just because a query is weak.

For files outside `apps/`, use documented paths and targeted source search. If refreshing fails, report the limitation and use targeted source verification without claiming the graph is fresh.

## Index maintenance

After each coherent batch of changes under `apps/`, refresh before reporting completion. Refresh again after later edits. `ensure` rebuilds only if needed; it checks current working-tree contents, including uncommitted changes, and coordinates writers. A status check is read-only:

```bash
./bin/graphify.sh 'python .agents/skills/graphify/scripts/index.py status'
```

For a forced refresh, recovery, status meanings, and writer coordination, read [index maintenance](references/index.md). Do not save answers, generate reflections, or change the graph during ordinary queries. Query history is opt-in, as described in [query guidance](references/query.md).

## Advanced tasks only

- Semantic extraction, full reports, or custom builds: [full pipeline](references/full-pipeline.md), then only the relevant reference it names.
- Explicit incremental semantic updates or reclustering: [advanced updates](references/update.md).
- Explicit exports or benchmarks: [exports](references/exports.md).
- Requested URL ingestion or watch mode: [ingestion/watch](references/add-watch.md).
- Requested cloning/merging: [clone/merge](references/github-and-merge.md).
- Requested hooks or agent instruction generation: [hooks](references/hooks.md).

Advanced operations can invalidate the freshness stamp; run `ensure` before relying on the application-code index again. Do not widen the code-only scope or add semantic extraction, MCP, hooks, or external services without a relevant user request.
