#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)
    ;;
  *)
    echo "mysql57 amd64 CI gate must run on amd64/x86_64, got: $ARCH" >&2
    echo "This gate is intentionally strict because mysql:5.7 is amd64-only and arm64 skips are not production evidence." >&2
    exit 2
    ;;
esac

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required for mysql57 amd64 CI gate" >&2
  exit 2
fi

docker info >/dev/null

./scripts/lint_mysql57_schema.sh
MOCHAT_ACCEPTANCE_SUITE=mysql57 ./scripts/standalone_acceptance.sh

SOURCE_FINGERPRINT="$(python3 ./scripts/source_fingerprint.py | python3 -c 'import json, sys; print(json.load(sys.stdin)["fingerprint"])')"
echo "源码指纹：$SOURCE_FINGERPRINT"
echo "mysql57 amd64 CI gate passed"
