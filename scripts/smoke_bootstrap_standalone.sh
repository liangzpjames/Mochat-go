#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-bootstrap-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13333}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26392}"
GO_ADDR="${MOCHAT_BOOTSTRAP_GO_ADDR:-127.0.0.1:18101}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-bootstrap.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""

SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-bootstrap-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800000000}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret123}"
TENANT_ID="${MOCHAT_BOOTSTRAP_TENANT_ID:-1}"

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
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat_bootstrap_check -e "$query" | tr -d '\r'
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

mysql_root <<'SQL'
DROP DATABASE IF EXISTS mochat_bootstrap_check;
CREATE DATABASE mochat_bootstrap_check CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON mochat_bootstrap_check.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat_bootstrap_check?parseTime=true&loc=Local"

"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0001_initial_schema\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0002_seed_core_data\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0003_saas_provisioning\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0004_saas_storage_objects\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0005_background_tasks\tapplied_now' "$WORK_DIR/migrate.out"
EXPECTED_MENU_COUNT="$(mysql_scalar "SELECT COUNT(*) FROM mc_rbac_menu WHERE deleted_at IS NULL")"
test "$EXPECTED_MENU_COUNT" -gt 0

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id "$TENANT_ID" \
  -tenant-name "Bootstrap验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "Bootstrap管理员" \
  -role-name "Bootstrap超级管理员" \
  -package-code "bootstrap-standard" \
  -package-name "Bootstrap标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -channel-codes 120 \
  -shop-codes 45 \
  -radars 33 \
  -lotteries 22 \
  -room-infinite-pulls 24 \
  -room-fissions 26 \
  -room-clock-ins 28 \
  -room-qualities 30 \
  -room-calendars 32 \
  -room-reminds 34 \
  -contact-sops 36 \
  -room-sops 38 \
  -sensitive-words 39 \
  -storage-mb 1024 \
  -contact-message-batches 300 \
  -room-message-batches 150 \
  -room-tag-pulls 80 \
  -work-room-auto-pulls 60 \
  -work-fissions 40 \
  -official-accounts 10 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"
grep -q $'tenant_id\t'"$TENANT_ID" "$WORK_DIR/bootstrap.out"
grep -q $'role_menu_count\t'"$EXPECTED_MENU_COUNT" "$WORK_DIR/bootstrap.out"
grep -q $'package_code\tbootstrap-standard' "$WORK_DIR/bootstrap.out"
grep -q $'usage_metric_count\t26' "$WORK_DIR/bootstrap.out"
grep -q $'seed_version_count\t3' "$WORK_DIR/bootstrap.out"

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id "$TENANT_ID" \
  -tenant-name "Bootstrap验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "Bootstrap管理员" \
  -role-name "Bootstrap超级管理员" \
  -package-code "bootstrap-standard" \
  -package-name "Bootstrap标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -channel-codes 120 \
  -shop-codes 45 \
  -radars 33 \
  -lotteries 22 \
  -room-infinite-pulls 24 \
  -room-fissions 26 \
  -room-clock-ins 28 \
  -room-qualities 30 \
  -room-calendars 32 \
  -room-reminds 34 \
  -contact-sops 36 \
  -room-sops 38 \
  -sensitive-words 39 \
  -storage-mb 1024 \
  -contact-message-batches 300 \
  -room-message-batches 150 \
  -room-tag-pulls 80 \
  -work-room-auto-pulls 60 \
  -work-fissions 40 \
  -official-accounts 10 \
  -async-executions 10000 >"$WORK_DIR/bootstrap-again.out"
grep -q $'role_menu_count\t'"$EXPECTED_MENU_COUNT" "$WORK_DIR/bootstrap-again.out"
grep -q $'usage_metric_count\t26' "$WORK_DIR/bootstrap-again.out"
grep -q $'seed_version_count\t3' "$WORK_DIR/bootstrap-again.out"

USER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '$PHONE' AND tenant_id = $TENANT_ID AND status = 1 AND isSuperAdmin = 1 AND deleted_at IS NULL")"
ROLE_ID="$(mysql_scalar "SELECT id FROM mc_rbac_role WHERE tenant_id = $TENANT_ID AND name = 'Bootstrap超级管理员' AND status = 1 AND deleted_at IS NULL")"
test -n "$USER_ID"
test -n "$ROLE_ID"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_tenant WHERE id = $TENANT_ID AND status = 1 AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_user WHERE phone = '$PHONE' AND tenant_id = $TENANT_ID AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_rbac_role WHERE tenant_id = $TENANT_ID AND name = 'Bootstrap超级管理员' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_rbac_role_menu WHERE role_id = $ROLE_ID")" = "$EXPECTED_MENU_COUNT"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_rbac_user_role WHERE user_id = $USER_ID AND role_id = $ROLE_ID AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_packages WHERE code = 'bootstrap-standard' AND max_users = 25 AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_packages WHERE tenant_id = $TENANT_ID AND package_code = 'bootstrap-standard' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND period_key = 'lifetime' AND deleted_at IS NULL")" = "26"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'users' AND deleted_at IS NULL")" = "25"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'channel_codes' AND deleted_at IS NULL")" = "120"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'shop_codes' AND deleted_at IS NULL")" = "45"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'radars' AND deleted_at IS NULL")" = "33"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'lotteries' AND deleted_at IS NULL")" = "22"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_infinite_pulls' AND deleted_at IS NULL")" = "24"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_fissions' AND deleted_at IS NULL")" = "26"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_clock_ins' AND deleted_at IS NULL")" = "28"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_qualities' AND deleted_at IS NULL")" = "30"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_calendars' AND deleted_at IS NULL")" = "32"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_reminds' AND deleted_at IS NULL")" = "34"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'contact_sops' AND deleted_at IS NULL")" = "36"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_sops' AND deleted_at IS NULL")" = "38"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'sensitive_words' AND deleted_at IS NULL")" = "39"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'contact_message_batches' AND deleted_at IS NULL")" = "300"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_message_batches' AND deleted_at IS NULL")" = "150"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_tag_pulls' AND deleted_at IS NULL")" = "80"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'work_room_auto_pulls' AND deleted_at IS NULL")" = "60"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'work_fissions' AND deleted_at IS NULL")" = "40"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'official_accounts' AND deleted_at IS NULL")" = "10"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'async_executions' AND deleted_at IS NULL")" = "10000"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'users' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_seed_versions WHERE scope = 'tenant' AND target_id = $TENANT_ID")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_tenant_provision_runs WHERE tenant_id = $TENANT_ID AND package_code = 'bootstrap-standard' AND status = 1")" = "1"

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
curl -sS -f \
  -H "Content-Type: application/json" \
  -d "{\"phone\":\"$PHONE\",\"password\":\"$PASSWORD\"}" \
  "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/auth.json"

python3 - "$WORK_DIR/auth.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
assert payload["data"]["token"], payload
assert payload["data"]["expire"] > 0, payload
print("bootstrap standalone smoke passed")
PY
