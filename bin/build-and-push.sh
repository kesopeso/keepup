#!/usr/bin/env sh
set -eu

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  echo "Usage: $0 <password>"
  echo "Log in, build, and push KeepUp web and API images tagged latest."
  echo "Optional: PLATFORM=linux/amd64 $0 <password>"
  exit 0
fi

if [ "$#" -ne 1 ] || [ -z "$1" ]; then
  echo "Usage: $0 <password>" >&2
  exit 1
fi

registry=docker-registry.kesopeso.eu
printf '%s' "$1" | docker login "$registry" --username kesopeso --password-stdin
set --

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

build_image() {
  app=$1
  set -- docker build
  if [ -n "${PLATFORM:-}" ]; then
    set -- "$@" --platform "$PLATFORM"
  fi
  "$@" -f "$repo_dir/apps/$app/Dockerfile" \
    -t "$registry/keepup-$app:latest" "$repo_dir"
}

# Finish both builds before publishing either image.
build_image web
build_image api

docker push "$registry/keepup-web:latest"
docker push "$registry/keepup-api:latest"
