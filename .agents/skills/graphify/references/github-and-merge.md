# graphify reference: GitHub clone and cross-repo merge

Load this when the user passed one or more `https://github.com/...` URLs, or named several local subfolders to merge into one graph.

### Step 0 - Clone GitHub repo(s) (only if a GitHub URL was given)

The wrapper's `/tmp` home is disposable. Clone, copy the checkout into an explicitly chosen directory under `/workspace/graphify-out/`, and inspect it within the same wrapped invocation before exiting. Do not rely on `~/.graphify/repos/` surviving the container. Variables such as `LOCAL_PATH` live only inside the wrapped command. For separate output graphs, set `GRAPHIFY_OUT` inside each command to its own absolute path under `/workspace/graphify-out/`; the launcher's default otherwise sends all outputs to the same root directory. KeepUp's default scan scope remains `apps/`.

**Single repo:**
```bash
./bin/graphify.sh 'LOCAL_PATH=$(graphify clone <github-url> [--branch <branch>])
# Use LOCAL_PATH as the target for all subsequent steps'
```

**Multiple repos (cross-repo graph):**
```bash
./bin/graphify.sh '# Clone each repo, run the full pipeline on each, then merge
graphify clone <url1>   # → ~/.graphify/repos/<owner1>/<repo1>
graphify clone <url2>   # → ~/.graphify/repos/<owner2>/<repo2>
# Run /graphify on each local path to produce their graph.json files
# Then merge:
graphify merge-graphs \
  ~/.graphify/repos/<owner1>/<repo1>/graphify-out/graph.json \
  ~/.graphify/repos/<owner2>/<repo2>/graphify-out/graph.json \
  --out graphify-out/cross-repo-graph.json'
```

Graphify clones into `~/.graphify/repos/<owner>/<repo>` inside the container; only a copy placed under `/workspace/` persists between calls. Each node in the merged graph carries a `repo` attribute so you can filter by origin.

**Multiple local subfolders (monorepo or multi-service layout):**

The skill pipeline writes all intermediate and final outputs to `graphify-out/` in the current working directory. Running the skill on each subfolder separately will clobber the same output dir. Instead, explicitly override the output directory for each scan:

```bash
./bin/graphify.sh 'GRAPHIFY_OUT=/workspace/graphify-out/core graphify extract ./core/     # → graphify-out/core/graph.json
GRAPHIFY_OUT=/workspace/graphify-out/service graphify extract ./service/  # → graphify-out/service/graph.json
GRAPHIFY_OUT=/workspace/graphify-out/platform graphify extract ./platform/ # → graphify-out/platform/graph.json
# Add --backend gemini|kimi|openai|deepseek|claude-cli depending on which API key you have set

# Then merge at the project root:
graphify merge-graphs \
  graphify-out/core/graph.json \
  graphify-out/service/graph.json \
  graphify-out/platform/graph.json \
  --out graphify-out/graph.json'
```

Once `graphify-out/graph.json` exists, the fast path above takes over: any codebase question runs `./bin/graphify.sh 'graphify query'` directly on the merged graph — no re-extraction, no size gate.
