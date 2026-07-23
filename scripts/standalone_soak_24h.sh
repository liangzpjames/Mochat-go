#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_SOAK_PROJECT:-mochat-go-soak-24h}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18161}"
GO_HOST="${GO_ADDR%:*}"
GO_PORT="${GO_ADDR##*:}"
SIDEBAR_ADDR="${MOCHAT_SIDEBAR_FRONTEND_ADDR:-$GO_HOST:$((GO_PORT + 1))}"
OPERATION_ADDR="${MOCHAT_OPERATION_FRONTEND_ADDR:-$GO_HOST:$((GO_PORT + 2))}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13361}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26411}"
SOAK_DURATION_SECONDS="${MOCHAT_SOAK_DURATION_SECONDS:-86400}"
SOAK_INTERVAL_SECONDS="${MOCHAT_SOAK_INTERVAL_SECONDS:-60}"
SOAK_MIN_ITERATIONS="${MOCHAT_SOAK_MIN_ITERATIONS:-2}"
WORK_DIR="${MOCHAT_SOAK_WORK_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-soak.XXXXXX")}"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
SOAK_LOG="${MOCHAT_SOAK_LOG:-$WORK_DIR/soak.ndjson}"
GO_PID=""

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ "${MOCHAT_SOAK_KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
  if [ "${MOCHAT_SOAK_KEEP_WORK_DIR:-1}" != "1" ]; then
    rm -rf "$WORK_DIR"
  else
    echo "standalone soak workdir kept: $WORK_DIR" >&2
  fi
}
trap cleanup EXIT INT TERM

assert_positive_integer() {
  local name="$1"
  local value="$2"
  if ! [[ "$value" =~ ^[1-9][0-9]*$ ]]; then
    echo "$name must be a positive integer, got: $value" >&2
    exit 1
  fi
}

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

assert_go_alive() {
  if [ -z "$GO_PID" ] || ! kill -0 "$GO_PID" 2>/dev/null; then
    echo "mochat-go process is not running" >&2
    [ -f "$GO_LOG" ] && tail -160 "$GO_LOG" >&2 || true
    exit 1
  fi
}

probe_once() {
  local iteration="$1"
  local probe_dir="$WORK_DIR/probe-$iteration"
  local unknown_code
  local sidebar_unknown_code
  local operation_unknown_code
  local rss_kb

  assert_go_alive
  mkdir -p "$probe_dir"

  curl -sS -f "http://$GO_ADDR/readyz" >"$probe_dir/readyz.json"
  curl -sS -f "http://$GO_ADDR/compat/routes" >"$probe_dir/routes.json"
  curl -sS -f "http://$GO_ADDR/login" >"$probe_dir/dashboard-login.html"
  curl -sS -f "http://$GO_ADDR/sidebar-app/contact" >"$probe_dir/sidebar-prefixed-contact.html"
  curl -sS -f "http://$GO_ADDR/operation-app/workFission" >"$probe_dir/operation-prefixed-work-fission.html"
  curl -sS -f "http://$SIDEBAR_ADDR/contact" >"$probe_dir/sidebar-contact.html"
  curl -sS -f "http://$OPERATION_ADDR/workFission" >"$probe_dir/operation-work-fission.html"
  unknown_code="$(curl -sS -o "$probe_dir/unknown.json" -w '%{http_code}' "http://$GO_ADDR/dashboard/notMigrated")"
  sidebar_unknown_code="$(curl -sS -o "$probe_dir/sidebar-unknown.json" -w '%{http_code}' "http://$SIDEBAR_ADDR/sidebar/notMigrated")"
  operation_unknown_code="$(curl -sS -o "$probe_dir/operation-unknown.json" -w '%{http_code}' "http://$OPERATION_ADDR/operation/notMigrated")"

  compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT 1;" | grep -q '^1$'
  compose exec -T redis redis-cli ping | grep -q '^PONG$'

  rss_kb="$(ps -o rss= -p "$GO_PID" 2>/dev/null | tr -d ' ' || true)"

  python3 - "$probe_dir" "$SOAK_LOG" "$iteration" "$unknown_code" "$sidebar_unknown_code" "$operation_unknown_code" "$rss_kb" <<'PY'
import datetime as dt
import json
import pathlib
import sys

probe = pathlib.Path(sys.argv[1])
soak_log = pathlib.Path(sys.argv[2])
iteration = int(sys.argv[3])
unknown_code = sys.argv[4]
sidebar_unknown_code = sys.argv[5]
operation_unknown_code = sys.argv[6]
rss_kb_raw = sys.argv[7]
rss_kb = int(rss_kb_raw) if rss_kb_raw else None

readyz = json.loads((probe / "readyz.json").read_text(encoding="utf-8"))
routes = json.loads((probe / "routes.json").read_text(encoding="utf-8"))
unknown = json.loads((probe / "unknown.json").read_text(encoding="utf-8"))
sidebar_unknown = json.loads((probe / "sidebar-unknown.json").read_text(encoding="utf-8"))
operation_unknown = json.loads((probe / "operation-unknown.json").read_text(encoding="utf-8"))
dashboard_login = (probe / "dashboard-login.html").read_text(encoding="utf-8")
sidebar_prefixed_contact = (probe / "sidebar-prefixed-contact.html").read_text(encoding="utf-8")
operation_prefixed_work_fission = (probe / "operation-prefixed-work-fission.html").read_text(encoding="utf-8")
sidebar_contact = (probe / "sidebar-contact.html").read_text(encoding="utf-8")
operation_work_fission = (probe / "operation-work-fission.html").read_text(encoding="utf-8")

route_total = len(routes.get("routes", []))
migrated_count = int(routes.get("migrated_route_count") or len(routes.get("migrated_routes", [])))

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
assert route_total >= 224, route_total
assert migrated_count >= route_total, (migrated_count, route_total)
assert unknown_code == "501", unknown_code
assert sidebar_unknown_code == "501", sidebar_unknown_code
assert operation_unknown_code == "501", operation_unknown_code
assert unknown["message"] == "route not yet migrated in standalone Go runtime", unknown
assert sidebar_unknown["message"] == "route not yet migrated in standalone Go runtime", sidebar_unknown
assert operation_unknown["message"] == "route not yet migrated in standalone Go runtime", operation_unknown
assert 'id="app"' in dashboard_login and "/js/app." in dashboard_login, dashboard_login[:200]
assert 'id="app"' in sidebar_prefixed_contact and "/sidebar-app/js/app." in sidebar_prefixed_contact, sidebar_prefixed_contact[:300]
assert 'id="app"' in operation_prefixed_work_fission and "/operation-app/js/app." in operation_prefixed_work_fission, operation_prefixed_work_fission[:300]
assert 'id="app"' in sidebar_contact and "/js/app." in sidebar_contact, sidebar_contact[:200]
assert 'id="app"' in operation_work_fission and "/js/app." in operation_work_fission, operation_work_fission[:200]

event = {
    "ts": dt.datetime.now(dt.timezone.utc).isoformat(),
    "iteration": iteration,
    "route_total": route_total,
    "migrated_route_count": migrated_count,
    "rss_kb": rss_kb,
    "standalone": readyz["standalone"],
    "mode": readyz["mode"],
}
soak_log.parent.mkdir(parents=True, exist_ok=True)
with soak_log.open("a", encoding="utf-8") as fh:
    fh.write(json.dumps(event, ensure_ascii=False, sort_keys=True) + "\n")
print(f"soak probe {iteration} passed: routes={route_total} migrated={migrated_count} rss_kb={rss_kb}")
PY
}

