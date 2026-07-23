#!/usr/bin/env sh
set -eu

DEFAULT_ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
ROOT="${MOCHAT_ARCH_ROOT:-$DEFAULT_ROOT}"
BASELINE="${MOCHAT_ARCH_SIZE_BASELINE:-$ROOT/scripts/architecture-size-baseline.txt}"
FAILED=0

if [ ! -f "$BASELINE" ]; then
  echo "architecture audit: size baseline not found: $BASELINE" >&2
  exit 1
fi

DOMAIN_ROOT="$ROOT/internal/modules"
if [ -d "$DOMAIN_ROOT" ]; then
  BAD_IMPORTS="$(
    find "$DOMAIN_ROOT" -type f -path '*/domain/*.go' -print0 |
      xargs -0 grep -nE '"jiyi/mochat-go/(cmd|internal/(dashboard|store|server|config|frontend))([/"]|$)' 2>/dev/null ||
      true
  )"
  if [ -n "$BAD_IMPORTS" ]; then
    echo "architecture audit: domain packages contain forbidden inward dependencies:" >&2
    echo "$BAD_IMPORTS" >&2
    FAILED=1
  fi
fi

while IFS=' ' read -r limit relative_path; do
  [ -n "${limit:-}" ] || continue
  case "$limit" in
    \#*) continue ;;
    *[!0-9]*)
      echo "architecture audit: invalid byte limit in $BASELINE: $limit" >&2
      FAILED=1
      continue
      ;;
  esac
  target="$ROOT/$relative_path"
  if [ ! -f "$target" ]; then
    echo "architecture audit: protected file is missing: $relative_path" >&2
    FAILED=1
    continue
  fi
  actual="$(wc -c <"$target" | tr -d ' ')"
  if [ "$actual" -gt "$limit" ]; then
    echo "architecture audit: protected file grew: $relative_path ($actual > $limit bytes)" >&2
    FAILED=1
  fi
done <"$BASELINE"

if [ "$FAILED" -ne 0 ]; then
  exit 1
fi

echo "architecture boundaries passed"
