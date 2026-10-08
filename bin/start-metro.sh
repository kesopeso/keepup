#!/usr/bin/env bash
set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

./bin/check-mobile-app.sh

./bin/mobile-pnpm.sh exec expo start --dev-client --localhost --android --port 8081 |
  awk '
    { print; fflush() }
    !printed && /Waiting on http:\/\/(localhost|127\.0\.0\.1):8081/ {
      print ""
      print "React Native DevTools: http://127.0.0.1:8081/open-debugger"
      fflush()
      printed = 1
    }
  '