assert_positive_integer MOCHAT_SOAK_DURATION_SECONDS "$SOAK_DURATION_SECONDS"
assert_positive_integer MOCHAT_SOAK_INTERVAL_SECONDS "$SOAK_INTERVAL_SECONDS"
assert_positive_integer MOCHAT_SOAK_MIN_ITERATIONS "$SOAK_MIN_ITERATIONS"

mkdir -p "$WORK_DIR"
: >"$SOAK_LOG"

assert_port_free "$GO_ADDR"
assert_port_free "$SIDEBAR_ADDR"
assert_port_free "$OPERATION_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

compose exec -T mysql mariadb -umochat -pmochat_pass mochat -e "SHOW TABLES LIKE 'mc_user';" | grep -q 'mc_user'
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -e "SHOW TABLES LIKE 'mochat_go_saas_packages';" | grep -q 'mochat_go_saas_packages'
compose exec -T redis redis-cli ping | grep -q '^PONG$'

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ENABLE_FRONTEND_SERVERS=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_PHP_UPSTREAM="not-a-valid-upstream-url" \
  MOCHAT_SOURCE_ROOT="$WORK_DIR/should-not-read-php-source" \
  MOCHAT_COMPAT_MANIFEST="$WORK_DIR/should-not-read-manifest.json" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-standalone-soak-secret}" \
  MOCHAT_FILE_STORAGE_ROOT="$WORK_DIR/upload" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
wait_url "http://$GO_ADDR/sidebar-app/contact" 200
wait_url "http://$GO_ADDR/operation-app/workFission" 200
wait_url "http://$SIDEBAR_ADDR/contact" 200
wait_url "http://$OPERATION_ADDR/workFission" 200

deadline=$((SECONDS + SOAK_DURATION_SECONDS))
iteration=0
while true; do
  iteration=$((iteration + 1))
  probe_once "$iteration"
  if [ "$SECONDS" -ge "$deadline" ] && [ "$iteration" -ge "$SOAK_MIN_ITERATIONS" ]; then
    break
  fi
  sleep_for="$SOAK_INTERVAL_SECONDS"
  remaining=$((deadline - SECONDS))
  if [ "$remaining" -gt 0 ] && [ "$remaining" -lt "$sleep_for" ]; then
    sleep_for="$remaining"
  fi
  if [ "$sleep_for" -le 0 ]; then
    sleep_for=1
  fi
  sleep "$sleep_for"
done

echo "standalone soak passed: duration_seconds=$SOAK_DURATION_SECONDS iterations=$iteration log=$SOAK_LOG"
