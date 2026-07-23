#!/usr/bin/env bash

validate_production_base_url() {
  local name="$1"
  local value="${2:-}"
  if [ -z "$value" ]; then
    echo "$name is required" >&2
    return 2
  fi
  if [ "${MOCHAT_PRODUCTION_EVIDENCE_ALLOW_NON_PROD_URLS:-0}" = "1" ]; then
    return 0
  fi

  python3 - "$name" "$value" <<'PY'
import sys
from urllib.parse import urlparse

name, raw = sys.argv[1], sys.argv[2]
parsed = urlparse(raw)
host = (parsed.hostname or "").strip().lower()
non_prod_hosts = {
    "localhost",
    "127.0.0.1",
    "0.0.0.0",
    "::1",
    "example.com",
    "example.org",
    "example.net",
}
non_prod_suffixes = (
    ".localhost",
    ".example.com",
    ".example.org",
    ".example.net",
    ".test",
    ".invalid",
)

if parsed.scheme not in {"http", "https"} or not parsed.netloc:
    print(f"{name} must be an http(s) URL", file=sys.stderr)
    sys.exit(2)
if host in non_prod_hosts or host.endswith(non_prod_suffixes):
    print(
        f"{name} points to placeholder or local host `{host}`; "
        "set MOCHAT_PRODUCTION_EVIDENCE_ALLOW_NON_PROD_URLS=1 only for local fixture smoke",
        file=sys.stderr,
    )
    sys.exit(2)
PY
}
