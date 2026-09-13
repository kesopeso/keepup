#!/usr/bin/env sh
set -eu

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  echo "Usage: $0 <registry-password> <server-ip>"
  echo "Publish latest images, copy migrations, and restart KeepUp as keso."
  echo "Optional: PLATFORM=linux/amd64 $0 <registry-password> <server-ip>"
  exit 0
fi

if [ "$#" -ne 2 ] || [ -z "$1" ] || [ -z "$2" ]; then
  echo "Usage: $0 <registry-password> <server-ip>" >&2
  exit 1
fi

# Restrict the destination to address characters, excluding SSH options and shell syntax.
case "$2" in
  *[!0-9a-fA-F.:]*|.*|:)
    echo "Provide the server's IPv4 or IPv6 address." >&2
    exit 1
    ;;
esac

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
server_ip=$2
remote_dir=/home/keso/projects/active-sites/keepup.kesopeso.eu

registry=docker-registry.kesopeso.eu
printf '%s' "$1" | docker login "$registry" --username kesopeso --password-stdin
set --

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

# Include hidden regular files, excluding .gitkeep; never copy directories.
for migration in "$repo_dir/db/migrations/"* "$repo_dir/db/migrations/".[!.]* "$repo_dir/db/migrations/"..?*; do
  [ -f "$migration" ] || continue
  [ "${migration##*/}" != .gitkeep ] || continue
  set -- "$@" "$migration"
done

# Brackets disambiguate IPv6 addresses from scp's remote-path separator.
scp_host=$server_ip
case "$scp_host" in
  *:*) scp_host="[$scp_host]" ;;
esac
scp -o BatchMode=yes -o StrictHostKeyChecking=yes "$@" \
  "keso@$scp_host:$remote_dir/db/migrations/"

ssh -o BatchMode=yes -o StrictHostKeyChecking=yes "keso@$server_ip" \
  "cd '$remote_dir' && ./restart"
