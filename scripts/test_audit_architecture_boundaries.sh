#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
AUDIT="$ROOT/scripts/audit_architecture_boundaries.sh"
TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

mkdir -p "$TMP_ROOT/internal/modules/bad/domain" "$TMP_ROOT/fake-bin"

printf '%s\n' \
  'package domain' \
  'import _ "database/sql"' \
  >"$TMP_ROOT/internal/modules/bad/domain/model.go"
printf '%s\n' \
  '{' \
  '  "productionModules": [],' \
  '  "protectedFiles": [],' \
  '  "forbiddenNewFiles": [],' \
  '  "exceptions": []' \
  '}' \
  >"$TMP_ROOT/architecture-policy.json"

if output="$(
  cd "$ROOT"
  go run ./cmd/mochat-architecture -root "$TMP_ROOT" -policy architecture-policy.json 2>&1
)"; then
  echo "architecture CLI accepted a forbidden domain dependency" >&2
  exit 1
fi
printf '%s\n' "$output" | grep -q '^ARCH-DOMAIN-DEPENDENCY '

output="$(
  cd "$ROOT"
  go run ./cmd/mochat-architecture -root . 2>&1
)"
test "$output" = "architecture boundaries passed"

output="$(sh "$AUDIT" 2>&1)"
test "$output" = "architecture boundaries passed"

printf '%s\n' \
  '#!/usr/bin/env sh' \
  'printf "%s\n" "$*" >"$MOCHAT_TEST_GO_ARGS"' \
  'printf "%s\n" "architecture boundaries passed"' \
  >"$TMP_ROOT/fake-bin/go"
chmod +x "$TMP_ROOT/fake-bin/go"

MOCHAT_TEST_GO_ARGS="$TMP_ROOT/go.args" \
  PATH="$TMP_ROOT/fake-bin:$PATH" \
  sh "$AUDIT" >/dev/null
if [ ! -f "$TMP_ROOT/go.args" ]; then
  echo "architecture audit did not delegate to the Go CLI" >&2
  exit 1
fi
grep -Fqx 'run ./cmd/mochat-architecture -root .' "$TMP_ROOT/go.args"

echo "architecture boundary wrapper self-test passed"
