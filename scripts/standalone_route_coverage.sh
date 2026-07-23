#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_ROUTE_COVERAGE_PROJECT:-mochat-go-route-coverage}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18100}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13317}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26390}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-route-coverage.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
ROUTES_JSON="$WORK_DIR/routes.json"
READYZ_JSON="$WORK_DIR/readyz.json"
COVERAGE_OUT="${MOCHAT_ROUTE_COVERAGE_OUT:-$WORK_DIR/coverage.json}"
MAX_MISSING="${MOCHAT_ROUTE_COVERAGE_MAX_MISSING:-}"
TOP_MISSING="${MOCHAT_ROUTE_COVERAGE_TOP:-50}"
GO_PID=""

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ "${KEEP_ROUTE_COVERAGE_WORKDIR:-0}" != "1" ]; then
    rm -rf "$WORK_DIR"
  else
    echo "route coverage workdir kept: $WORK_DIR" >&2
  fi
  if [ "${KEEP_ROUTE_COVERAGE_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
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

wait_service_healthy() {
  local service="$1"
  local deadline=$((SECONDS + ${MOCHAT_SERVICE_HEALTH_TIMEOUT_SECONDS:-360}))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local status
    status="$(compose ps --format json "$service" 2>/dev/null | python3 -c 'import json,sys; data=sys.stdin.read().strip(); print(json.loads(data).get("Health", "")) if data else print("")' 2>/dev/null || true)"
    if [ "$status" = "healthy" ]; then
      return 0
    fi
    sleep 2
  done
  echo "$service did not become healthy" >&2
  compose ps >&2 || true
  compose logs --tail=120 "$service" >&2 || true
  exit 1
}

wait_url() {
  local url="$1"
  local expected="$2"
  local deadline=$((SECONDS + 45))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $url to return $expected" >&2
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
}

assert_port_free "$GO_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_PHP_UPSTREAM="not-a-valid-upstream-url" \
  MOCHAT_SOURCE_ROOT="$WORK_DIR/should-not-read-php-source" \
  MOCHAT_COMPAT_MANIFEST="$WORK_DIR/should-not-read-manifest.json" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-route-coverage-secret}" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
curl -sS -f "http://$GO_ADDR/readyz" >"$READYZ_JSON"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$ROUTES_JSON"

python3 - "$ROUTES_JSON" "$READYZ_JSON" "$COVERAGE_OUT" "$MAX_MISSING" "$TOP_MISSING" <<'PY'
import json
import pathlib
import sys

routes_path = pathlib.Path(sys.argv[1])
readyz_path = pathlib.Path(sys.argv[2])
out_path = pathlib.Path(sys.argv[3])
max_missing_raw = sys.argv[4]
top_missing = int(sys.argv[5] or "50")

payload = json.loads(routes_path.read_text(encoding="utf-8"))
readyz = json.loads(readyz_path.read_text(encoding="utf-8"))

assert readyz["standalone"] is True, readyz
assert readyz["source_root"] == "", readyz
assert readyz["manifest_path"] == "", readyz
assert readyz["proxy_fallback_enabled"] is False, readyz
assert readyz["php_upstream"] == "", readyz

def route_key(route):
    if isinstance(route, str):
        return route.strip()
    return f"{str(route.get('method', '')).upper()} {route.get('path', '')}".strip()

manifest_routes = sorted({route_key(route) for route in payload.get("routes", []) if route_key(route)})
migrated_routes = sorted({route_key(route) for route in payload.get("migrated_routes", []) if route_key(route)})
manifest = set(manifest_routes)
migrated = set(migrated_routes)
migrated_manifest = sorted(manifest & migrated)
missing = sorted(manifest - migrated)
extra = sorted(migrated - manifest)

coverage = {
    "source_revision": payload.get("source_revision", ""),
    "route_total": len(manifest_routes),
    "migrated_route_total": len(migrated_routes),
    "migrated_manifest_route_total": len(migrated_manifest),
    "missing_route_total": len(missing),
    "extra_route_total": len(extra),
    "missing_routes": missing,
    "migrated_manifest_routes": migrated_manifest,
    "extra_routes": extra,
}

out_path.parent.mkdir(parents=True, exist_ok=True)
out_path.write_text(json.dumps(coverage, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

print(f"route_total={coverage['route_total']}")
print(f"migrated_manifest_route_total={coverage['migrated_manifest_route_total']}")
print(f"missing_route_total={coverage['missing_route_total']}")
print(f"extra_route_total={coverage['extra_route_total']}")
print(f"coverage_json={out_path}")
if missing:
    print("missing_routes_sample:")
    for route in missing[:top_missing]:
        print(f"  {route}")

if max_missing_raw:
    max_missing = int(max_missing_raw)
    if len(missing) > max_missing:
        print(f"missing route total {len(missing)} exceeds MOCHAT_ROUTE_COVERAGE_MAX_MISSING={max_missing}", file=sys.stderr)
        sys.exit(1)
PY
