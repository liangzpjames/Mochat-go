#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18080}"
PHP_ADDR="${MOCHAT_FAKE_PHP_ADDR:-127.0.0.1:19051}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-smoke.XXXXXX")"
GO_LOG="$WORK_DIR/go.log"
PHP_LOG="$WORK_DIR/fake-php.log"
GO_BIN="$WORK_DIR/mochat-go"
GO_PID=""
PHP_PID=""

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ -n "$PHP_PID" ] && kill -0 "$PHP_PID" 2>/dev/null; then
    kill "$PHP_PID" 2>/dev/null || true
    wait "$PHP_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT INT TERM

assert_port_free() {
  local addr="$1"
  local port="${addr##*:}"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port $port is already in use" >&2
    exit 1
  fi
}

wait_url() {
  local url="$1"
  local expected="$2"
  local deadline=$((SECONDS + 30))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $url to return $expected" >&2
  [ -f "$GO_LOG" ] && tail -50 "$GO_LOG" >&2 || true
  [ -f "$PHP_LOG" ] && tail -50 "$PHP_LOG" >&2 || true
  exit 1
}

request() {
  local method="$1"
  local route_path="$2"
  local body="${3:-}"
  local expected="$4"
  local require_fallback_header="${5:-false}"
  local headers="$WORK_DIR/headers"
  local output="$WORK_DIR/body"
  local code

  if [ -n "$body" ]; then
    code="$(curl -sS -X "$method" -H 'Content-Type: application/json' -d "$body" -D "$headers" -o "$output" -w '%{http_code}' "http://$GO_ADDR$route_path")"
  else
    code="$(curl -sS -X "$method" -D "$headers" -o "$output" -w '%{http_code}' "http://$GO_ADDR$route_path")"
  fi

  if [ "$code" != "$expected" ]; then
    echo "$method $route_path returned $code, expected $expected" >&2
    cat "$headers" >&2 || true
    cat "$output" >&2 || true
    exit 1
  fi
  if [ "$require_fallback_header" = "true" ] && ! grep -qi '^X-Mochat-Go-Compat: fallback-proxy' "$headers"; then
    echo "$method $route_path did not include fallback header" >&2
    cat "$headers" >&2 || true
    exit 1
  fi
}

assert_port_free "$GO_ADDR"
assert_port_free "$PHP_ADDR"

python3 - "$PHP_ADDR" >"$PHP_LOG" 2>&1 <<'PY' &
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

host, port = sys.argv[1].rsplit(":", 1)

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self._reply()

    def do_POST(self):
        self._reply()

    def do_PUT(self):
        self._reply()

    def do_HEAD(self):
        self._reply(head=True)

    def _reply(self, head=False):
        if self.path == "/":
            payload = b"Hello MoChat "
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            if not head:
                self.wfile.write(payload)
            return

        body = json.dumps({
            "code": 0,
            "msg": "fake php fallback",
            "data": {"method": self.command, "path": self.path},
        }).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if not head:
            self.wfile.write(body)

    def log_message(self, format, *args):
        return

ThreadingHTTPServer((host, int(port)), Handler).serve_forever()
PY
PHP_PID="$!"

wait_url "http://$PHP_ADDR/" 200
go build -o "$GO_BIN" ./cmd/mochat-go

env \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_SOURCE_ROOT=../mochat \
  MOCHAT_COMPAT_MANIFEST=../docs/migration/compat_manifest.json \
  MOCHAT_PHP_UPSTREAM="http://$PHP_ADDR" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/healthz" 200
wait_url "http://$GO_ADDR/readyz" 200

request GET /healthz "" 200 false
request GET /compat/routes "" 200 false
request GET /dashboard/user/loginShow "" 200 true
request PUT /dashboard/user/logout "" 200 true
request GET /dashboard/role/permissionByUser "" 200 true
request POST /dashboard/user/auth '{"phone":"13800000000","password":"secret"}' 200 true
request GET /dashboard/corp/select "" 200 true
request POST /dashboard/corp/bind '{"corpId":1}' 200 true
request GET /dashboard/chatTool/config "" 200 true
request GET /WW_verify_ABCDEF1234567890.txt "" 200 true
request POST /dashboard/agent/txtVerifyUpload "" 200 true
request GET /dashboard/role/select "" 200 true
request GET /dashboard/role/index "" 200 true
request GET /dashboard/role/show "" 200 true
request GET /dashboard/role/permissionShow "" 200 true
request GET /dashboard/role/showEmployee "" 200 true
request GET /dashboard/menu/iconIndex "" 200 true
request GET /dashboard/menu/select "" 200 true
request GET /dashboard/menu/index "" 200 true
request GET /dashboard/menu/show "" 200 true
request GET /dashboard/corpData/index "" 200 true
request GET /dashboard/corpData/lineChat "" 200 true
request GET /dashboard/workEmployee/index "" 200 true
request GET /dashboard/workEmployee/searchCondition "" 200 true
request GET /dashboard/workDepartment/index "" 200 true
request GET /dashboard/workEmployeeDepartment/memberIndex "" 200 true
request GET /dashboard/workDepartment/selectByPhone "" 200 true
request GET /dashboard/workDepartment/pageIndex "" 200 true
request GET /dashboard/workDepartment/showEmployee "" 200 true
request GET /dashboard/workContactTagGroup/index "" 200 true
request GET /dashboard/workContactTagGroup/detail "" 200 true
request GET /sidebar/workContactTagGroup/index "" 200 true
request GET /dashboard/workContactTag/index "" 200 true
request GET /dashboard/workContactTag/detail "" 200 true
request GET /dashboard/workContactTag/contactTagList "" 200 true
request GET /dashboard/workContactTag/allTag "" 200 true
request GET /dashboard/workContact/source "" 200 true
request GET /dashboard/workContact/show?contactId=900001\&employeeId=1 "" 200 true
request GET /dashboard/workContact/track "" 200 true
request GET /dashboard/workContactRoom/index?workRoomId=900001 "" 200 true
request GET /dashboard/workRoom/roomIndex "" 200 true
request GET /sidebar/workContactTag/allTag "" 200 true
request GET /sidebar/workContact/detail "" 200 true
request GET /sidebar/workContact/show?contactId=900001 "" 200 true
request GET /sidebar/workContact/track "" 200 true
request GET /sidebar/contactProcessStatus/index "" 200 true
request PUT /sidebar/contactProcessStatus/update '{"contactId":900001,"statusId":3}' 200 true
request GET /dashboard/contactField/index "" 200 true
request GET /dashboard/contactField/show "" 200 true
request GET /dashboard/contactField/portrait "" 200 true
request GET /dashboard/contactFieldPivot/index "" 200 true
request GET /sidebar/contactFieldPivot/index "" 200 true

echo "fallback smoke passed"
