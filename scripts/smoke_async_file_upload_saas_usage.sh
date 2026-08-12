#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-async-file-upload-saas-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13344}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26399}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18097}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-async-file-upload-saas.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""

SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-async-file-upload-saas-secret}"
TENANT_A_ID=711
TENANT_B_ID=712
CORP_A_ID=711001
CORP_B_ID=712001
PHONE_A=13800000711
PHONE_B=13800000712
PASSWORD_A=secret711
PASSWORD_B=secret712

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" \
    MOCHAT_REDIS_PORT="$REDIS_PORT" \
    docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
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

wait_url() {
  local url="$1"
  local expected="$2"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $url to return $expected" >&2
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
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

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  local query="$1"
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat_async_file_upload_saas_check -e "$query" | tr -d '\r'
}

wait_mysql_scalar() {
  local query="$1"
  local expected="$2"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local value
    value="$(mysql_scalar "$query" || true)"
    if [ "$value" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for MySQL scalar to equal $expected" >&2
  echo "$query" >&2
  echo "last value: $(mysql_scalar "$query" || true)" >&2
  [ -f "$GO_LOG" ] && tail -160 "$GO_LOG" >&2 || true
  exit 1
}

wait_file_content() {
  local path="$1"
  local expected="$2"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if [ -f "$path" ] && [ "$(cat "$path")" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for file $path" >&2
  [ -f "$path" ] && cat "$path" >&2 || true
  [ -f "$GO_LOG" ] && tail -160 "$GO_LOG" >&2 || true
  exit 1
}

wait_file_absent() {
  local path="$1"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if [ ! -e "$path" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for file $path to be deleted" >&2
  [ -f "$GO_LOG" ] && tail -160 "$GO_LOG" >&2 || true
  exit 1
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

mysql_root <<'SQL'
DROP DATABASE IF EXISTS mochat_async_file_upload_saas_check;
CREATE DATABASE mochat_async_file_upload_saas_check CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON mochat_async_file_upload_saas_check.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat_async_file_upload_saas_check?parseTime=true&loc=Local"

"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id "$TENANT_A_ID" \
  -tenant-name "异步上传 SaaS 租户A" \
  -phone "$PHONE_A" \
  -password "$PASSWORD_A" \
  -user-name "异步上传 SaaS 管理员A" \
  -role-name "异步上传 SaaS 超管A" \
  -package-code "async-upload-a" \
  -package-name "异步上传A版" \
  -storage-mb 20 \
  -async-executions 1 >"$WORK_DIR/bootstrap-a.out"
"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id "$TENANT_B_ID" \
  -tenant-name "异步上传 SaaS 租户B" \
  -phone "$PHONE_B" \
  -password "$PASSWORD_B" \
  -user-name "异步上传 SaaS 管理员B" \
  -role-name "异步上传 SaaS 超管B" \
  -package-code "async-upload-b" \
  -package-name "异步上传B版" \
  -storage-mb 20 \
  -async-executions 5 >"$WORK_DIR/bootstrap-b.out"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat_async_file_upload_saas_check <<SQL
INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_A_ID, '异步上传 SaaS 企业A', 'ww-async-upload-saas-a', 'employee-secret-a', 'contact-secret-a', 'callback-token-a', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', $TENANT_A_ID, NOW(), NOW(), NULL),
  ($CORP_B_ID, '异步上传 SaaS 企业B', 'ww-async-upload-saas-b', 'employee-secret-b', 'contact-secret-b', 'callback-token-b', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', $TENANT_B_ID, NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ENABLE_ALL_MIGRATED_ROUTES=0 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_GO_ENABLE_ASYNC_FILE_UPLOAD_WORKER=1 \
  MOCHAT_GO_WORKER_PROCESSING_TIMEOUT_SECONDS=2 \
  MOCHAT_FILE_STORAGE_ROOT="$WORK_DIR/storage" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
curl -sS -f "http://$GO_ADDR/compat/status" >"$WORK_DIR/status.json"
python3 - "$WORK_DIR/status.json" <<'PY'
import json
import pathlib
import sys

status = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tasks = {task.get("name"): task for task in status.get("background_tasks", [])}
task = tasks.get("async-file-upload")
assert task, status
assert task.get("status") == "running", task
assert task.get("run_id"), task
PY

enqueue_upload() {
  local label="$1"
  local tenant_id="$2"
  local corp_id="$3"
  local target_path="$4"
  local content="$5"
  local source="$WORK_DIR/source-$label.txt"
  local event="$WORK_DIR/event-$label.json"
  printf "%s" "$content" >"$source"
  python3 - "$event" "$source" "$target_path" "$tenant_id" "$corp_id" <<'PY'
import json
import pathlib
import sys

target = pathlib.Path(sys.argv[1])
source = sys.argv[2]
target_path = sys.argv[3]
tenant_id = int(sys.argv[4])
corp_id = int(sys.argv[5])
payload = {
    "source": "file_upload_queue",
    "files": [[source, target_path, 1]],
}
if tenant_id > 0:
    payload["tenantId"] = tenant_id
if corp_id > 0:
    payload["corpId"] = corp_id
target.write_text(json.dumps(payload, ensure_ascii=False), encoding="utf-8")
PY
  compose exec -T redis redis-cli -x RPUSH mochat-go:async-file-upload <"$event" >/dev/null
  wait_file_content "$WORK_DIR/storage/$target_path" "$content"
  wait_file_absent "$source"
}

enqueue_upload "tenant-a-1" "$TENANT_A_ID" 0 "tenant-a/1.txt" "tenant A payload 1"
enqueue_upload "tenant-a-2" 0 "$CORP_A_ID" "tenant-a/2.txt" "tenant A payload 2"
enqueue_upload "tenant-b-1" 0 "$CORP_B_ID" "tenant-b/1.txt" "tenant B payload 1"

wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE tenant_id = $TENANT_A_ID AND task_name = 'async-file-upload' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "2"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE tenant_id = $TENANT_B_ID AND task_name = 'async-file-upload' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE tenant_id NOT IN ($TENANT_A_ID, $TENANT_B_ID) AND task_name = 'async-file-upload' AND kind = 'queue_item'" "0"
wait_mysql_scalar "SELECT CONCAT(used_value, '/', limit_value, '/', updated_by) FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_A_ID AND metric = 'async_executions' AND period_key = 'lifetime' AND deleted_at IS NULL" "2/1/runtime"
wait_mysql_scalar "SELECT CONCAT(used_value, '/', limit_value, '/', updated_by) FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_B_ID AND metric = 'async_executions' AND period_key = 'lifetime' AND deleted_at IS NULL" "1/5/runtime"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_A_ID AND source = 'file_upload_queue' AND relative_path IN ('tenant-a/1.txt', 'tenant-a/2.txt') AND size_bytes > 0 AND deleted_at IS NULL" "2"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_B_ID AND source = 'file_upload_queue' AND relative_path = 'tenant-b/1.txt' AND size_bytes > 0 AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_A_ID AND relative_path LIKE 'tenant-b/%' AND deleted_at IS NULL" "0"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_B_ID AND relative_path LIKE 'tenant-a/%' AND deleted_at IS NULL" "0"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_A_ID AND corp_id = $CORP_A_ID AND relative_path = 'tenant-a/2.txt' AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_B_ID AND corp_id = $CORP_B_ID AND relative_path = 'tenant-b/1.txt' AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT CONCAT(used_value, '/', limit_value, '/', updated_by) FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_A_ID AND metric = 'storage_mb' AND period_key = 'lifetime' AND deleted_at IS NULL" "1/20/runtime"
wait_mysql_scalar "SELECT CONCAT(used_value, '/', limit_value, '/', updated_by) FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_B_ID AND metric = 'storage_mb' AND period_key = 'lifetime' AND deleted_at IS NULL" "1/20/runtime"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alerts WHERE tenant_id = $TENANT_A_ID AND metric = 'async_executions' AND alert_type = 'quota_exceeded' AND status = 'open' AND current_value = 2 AND limit_value = 1 AND occurrence_count = 1 AND source = 'worker.queue_item' AND message LIKE '%异步执行量 2/1%'" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alerts WHERE tenant_id = $TENANT_B_ID AND metric = 'async_executions' AND alert_type = 'quota_exceeded' AND deleted_at IS NULL" "0"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alerts WHERE tenant_id NOT IN ($TENANT_A_ID, $TENANT_B_ID) AND metric = 'async_executions' AND deleted_at IS NULL" "0"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alerts WHERE metric = 'async_executions' AND alert_type = 'quota_exceeded' AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_tasks WHERE name = 'async-file-upload' AND status = 'running'" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_task_runs WHERE name = 'async-file-upload' AND status = 'running' AND run_id <> ''" "1"

echo "async file upload SaaS tenant isolation smoke passed"
