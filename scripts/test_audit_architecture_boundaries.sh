#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
AUDIT="$ROOT/scripts/audit_architecture_boundaries.sh"
TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

mkdir -p \
  "$TMP_ROOT/internal/modules/good/domain" \
  "$TMP_ROOT/internal/modules/bad/domain" \
  "$TMP_ROOT/internal/dashboard" \
  "$TMP_ROOT/internal/store" \
  "$TMP_ROOT/cmd/mochat-go" \
  "$TMP_ROOT/scripts"

printf 'package domain\n' >"$TMP_ROOT/internal/modules/good/domain/model.go"
printf 'package dashboard\n' >"$TMP_ROOT/internal/dashboard/saas_admin_page.go"
printf 'package dashboard\n' >"$TMP_ROOT/internal/dashboard/saas_admin.go"
printf 'package store\n' >"$TMP_ROOT/internal/store/mysql.go"
printf 'package main\n' >"$TMP_ROOT/cmd/mochat-go/main.go"

write_baseline() {
  wc -c \
    "$TMP_ROOT/internal/dashboard/saas_admin_page.go" \
    "$TMP_ROOT/internal/dashboard/saas_admin.go" \
    "$TMP_ROOT/internal/store/mysql.go" \
    "$TMP_ROOT/cmd/mochat-go/main.go" |
    awk -v root="$TMP_ROOT/" 'NF == 2 && $2 != "total" { sub(root, "", $2); print $1, $2 }' \
      >"$TMP_ROOT/scripts/architecture-size-baseline.txt"
}

write_baseline
MOCHAT_ARCH_ROOT="$TMP_ROOT" "$AUDIT"

printf 'package domain\nimport _ "jiyi/mochat-go/internal/store"\n' \
  >"$TMP_ROOT/internal/modules/bad/domain/model.go"
if MOCHAT_ARCH_ROOT="$TMP_ROOT" "$AUDIT" >/dev/null 2>&1; then
  echo "architecture audit accepted a domain dependency on internal/store" >&2
  exit 1
fi
rm "$TMP_ROOT/internal/modules/bad/domain/model.go"

printf '// growth\n' >>"$TMP_ROOT/internal/store/mysql.go"
if MOCHAT_ARCH_ROOT="$TMP_ROOT" "$AUDIT" >/dev/null 2>&1; then
  echo "architecture audit accepted protected-file growth" >&2
  exit 1
fi

echo "architecture boundary audit self-test passed"
