#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_ROOT_DIR="$(dirname "$SCRIPT_DIR")/frontend"

docker run --rm -it --name keepup.frontend.scripts.npm \
    --user $(id -u):$(id -g) \
    --volume "$FRONTEND_ROOT_DIR":/app \
    --workdir /app \
    node:24-alpine \
        "$@"
