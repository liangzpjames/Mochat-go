#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
CHECK="$ROOT/scripts/dev_check.sh"

if "$CHECK" unknown >/dev/null 2>&1; then
  echo "dev_check accepted an unknown command" >&2
  exit 1
fi

output="$("$CHECK" help)"
echo "$output" | grep -q 'quick'
echo "$output" | grep -q 'build'
echo "$output" | grep -q 'standalone'

echo "dev check command self-test passed"
