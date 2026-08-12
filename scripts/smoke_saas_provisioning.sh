#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-provision-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13334}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26470}"
GO_ADDR="${MOCHAT_SAAS_GO_ADDR:-127.0.0.1:18102}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-provision.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""

SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-provision-secret}"

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
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat_saas_check -e "$query" | tr -d '\r'
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

mysql_root <<'SQL'
DROP DATABASE IF EXISTS mochat_saas_check;
CREATE DATABASE mochat_saas_check CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON mochat_saas_check.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat_saas_check?parseTime=true&loc=Local"

"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0003_saas_provisioning\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0004_saas_storage_objects\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0005_background_tasks\tapplied_now' "$WORK_DIR/migrate.out"
EXPECTED_MENU_COUNT="$(mysql_scalar "SELECT COUNT(*) FROM mc_rbac_menu WHERE deleted_at IS NULL")"
test "$EXPECTED_MENU_COUNT" -gt 0

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
201,SaaS批量租户A,13800000201,secret201,SaaS管理员A,SaaS超级管理员,growth,增长版,2,10,1000,50,3,12,7,9,11,13,15,17,19,21,23,25,27,29,512,30,20,8,6,4,2,1000,2027-01-01,missing
202,SaaS批量租户B,13800000202,secret202,SaaS管理员B,SaaS超级管理员,enterprise,企业版,5,50,20000,500,10,100,70,90,110,130,150,170,190,210,230,250,270,290,2048,300,200,80,60,40,20,10000,,overwrite
CSV

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap-batch.out"

test "$(grep -c $'^tenant_id\t' "$WORK_DIR/bootstrap-batch.out")" = "2"
grep -q $'tenant_id\t201' "$WORK_DIR/bootstrap-batch.out"
grep -q $'tenant_id\t202' "$WORK_DIR/bootstrap-batch.out"
grep -q $'package_code\tgrowth' "$WORK_DIR/bootstrap-batch.out"
grep -q $'package_code\tenterprise' "$WORK_DIR/bootstrap-batch.out"
test "$(grep -c $'^usage_metric_count\t26' "$WORK_DIR/bootstrap-batch.out")" = "2"
test "$(grep -c $'^seed_version_count\t3' "$WORK_DIR/bootstrap-batch.out")" = "2"

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_tenant WHERE id IN (201, 202) AND status = 1 AND deleted_at IS NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_user WHERE tenant_id IN (201, 202) AND isSuperAdmin = 1 AND status = 1 AND deleted_at IS NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_rbac_role WHERE tenant_id IN (201, 202) AND name = 'SaaS超级管理员' AND deleted_at IS NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_rbac_role_menu rm JOIN mc_rbac_role r ON r.id = rm.role_id WHERE r.tenant_id IN (201, 202)")" = "$((EXPECTED_MENU_COUNT * 2))"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_packages WHERE code IN ('growth', 'enterprise') AND deleted_at IS NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_packages WHERE tenant_id IN (201, 202) AND deleted_at IS NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_usage_counters WHERE tenant_id IN (201, 202) AND period_key = 'lifetime' AND deleted_at IS NULL")" = "52"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 201 AND metric = 'users' AND deleted_at IS NULL")" = "10"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 201 AND metric = 'channel_codes' AND deleted_at IS NULL")" = "12"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'shop_codes' AND deleted_at IS NULL")" = "70"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'radars' AND deleted_at IS NULL")" = "90"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'lotteries' AND deleted_at IS NULL")" = "110"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'room_infinite_pulls' AND deleted_at IS NULL")" = "130"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'room_fissions' AND deleted_at IS NULL")" = "150"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'room_clock_ins' AND deleted_at IS NULL")" = "170"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'room_qualities' AND deleted_at IS NULL")" = "190"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'room_calendars' AND deleted_at IS NULL")" = "210"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'room_reminds' AND deleted_at IS NULL")" = "230"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 201 AND metric = 'contact_sops' AND deleted_at IS NULL")" = "25"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'room_sops' AND deleted_at IS NULL")" = "270"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 201 AND metric = 'sensitive_words' AND deleted_at IS NULL")" = "29"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'sensitive_words' AND deleted_at IS NULL")" = "290"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'storage_mb' AND deleted_at IS NULL")" = "2048"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 201 AND metric = 'contact_message_batches' AND deleted_at IS NULL")" = "30"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'room_message_batches' AND deleted_at IS NULL")" = "200"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 201 AND metric = 'room_tag_pulls' AND deleted_at IS NULL")" = "8"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'work_room_auto_pulls' AND deleted_at IS NULL")" = "60"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'work_fissions' AND deleted_at IS NULL")" = "40"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 201 AND metric = 'official_accounts' AND deleted_at IS NULL")" = "2"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 202 AND metric = 'async_executions' AND deleted_at IS NULL")" = "10000"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 201 AND metric = 'users' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_seed_versions WHERE scope = 'tenant' AND target_id IN (201, 202)")" = "6"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_tenant_provision_runs WHERE tenant_id IN (201, 202) AND status = 1")" = "2"

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

for item in "13800000201 secret201" "13800000202 secret202"; do
  set -- $item
  phone="$1"
  password="$2"
  curl -sS -f \
    -H "Content-Type: application/json" \
    -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" \
    "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/auth-$phone.json"
  python3 - "$WORK_DIR/auth-$phone.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
assert payload["data"]["token"], payload
assert payload["data"]["expire"] > 0, payload
PY
done

echo "saas provisioning smoke passed"
