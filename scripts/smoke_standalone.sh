#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-standalone.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18082}"
GO_HOST="${GO_ADDR%:*}"
GO_PORT="${GO_ADDR##*:}"
SIDEBAR_ADDR="${MOCHAT_SIDEBAR_FRONTEND_ADDR:-$GO_HOST:$((GO_PORT + 1))}"
OPERATION_ADDR="${MOCHAT_OPERATION_FRONTEND_ADDR:-$GO_HOST:$((GO_PORT + 2))}"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" >/dev/null 2>&1; then
    kill "$GO_PID" >/dev/null 2>&1 || true
    wait "$GO_PID" >/dev/null 2>&1 || true
  fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

wait_url() {
  local url="$1"
  local expected="$2"
  local attempt
  for attempt in $(seq 1 80); do
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 0.25
  done
  echo "timed out waiting for $url" >&2
  if [ -f "$GO_LOG" ]; then
    cat "$GO_LOG" >&2
  fi
  return 1
}

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
	  MOCHAT_GO_STANDALONE=1 \
	  MOCHAT_GO_ENABLE_FRONTEND_SERVERS=1 \
	  MOCHAT_GO_ADDR="$GO_ADDR" \
	  MOCHAT_PHP_UPSTREAM="not-a-valid-upstream-url" \
	  MOCHAT_SOURCE_ROOT="$WORK_DIR/should-not-read-php-source" \
	  MOCHAT_COMPAT_MANIFEST="$WORK_DIR/should-not-read-manifest.json" \
	  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
wait_url "http://$SIDEBAR_ADDR/contact" 200
wait_url "http://$OPERATION_ADDR/workFission" 200

curl -sS -f "http://$GO_ADDR/readyz" >"$WORK_DIR/readyz.json"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
curl -sS -f "http://$GO_ADDR/login" >"$WORK_DIR/dashboard-login.html"
curl -sS -f "http://$SIDEBAR_ADDR/contact" >"$WORK_DIR/sidebar-contact.html"
curl -sS -f "http://$OPERATION_ADDR/workFission" >"$WORK_DIR/operation-work-fission.html"
curl -sS -f -o "$WORK_DIR/favicon.ico" "http://$GO_ADDR/favicon.ico"
unknown_code="$(curl -sS -D "$WORK_DIR/unknown.headers" -o "$WORK_DIR/unknown.json" -w '%{http_code}' "http://$GO_ADDR/dashboard/notMigrated")"
sidebar_unknown_code="$(curl -sS -o "$WORK_DIR/sidebar-unknown.json" -w '%{http_code}' "http://$SIDEBAR_ADDR/sidebar/notMigrated")"
operation_unknown_code="$(curl -sS -o "$WORK_DIR/operation-unknown.json" -w '%{http_code}' "http://$OPERATION_ADDR/operation/notMigrated")"

python3 - "$WORK_DIR" "$unknown_code" "$sidebar_unknown_code" "$operation_unknown_code" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
unknown_code = sys.argv[2]
sidebar_unknown_code = sys.argv[3]
operation_unknown_code = sys.argv[4]

readyz = json.loads((work / "readyz.json").read_text(encoding="utf-8"))
routes = json.loads((work / "routes.json").read_text(encoding="utf-8"))
unknown = json.loads((work / "unknown.json").read_text(encoding="utf-8"))
sidebar_unknown = json.loads((work / "sidebar-unknown.json").read_text(encoding="utf-8"))
operation_unknown = json.loads((work / "operation-unknown.json").read_text(encoding="utf-8"))
headers = (work / "unknown.headers").read_text(encoding="utf-8").lower()
dashboard_login = (work / "dashboard-login.html").read_text(encoding="utf-8")
sidebar_contact = (work / "sidebar-contact.html").read_text(encoding="utf-8")
operation_work_fission = (work / "operation-work-fission.html").read_text(encoding="utf-8")

assert readyz["standalone"] is True, readyz
assert readyz["mode"] == "standalone-go", readyz
assert readyz["source_root"] == "", readyz
assert readyz["source_root_exists"] is False, readyz
assert readyz["manifest_path"] == "", readyz
assert readyz["manifest_exists"] is True, readyz
assert readyz["proxy_fallback_enabled"] is False, readyz
assert readyz["php_upstream"] == "", readyz
assert readyz["php_upstream_ready"] is False, readyz
assert "standalone mode" in readyz["php_upstream_probe"], readyz
assert routes["source_revision"], routes
assert len(routes["routes"]) >= 200, len(routes["routes"])
assert unknown_code == "501", unknown_code
assert unknown["message"] == "route not yet migrated in standalone Go runtime", unknown
assert "x-mochat-go-compat" not in headers, headers
assert 'id="app"' in dashboard_login and "/js/app." in dashboard_login, dashboard_login[:200]
assert 'id="app"' in sidebar_contact and "/js/app." in sidebar_contact, sidebar_contact[:200]
assert 'id="app"' in operation_work_fission and "/js/app." in operation_work_fission, operation_work_fission[:200]
assert sidebar_unknown_code == "501", sidebar_unknown_code
assert operation_unknown_code == "501", operation_unknown_code
assert sidebar_unknown["message"] == "route not yet migrated in standalone Go runtime", sidebar_unknown
assert operation_unknown["message"] == "route not yet migrated in standalone Go runtime", operation_unknown
print("standalone smoke passed")
PY
