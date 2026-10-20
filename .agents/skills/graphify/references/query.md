# Focused graph queries

Read only when a direct query is noisy, empty, ambiguous, or when the user explicitly asks to save query history. Use the routine commands in SKILL.md first; no vocabulary-generation or reflection step is required.

## Narrow before expanding

Use the documented filename, path, or exact symbol rather than a broad question. Check returned `source_file` paths to distinguish identically named web/mobile helpers. Use `explain` for a symbol and `path` for named endpoints. Increase the query budget only when relevant results were truncated; truncation is not proof that omitted nodes are irrelevant.

Query text is matched lexically. If wording misses the graph's labels, try terms from the docs and source identifiers first. As a last resort, inspect a bounded subset of labels instead of loading the entire vocabulary:

```bash
./bin/graphify.sh 'python -c "import json; g=json.load(open("graphify-out/graph.json")); labels=sorted({n.get("label", "") for n in g["nodes"] if "route" in n.get("label", "").lower()}); print("\n".join(labels[:60]))"'
```

Replace the filter with a relevant term and keep the output bounded. Do not invent graph edges or infer certainty from an absent result. If the graph still does not help, use targeted source search and say which question the graph could not resolve. Do not keep retrying equivalent graph queries.

For user-supplied text, pass it as data to the inner shell:

```bash
./bin/graphify.sh 'python .agents/skills/graphify/scripts/index.py ensure && graphify query "$1" --budget 1200' graphify 'route sharing'
```

The first argument after the command is the inner shell's `$0`, and the next is `$1`.

## Optional history

Do not run `save-result`, `reflect`, generate lessons, or write vocabulary files automatically. These add writes and context to ordinary navigation and may contaminate repeated measurements. Only use them when the user requests persistent query history or reflection.

```bash
./bin/graphify.sh 'graphify save-result --question "$1" --answer "$2" --type query' graphify 'QUESTION' 'ANSWER'
```

```bash
./bin/graphify.sh 'graphify reflect --if-stale'
```

Saved answers are derived notes, not verified source facts. Keep them distinct from the code-only index. Refresh the application index before using it after an advanced mutation.
