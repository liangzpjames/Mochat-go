#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-storage-reconcile-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13336}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26470}"
GO_ADDR="${MOCHAT_SAAS_STORAGE_RECONCILE_GO_ADDR:-127.0.0.1:18104}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-storage-reconcile.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
MAINTENANCE_BIN="$WORK_DIR/mochat-saas-maintenance"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
STORAGE_ROOT="$WORK_DIR/storage/upload/static"
GO_PID=""

SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-storage-reconcile-secret}"
TENANT_ID="${MOCHAT_SAAS_STORAGE_RECONCILE_TENANT_ID:-401}"
PHONE="${MOCHAT_SAAS_STORAGE_RECONCILE_PHONE:-13800000401}"
PASSWORD="${MOCHAT_SAAS_STORAGE_RECONCILE_PASSWORD:-secret401}"

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
  local port="$1"
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

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  local query="$1"
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat_saas_storage_reconcile -e "$query" | tr -d '\r'
}

wait_mysql_scalar() {
  local query="$1"
  local expected="$2"
  local deadline=$((SECONDS + 45))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local value
    value="$(mysql_scalar "$query" || true)"
    if [ "$value" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for query result: $query" >&2
  echo "expected: $expected" >&2
  echo "actual: $(mysql_scalar "$query" || true)" >&2
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

mysql_root <<'SQL'
DROP DATABASE IF EXISTS mochat_saas_storage_reconcile;
CREATE DATABASE mochat_saas_storage_reconcile CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON mochat_saas_storage_reconcile.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$MAINTENANCE_BIN" ./cmd/mochat-saas-maintenance
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat_saas_storage_reconcile?parseTime=true&loc=Local"

"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0004_saas_storage_objects\tapplied_now' "$WORK_DIR/migrate.out"

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id "$TENANT_ID" \
  -tenant-name "SaaS存储校准租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "SaaS存储管理员" \
  -role-name "SaaS存储超级管理员" \
  -package-code "storage-reconcile" \
  -package-name "存储校准版" \
  -storage-mb 10 >"$WORK_DIR/bootstrap.out"
grep -q $'tenant_id\t'"$TENANT_ID" "$WORK_DIR/bootstrap.out"

mkdir -p "$STORAGE_ROOT/reconcile"
python3 - "$STORAGE_ROOT/reconcile/existing.bin" <<'PY'
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
path.write_bytes(b"x" * (2 * 1024 * 1024))
PY

USER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '$PHONE' AND tenant_id = $TENANT_ID AND deleted_at IS NULL")"
test -n "$USER_ID"

mysql_root mochat_saas_storage_reconcile <<SQL
INSERT INTO mochat_go_saas_storage_objects
  (tenant_id, user_id, employee_id, corp_id, source, original_name, relative_path, content_type, size_bytes, created_at, updated_at, deleted_at)
VALUES
  ($TENANT_ID, $USER_ID, 0, 0, 'seed.reconcile', 'existing.bin', 'reconcile/existing.bin', 'application/octet-stream', 1, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, 0, 'seed.reconcile', 'missing.bin', 'reconcile/missing.bin', 'application/octet-stream', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, 0, 'seed.reconcile', 'unsafe.bin', '../unsafe.bin', 'application/octet-stream', 1048576, NOW(), NOW(), NULL);
SQL

test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "0"

"$MAINTENANCE_BIN" \
  -dsn "$DSN" \
  -storage-root "$STORAGE_ROOT" \
  -tenant-id "$TENANT_ID" >"$WORK_DIR/reconcile.out"

grep -q $'action\treconcile-storage' "$WORK_DIR/reconcile.out"
grep -q $'tenant_id\t'"$TENANT_ID" "$WORK_DIR/reconcile.out"
grep -q $'scanned\t3' "$WORK_DIR/reconcile.out"
grep -q $'missing_marked\t1' "$WORK_DIR/reconcile.out"
grep -q $'unsafe_marked\t1' "$WORK_DIR/reconcile.out"
grep -q $'size_updated\t1' "$WORK_DIR/reconcile.out"
grep -q $'counters_refreshed\t1' "$WORK_DIR/reconcile.out"
grep -q $'refreshed_tenants\t'"$TENANT_ID" "$WORK_DIR/reconcile.out"

test "$(mysql_scalar "SELECT size_bytes FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'reconcile/existing.bin' AND deleted_at IS NULL")" = "2097152"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'reconcile/missing.bin' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = '../unsafe.bin' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "2"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "10"

python3 - "$STORAGE_ROOT/reconcile/cron.bin" <<'PY'
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
path.write_bytes(b"c" * 1048576)
PY

mysql_root mochat_saas_storage_reconcile <<SQL
INSERT INTO mochat_go_saas_storage_objects
  (tenant_id, user_id, employee_id, corp_id, source, original_name, relative_path, content_type, size_bytes, created_at, updated_at, deleted_at)
VALUES
  ($TENANT_ID, $USER_ID, 0, 0, 'seed.reconcile.cron', 'cron.bin', 'reconcile/cron.bin', 'application/octet-stream', 1, NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_FILE_STORAGE_ROOT="$STORAGE_ROOT" \
  MOCHAT_GO_ENABLE_SAAS_STORAGE_RECONCILE_CRON=1 \
  MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_INTERVAL_SECONDS=3600 \
  MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_RUN_ON_START=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
wait_mysql_scalar "SELECT size_bytes FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'reconcile/cron.bin' AND deleted_at IS NULL" "1048576"
wait_mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL" "3"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_tasks WHERE name = 'cron-saas-storage-reconcile' AND status = 'running'" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-storage-reconcile' AND kind = 'periodic_tick' AND status = 'succeeded'" "1"

echo "saas storage reconcile smoke passed"
