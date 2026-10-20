# Application index maintenance

All commands run through the container wrapper. The standard-library-only helper `scripts/index.py` checks working-tree file contents under `apps/`, including tracked edits, deletions, renames, and untracked non-ignored files. It also checks ignore configuration, the indexing script/image configuration, the installed Graphify version, and the generated graph contents. The Git revision is recorded for diagnosis but a revision change alone does not force a rebuild.

```bash
./bin/graphify.sh 'python .agents/skills/graphify/scripts/index.py status'
```

Status exits 0 for fresh, 3 for missing/stale, and 1 for an error. It writes nothing. Output is one concise reason, not a list of files.

```bash
./bin/graphify.sh 'python .agents/skills/graphify/scripts/index.py ensure'
```

`ensure` takes an exclusive file lock, rechecks freshness, and invokes `graphify extract . --code-only` only when needed. The first run without a stamp refreshes an existing graph once. Graphify reuses its own extraction cache. A successful stable run records `graphify-out/.keepup-index.json` atomically. Concurrent callers wait on the same `graphify-out/.update.lock`; readers do not take this writer lock. Keep queries separate from known active writers.

```bash
./bin/graphify.sh 'python .agents/skills/graphify/scripts/index.py refresh'
```

`refresh` runs extraction even when the stamp is fresh. It uses Graphify's `--force` to accept legitimate shrinkage after code deletion or scope changes. Use it for explicit rebuilds or recovery, not after every query.

A failed extraction, missing/malformed graph, or source change during extraction invalidates the stamp and returns an error. The graph may still exist, but must not be reported fresh. Retry once edits have settled; do not loop indefinitely during concurrent source edits. Source remains the authority.

Generated files and stamps stay under ignored `graphify-out/`. Do not commit or hand-edit them. Untracked files ignored by Git, such as dependencies and build output, do not trigger a refresh; tracked file edits remain part of the fingerprint. Optional extensions and semantic workflows remain in the advanced references; the helper maintains the application code graph only.
