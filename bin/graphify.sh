#!/usr/bin/env sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
image=keepup-graphify:latest

if [ "$#" -eq 0 ]; then
  printf '%s\n' 'Usage: ./bin/graphify.sh '\''COMMAND'\'' [ARG0 ARG...]' >&2
  exit 2
fi

if ! docker image inspect "$image" >/dev/null 2>&1; then
  docker build --tag "$image" --file "$project_root/Dockerfile.graphify" "$project_root"
fi

# Linked worktrees use an absolute .git pointer. Expose only their common
# metadata at that path so Git inside the container can resolve it.
if [ -f "$project_root/.git" ]; then
  common_git_dir=$(git -C "$project_root" rev-parse --path-format=absolute --git-common-dir)
  git_mount=$common_git_dir
else
  git_mount=
fi

if [ -n "$git_mount" ]; then
  exec docker run --rm --init \
    --entrypoint sh --user "$(id -u):$(id -g)" --workdir /workspace \
    --env GRAPHIFY_OUT=/workspace/graphify-out \
    --mount "type=bind,source=$project_root,target=/workspace" \
    --mount "type=bind,source=$git_mount,target=$git_mount,readonly" \
    "$image" -c "$@"
fi

exec docker run --rm --init \
  --entrypoint sh \
  --user "$(id -u):$(id -g)" \
  --workdir /workspace \
  --env GRAPHIFY_OUT=/workspace/graphify-out \
  --mount "type=bind,source=$project_root,target=/workspace" \
  "$image" -c "$@"
