#!/bin/sh

set -euo pipefail

cd /app
go mod tidy

exec "$@"
