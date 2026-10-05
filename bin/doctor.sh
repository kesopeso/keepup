#!/usr/bin/env sh
set -u

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root" || exit 1

mode=${1:-base}
case "$mode" in
  base|mobile) ;;
  *) printf 'Usage: %s [base|mobile]\n' "$0" >&2; exit 2 ;;
esac

failures=0
ok() { printf 'OK: %s\n' "$1"; }
fail() { printf 'Missing: %s\n' "$1" >&2; failures=$((failures + 1)); }

if command -v docker >/dev/null 2>&1; then
  ok 'Docker CLI is installed'
  if docker compose version >/dev/null 2>&1; then
    ok 'Docker Compose is available'
    if docker compose config --quiet >/dev/null 2>&1; then
      ok 'Compose configuration is valid'
    else
      fail 'Compose configuration is invalid; run docker compose config for details'
    fi
  else
    fail 'Docker Compose is unavailable; install the Docker Compose plugin'
  fi
  if docker info >/dev/null 2>&1; then
    ok 'Docker daemon is accessible'
  else
    fail 'Docker daemon is inaccessible; start Docker and check user access'
  fi
else
  fail 'Docker CLI is unavailable; install Docker Engine and Compose'
fi

if [ "$mode" = mobile ]; then
  if [ "$(uname -s)" = Linux ] && [ "$(uname -m)" = x86_64 ]; then
    ok 'Host is Linux x86_64'
  else
    fail 'This Android container workflow is supported on Linux x86_64'
  fi

  sdk_dir=${HOME:-}/Android/Sdk
  if [ -d "$sdk_dir" ]; then
    ok "Android SDK exists at $sdk_dir"
  else
    fail "Android SDK is missing at $sdk_dir; install it with Android Studio or update the Compose mount"
  fi

  for component in platforms/android-36 build-tools/36.0.0 ndk/27.1.12297006; do
    if [ -d "$sdk_dir/$component" ]; then
      ok "Android SDK $component is installed"
    else
      fail "Android SDK $component is missing; install it in Android Studio SDK Manager"
    fi
  done

  if [ -x "$sdk_dir/cmdline-tools/latest/bin/sdkmanager" ]; then
    ok 'Android SDK Command-line Tools are installed'
  else
    fail 'Android SDK Command-line Tools are missing; install them in Android Studio SDK Manager'
  fi

  if [ -x "$sdk_dir/platform-tools/adb" ]; then
    ok 'Android SDK Platform-Tools are installed'
    if "$sdk_dir/platform-tools/adb" devices 2>/dev/null | awk 'NR > 1 && $2 == "device" { found = 1 } END { exit !found }'; then
      ok 'ADB sees a running Android device or emulator'
    else
      fail 'ADB sees no ready device; start an emulator in Android Studio Device Manager'
    fi
  else
    fail 'Android SDK Platform-Tools are missing; install them in Android Studio SDK Manager'
  fi

  if [ -r /dev/kvm ] && [ -w /dev/kvm ]; then
    ok 'KVM acceleration is accessible'
  else
    printf 'Note: /dev/kvm is unavailable; check virtualization and KVM group access if the emulator is slow.\n'
  fi
fi

if [ "$failures" -gt 0 ]; then
  printf '\n%d setup check(s) need attention.\n' "$failures" >&2
  exit 1
fi

printf '\nSetup checks passed.\n'
