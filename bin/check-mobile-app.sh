#!/usr/bin/env bash
set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
adb="${HOME}/Android/Sdk/platform-tools/adb"
mapfile -t devices < <("$adb" devices | awk 'NR > 1 && $2 == "device" { print $1 }')
if ((${#devices[@]} != 1)); then
  printf 'Connect exactly one ready Android device or emulator before starting Metro. Found %s.\n' "${#devices[@]}" >&2
  exit 1
fi

# Read the same package and development scheme that Expo uses for this project.
mapfile -t config < <(python3 - "$repo_root/apps/mobile/app.json" <<'PY'
import json, sys
config = json.load(open(sys.argv[1]))['expo']
print(config['android']['package'])
scheme = config.get('scheme')
if isinstance(scheme, list):
    scheme = scheme[0] if scheme else None
print(scheme or 'exp+' + config['slug'])
PY
)
if ((${#config[@]} != 2)); then
  printf 'Could not read the Android package and development scheme from apps/mobile/app.json.\n' >&2
  exit 1
fi
package=${config[0]}
scheme=${config[1]}
device=${devices[0]}
if ! installed=$("$adb" -s "$device" shell dumpsys package "$package"); then
  printf 'Could not inspect the installed Android app on %s. Check the ADB connection.\n' "$device" >&2
  exit 1
fi
if [[ "$installed" != *"expo.modules.devlauncher"* ]]; then
  printf 'The KeepUp development app is missing or a release APK is installed on %s. Run make mobile-build.\n' "$device" >&2
  exit 1
fi
if ! activity=$("$adb" -s "$device" shell cmd package resolve-activity --brief \
  -a android.intent.action.VIEW -c android.intent.category.BROWSABLE \
  -d "$scheme://expo-development-client/" "$package"); then
  printf 'Could not check the development link on %s. Check the ADB connection.\n' "$device" >&2
  exit 1
fi
if [[ "$activity" != *"$package/"* ]]; then
  printf 'The installed KeepUp app on %s cannot open %s://. Run make mobile-build to update it.\n' "$device" "$scheme" >&2
  exit 1
fi
printf 'OK: KeepUp development app on %s supports %s://\n' "$device" "$scheme"
