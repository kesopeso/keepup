# graphify reference: commit hook and native AGENTS.md integration

Load this when the user asked to install the post-commit hook or wire graphify into a project's AGENTS.md.

## For git commit hook

Install a post-commit hook that auto-rebuilds the graph after every commit. No background process needed - triggers once per commit, works with any editor.

```bash
./bin/graphify.sh 'graphify hook install'
```

Remove the hook:

```bash
./bin/graphify.sh 'graphify hook uninstall'
```

Check its status:

```bash
./bin/graphify.sh 'graphify hook status'
```

After every `git commit`, the hook detects which code files changed (via `git diff HEAD~1`), re-runs AST extraction on those files, and rebuilds `graph.json` and `GRAPH_REPORT.md`. Doc/image changes are ignored by the hook - run `/graphify --update` manually for those.

If a post-commit hook already exists, graphify appends to it rather than replacing it.

---

## For native AGENTS.md integration

Run once per project to make graphify always-on in your agent sessions:

```bash
./bin/graphify.sh 'graphify agents install'
```

This writes a `## graphify` section to the local `AGENTS.md` that instructs your agent to check the graph before answering codebase questions and rebuild it after code changes. No manual `/graphify` needed in future sessions.

```bash
./bin/graphify.sh 'graphify agents uninstall  # remove the section'
```

After generating a hook or agent instruction section, inspect its executable commands. Rewrite every Graphify/Python/shell invocation to use the absolute host path to `bin/graphify.sh` with one shell command argument before treating the integration as usable. A hook must not call a host `graphify` binary or a container Python path directly. Do not overwrite unrelated existing instructions or hooks.
