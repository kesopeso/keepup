#!/usr/bin/env sh

if [ -t 0 ]; then
  docker compose run --rm -it mobile-helper pnpm --filter "@keepup/mobile" "$@"
else
  docker compose run --rm mobile-helper pnpm --filter "@keepup/mobile" "$@"
fi
