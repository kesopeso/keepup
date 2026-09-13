#!/usr/bin/env sh
set -eu

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  echo "Usage: $0 <registry-username:registry-password> <ssh-username@host>"
  echo "Publish latest images, copy migrations, and restart KeepUp as the supplied SSH user."
  echo "Optional: PLATFORM=linux/amd64 $0 <registry-username:registry-password> <ssh-username@host>"
  exit 0
fi

if [ "$#" -ne 2 ] || [ -z "$1" ] || [ -z "$2" ]; then
  echo "Usage: $0 <registry-username:registry-password> <ssh-username@host>" >&2
  exit 1
fi

# Split credentials at the first colon so passwords may contain colons.
case "$1" in
  *:*) registry_username=${1%%:*}; registry_password=${1#*:} ;;
  *) echo "Provide registry credentials as username:password." >&2; exit 1 ;;
esac
if [ -z "$registry_username" ] || [ -z "$registry_password" ]; then
  echo "Registry username and password must both be nonempty." >&2
  exit 1
fi

case "$2" in
  *@*) ssh_username=${2%%@*}; server_host=${2#*@} ;;
  *) echo "Provide the SSH connection as username@host." >&2; exit 1 ;;
esac
# Keep the username safe in both the SSH destination and remote home path.
case "$ssh_username" in
  ""|[!a-zA-Z_]*|*[!a-zA-Z0-9_-]*)
    echo "Provide a valid SSH username." >&2
    exit 1
    ;;
esac
# Accept bare or bracketed IPv6, IPv4, and DNS hostnames.
case "$server_host" in
  \[*\]) server_host=${server_host#\[}; server_host=${server_host%\]} ;;
esac
case "$server_host" in
  ""|[!a-zA-Z0-9:]*|*[!a-zA-Z0-9.:%_-]*|:)
    echo "Provide an IPv4 address, IPv6 address, or hostname." >&2
    exit 1
    ;;
esac

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
remote_dir=/home/$ssh_username/projects/active-sites/keepup.kesopeso.eu

registry=docker-registry.kesopeso.eu
printf '%s' "$registry_password" | docker login "$registry" --username "$registry_username" --password-stdin
unset registry_password
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
scp_host=$server_host
case "$scp_host" in
  *:*) scp_host="[$scp_host]" ;;
esac
scp -o BatchMode=yes -o StrictHostKeyChecking=yes "$@" \
  "$ssh_username@$scp_host:$remote_dir/db/migrations/"

ssh -o BatchMode=yes -o StrictHostKeyChecking=yes "$ssh_username@$server_host" \
  "cd '$remote_dir' && ./restart"
