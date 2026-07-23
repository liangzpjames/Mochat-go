#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

SOURCE_ROOT="${MOCHAT_SOURCE_ROOT:-../mochat}"
EMBEDDED_MANIFEST="${MOCHAT_EMBEDDED_MANIFEST:-internal/server/compat_manifest_embedded.json}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-inventory-parity.XXXXXX")"

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT INT TERM

if [ ! -d "$SOURCE_ROOT/api-server" ]; then
  echo "MoChat PHP source root not found: $SOURCE_ROOT" >&2
  echo "Set MOCHAT_SOURCE_ROOT to a checkout that contains api-server/." >&2
  exit 1
fi

if [ ! -f "$EMBEDDED_MANIFEST" ]; then
  echo "embedded manifest not found: $EMBEDDED_MANIFEST" >&2
  exit 1
fi

env -u GOROOT go run ./cmd/mochat-inventory \
  -source-root "$SOURCE_ROOT" \
  -out "$WORK_DIR" \
  -source-revision "inventory-parity" >/dev/null

python3 - "$WORK_DIR/compat_manifest.json" "$EMBEDDED_MANIFEST" <<'PY'
from collections import Counter
import json
import sys

generated_path, embedded_path = sys.argv[1], sys.argv[2]
with open(generated_path, encoding="utf-8") as fh:
    generated = json.load(fh)
with open(embedded_path, encoding="utf-8") as fh:
    embedded = json.load(fh)

def counter(items, fields):
    return Counter(tuple(str(item.get(field, "")) for field in fields) for item in items)

sections = [
    ("routes", ("method", "path", "file")),
    ("tables", ("name",)),
    ("crontabs", ("name", "rule", "memo", "file")),
    ("event_handlers", ("event_path", "file")),
    ("async_queues", ("pool", "file")),
]

failed = False
for section, fields in sections:
    left = counter(generated.get(section, []), fields)
    right = counter(embedded.get(section, []), fields)
    print(f"{section}: php_scan={sum(left.values())} embedded={sum(right.values())}")
    missing = left - right
    extra = right - left
    if missing:
        failed = True
        print(f"{section}: missing_in_embedded={sum(missing.values())}")
        for key, count in sorted(missing.items())[:50]:
            suffix = f" x{count}" if count > 1 else ""
            print("  MISSING", dict(zip(fields, key)), suffix)
    if extra:
        failed = True
        print(f"{section}: extra_in_embedded={sum(extra.values())}")
        for key, count in sorted(extra.items())[:50]:
            suffix = f" x{count}" if count > 1 else ""
            print("  EXTRA", dict(zip(fields, key)), suffix)

if failed:
    raise SystemExit(1)
print("standalone inventory parity passed")
PY
