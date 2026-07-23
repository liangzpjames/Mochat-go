#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-standalone-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18083}"
GO_HOST="${GO_ADDR%:*}"
GO_PORT="${GO_ADDR##*:}"
SIDEBAR_ADDR="${MOCHAT_SIDEBAR_FRONTEND_ADDR:-$GO_HOST:$((GO_PORT + 1))}"
OPERATION_ADDR="${MOCHAT_OPERATION_FRONTEND_ADDR:-$GO_HOST:$((GO_PORT + 2))}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13316}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26389}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-standalone-stack.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR"
  if [ "${KEEP_STACK:-0}" != "1" ]; then
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
  [ -f "$GO_LOG" ] && tail -80 "$GO_LOG" >&2 || true
  exit 1
}

assert_port_free "$GO_ADDR"
assert_port_free "$SIDEBAR_ADDR"
assert_port_free "$OPERATION_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

compose exec -T mysql mariadb -umochat -pmochat_pass mochat -e "SHOW TABLES LIKE 'mc_user';" | grep -q 'mc_user'
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT COUNT(*) FROM mc_rbac_menu;" | grep -q '458'
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -e "SHOW TABLES LIKE 'mochat_go_saas_packages';" | grep -q 'mochat_go_saas_packages'
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -e "SHOW TABLES LIKE 'mochat_go_saas_storage_objects';" | grep -q 'mochat_go_saas_storage_objects'
compose exec -T redis redis-cli ping | grep -q 'PONG'

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
  MOCHAT_SIMPLE_JWT_SECRET="standalone-stack-secret" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
wait_url "http://$GO_ADDR/sidebar-app/contact" 200
wait_url "http://$GO_ADDR/operation-app/workFission" 200
wait_url "http://$SIDEBAR_ADDR/contact" 200
wait_url "http://$OPERATION_ADDR/workFission" 200

curl -sS -f "http://$GO_ADDR/readyz" >"$WORK_DIR/readyz.json"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
curl -sS -f "http://$GO_ADDR/login" >"$WORK_DIR/dashboard-login.html"
curl -sS -f "http://$GO_ADDR/sidebar-app/contact" >"$WORK_DIR/sidebar-prefixed-contact.html"
curl -sS -f "http://$GO_ADDR/operation-app/workFission" >"$WORK_DIR/operation-prefixed-work-fission.html"
curl -sS -f "http://$SIDEBAR_ADDR/contact" >"$WORK_DIR/sidebar-contact.html"
curl -sS -f "http://$OPERATION_ADDR/workFission" >"$WORK_DIR/operation-work-fission.html"
unknown_code="$(curl -sS -o "$WORK_DIR/unknown.json" -w '%{http_code}' "http://$GO_ADDR/dashboard/notMigrated")"
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
dashboard_login = (work / "dashboard-login.html").read_text(encoding="utf-8")
sidebar_prefixed_contact = (work / "sidebar-prefixed-contact.html").read_text(encoding="utf-8")
operation_prefixed_work_fission = (work / "operation-prefixed-work-fission.html").read_text(encoding="utf-8")
sidebar_contact = (work / "sidebar-contact.html").read_text(encoding="utf-8")
operation_work_fission = (work / "operation-work-fission.html").read_text(encoding="utf-8")

assert readyz["standalone"] is True, readyz
assert readyz["mode"] == "standalone-go", readyz
assert readyz["source_root"] == "", readyz
assert readyz["manifest_path"] == "", readyz
assert readyz["proxy_fallback_enabled"] is False, readyz
assert readyz["php_upstream"] == "", readyz
assert len(routes["routes"]) >= 200, len(routes["routes"])
assert routes["migrated_route_count"] >= len(routes["routes"]), routes["migrated_route_count"]
assert unknown_code == "501", unknown_code
assert unknown["message"] == "route not yet migrated in standalone Go runtime", unknown
assert 'id="app"' in dashboard_login and "/js/app." in dashboard_login, dashboard_login[:200]
assert 'id="app"' in sidebar_prefixed_contact and "/sidebar-app/js/app." in sidebar_prefixed_contact and "/sidebar-app/css/app." in sidebar_prefixed_contact, sidebar_prefixed_contact[:300]
assert 'id="app"' in operation_prefixed_work_fission and "/operation-app/js/app." in operation_prefixed_work_fission and "/operation-app/css/app." in operation_prefixed_work_fission, operation_prefixed_work_fission[:300]
assert 'id="app"' in sidebar_contact and "/js/app." in sidebar_contact, sidebar_contact[:200]
assert 'id="app"' in operation_work_fission and "/js/app." in operation_work_fission, operation_work_fission[:200]
assert sidebar_unknown_code == "501", sidebar_unknown_code
assert operation_unknown_code == "501", operation_unknown_code
assert sidebar_unknown["message"] == "route not yet migrated in standalone Go runtime", sidebar_unknown
assert operation_unknown["message"] == "route not yet migrated in standalone Go runtime", operation_unknown
print("standalone stack check passed")
PY
