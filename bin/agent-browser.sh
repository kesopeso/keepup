#!/usr/bin/env sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
image=keepup-agent-browser:latest
project_id=$(printf '%s' "$project_root" | cksum | cut -d ' ' -f 1)
container=keepup-agent-browser-$project_id

# Container cleanup is separate from closing individual browser sessions.
if [ "${1:-}" = terminate ]; then
  if [ "$#" -ne 1 ]; then
    printf '%s\n' 'Usage: ./bin/agent-browser.sh terminate' >&2
    exit 2
  fi
  if docker container inspect "$container" >/dev/null 2>&1; then
    docker stop "$container" >/dev/null
    docker rm "$container" >/dev/null
  fi
  exit 0
fi

if ! docker image inspect "$image" >/dev/null 2>&1; then
  docker build --tag "$image" --file "$project_root/Dockerfile.agent-browser" "$project_root"
fi

# Keep the daemon and its browser alive between CLI calls.
if ! docker container inspect "$container" >/dev/null 2>&1; then
  docker create --name "$container" --init \
    --network host \
    --entrypoint sleep \
    "$image" infinity >/dev/null
fi

docker start "$container" >/dev/null

exec docker exec --interactive "$container" agent-browser "$@"
