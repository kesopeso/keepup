#!/usr/bin/env sh

if [ -t 0 ]; then
  docker compose run --rm -it web-helper pnpm --filter "@keepup/web" "$@"
else
  docker compose run --rm web-helper pnpm --filter "@keepup/web" "$@"
fi
