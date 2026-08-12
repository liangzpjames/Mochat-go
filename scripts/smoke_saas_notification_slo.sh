#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-notification-slo-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13379}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26429}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18149}"
DATABASE="mochat_notification_slo"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-notification-slo.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-notification-slo-secret}"

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
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B "$DATABASE" -e "$1" | tr -d '\r'
}

login_token() {
  local phone="$1"
  local password="$2"
  local output="$3"
  curl -sS -f \
    -H "Content-Type: application/json" \
    -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" \
    "http://$GO_ADDR/dashboard/user/auth" >"$output"
  python3 - "$output" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("code") == 200, payload
token = payload.get("data", {}).get("token", "")
assert token, payload
print(token)
PY
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

test "$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = 'mochat' AND TABLE_NAME = 'mochat_go_saas_alert_notifications' AND INDEX_NAME = 'idx_mochat_go_saas_alert_notifications_slo_window'" | tr -d '\r')" = "5"

mysql_root <<SQL
DROP DATABASE IF EXISTS $DATABASE;
CREATE DATABASE $DATABASE CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON $DATABASE.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/$DATABASE?parseTime=true&loc=Local"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0037_saas_notification_health_index\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0038_saas_notification_slo_index\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0039_saas_subscription_lifecycle\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0040_saas_payment_collection\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = '$DATABASE' AND TABLE_NAME = 'mochat_go_saas_alert_notifications' AND INDEX_NAME = 'idx_mochat_go_saas_alert_notifications_slo_window'")" = "5"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,SLO平台租户,13800000001,secret001,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
901,SLO违约租户,13800000901,secret901,违约租户管理员,SaaS租户超级管理员,growth,增长版,2,10,1000,50,3,20,20,20,20,20,20,20,20,20,20,20,20,20,512,20,20,20,20,20,2,1000,2037-01-01,missing
902,SLO达标租户,13800000902,secret902,达标租户管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
903,SLO无数据租户,13800000903,secret903,无数据租户管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
CSV

"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"
test "$(grep -c $'^tenant_id\t' "$WORK_DIR/bootstrap.out")" = "4"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  ('smoke-slo-901-today-delivered-1', 'smoke-slo-901', 901, 'webhook', 'delivered', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), '', NULL, DATE_ADD(DATE_SUB(NOW(), INTERVAL 5 MINUTE), INTERVAL 30 SECOND), DATE_SUB(NOW(), INTERVAL 5 MINUTE), NOW(), NULL),
  ('smoke-slo-901-today-delivered-2', 'smoke-slo-901', 901, 'webhook', 'delivered', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), '', NULL, DATE_ADD(DATE_SUB(NOW(), INTERVAL 4 MINUTE), INTERVAL 60 SECOND), DATE_SUB(NOW(), INTERVAL 4 MINUTE), NOW(), NULL),
  ('smoke-slo-901-today-delivered-3', 'smoke-slo-901', 901, 'webhook', 'delivered', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), '', NULL, DATE_ADD(DATE_SUB(NOW(), INTERVAL 3 MINUTE), INTERVAL 90 SECOND), DATE_SUB(NOW(), INTERVAL 3 MINUTE), NOW(), NULL),
  ('smoke-slo-901-today-failed', 'smoke-slo-901', 901, 'webhook', 'failed', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), 'temporary failure', NOW(), NULL, DATE_SUB(NOW(), INTERVAL 2 MINUTE), NOW(), NULL),
  ('smoke-slo-901-today-dead', 'smoke-slo-901', 901, 'webhook', 'dead', 3, 3, JSON_OBJECT('AlertType', 'quota_warning'), 'retry exhausted', NULL, NULL, DATE_SUB(NOW(), INTERVAL 2 MINUTE), NOW(), NULL),
  ('smoke-slo-901-today-closed', 'smoke-slo-901', 901, 'webhook', 'closed', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), 'closed by operator', NULL, NULL, DATE_SUB(NOW(), INTERVAL 1 MINUTE), NOW(), NULL),
  ('smoke-slo-901-yesterday-delivered-1', 'smoke-slo-901', 901, 'webhook', 'delivered', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), '', NULL, DATE_ADD(DATE_SUB(NOW(), INTERVAL 1 DAY), INTERVAL 30 SECOND), DATE_SUB(NOW(), INTERVAL 1 DAY), NOW(), NULL),
  ('smoke-slo-901-yesterday-delivered-2', 'smoke-slo-901', 901, 'webhook', 'delivered', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), '', NULL, DATE_ADD(DATE_SUB(NOW(), INTERVAL 1 DAY), INTERVAL 60 SECOND), DATE_SUB(NOW(), INTERVAL 1 DAY), NOW(), NULL),
  ('smoke-slo-901-yesterday-delivered-3', 'smoke-slo-901', 901, 'webhook', 'delivered', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), '', NULL, DATE_ADD(DATE_SUB(NOW(), INTERVAL 1 DAY), INTERVAL 120 SECOND), DATE_SUB(NOW(), INTERVAL 1 DAY), NOW(), NULL),
  ('smoke-slo-901-yesterday-delivered-slow', 'smoke-slo-901', 901, 'webhook', 'delivered', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), '', NULL, DATE_ADD(DATE_SUB(NOW(), INTERVAL 1 DAY), INTERVAL 600 SECOND), DATE_SUB(NOW(), INTERVAL 1 DAY), NOW(), NULL),
  ('smoke-slo-902-today-delivered-1', 'smoke-slo-902', 902, 'webhook', 'delivered', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), '', NULL, DATE_ADD(DATE_SUB(NOW(), INTERVAL 5 MINUTE), INTERVAL 30 SECOND), DATE_SUB(NOW(), INTERVAL 5 MINUTE), NOW(), NULL),
  ('smoke-slo-902-today-delivered-2', 'smoke-slo-902', 902, 'webhook', 'delivered', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), '', NULL, DATE_ADD(DATE_SUB(NOW(), INTERVAL 4 MINUTE), INTERVAL 60 SECOND), DATE_SUB(NOW(), INTERVAL 4 MINUTE), NOW(), NULL),
  ('smoke-slo-903-suppressed', 'smoke-slo-903', 903, 'webhook', 'suppressed', 0, 3, JSON_OBJECT('AlertType', 'quota_warning'), 'policy suppressed', NULL, NULL, DATE_SUB(NOW(), INTERVAL 3 MINUTE), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
  MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
PLATFORM_TOKEN="$(login_token 13800000001 secret001 "$WORK_DIR/platform-auth.json")"
TENANT_TOKEN="$(login_token 13800000901 secret901 "$WORK_DIR/tenant-auth.json")"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationSlo?days=3&successRateTarget=0.9&latencySecondsTarget=300&latencyRateTarget=0.75&limit=10" >"$WORK_DIR/slo.json"

python3 - "$WORK_DIR/slo.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("code") == 200, payload
data = payload["data"]
assert data["objectives"] == {
    "successRateTarget": 0.9,
    "latencySecondsTarget": 300,
    "latencyRateTarget": 0.75,
}, data
assert data["measurementBasis"]["cohort"] == "created_at", data
assert "closed" in data["measurementBasis"]["attemptedStatuses"], data
summary = data["summary"]
assert summary["attemptedCount"] == 12, summary
assert summary["deliveredCount"] == 9, summary
assert summary["closedCount"] == 1, summary
assert summary["deliverySuccessRate"] == 0.75, summary
assert summary["deliveredWithinTarget"] == 8, summary
assert summary["latencyAttainmentRate"] == 0.8889, summary
assert summary["sloState"] == "breached", summary
assert summary["dayCount"] == 3, summary
assert summary["metDayCount"] == 1, summary
assert summary["breachedDayCount"] == 1, summary
assert summary["noDataDayCount"] == 1, summary
assert summary["tenantCount"] == 4, summary
assert summary["metTenantCount"] == 1, summary
assert summary["breachedTenantCount"] == 1, summary
assert summary["noDataTenantCount"] == 2, summary
days = data["days"]
assert len(days) == 3, days
assert [item["sloState"] for item in days] == ["no_data", "met", "breached"], days
tenants = data["tenants"]
assert [item["tenantId"] for item in tenants] == [901, 902, 1, 903], tenants
assert [item["sloState"] for item in tenants] == ["breached", "met", "no_data", "no_data"], tenants
assert tenants[0]["attemptedCount"] == 10, tenants[0]
assert tenants[0]["deliverySuccessRate"] == 0.7, tenants[0]
assert tenants[0]["latencyAttainmentRate"] == 0.8571, tenants[0]
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationSlo?days=3&successRateTarget=0.9&latencySecondsTarget=300&latencyRateTarget=0.75&keyword=%E8%BE%BE%E6%A0%87&limit=10" >"$WORK_DIR/slo-filtered.json"

python3 - "$WORK_DIR/slo-filtered.json" <<'PY'
import json
import pathlib
import sys

data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
summary = data["summary"]
assert summary["tenantCount"] == 1, summary
assert summary["metTenantCount"] == 1, summary
assert summary["attemptedCount"] == 2, summary
assert summary["deliverySuccessRate"] == 1, summary
assert [item["tenantId"] for item in data["tenants"]] == [902], data
assert [item["sloState"] for item in data["days"]] == ["no_data", "no_data", "met"], data
PY

curl -sS -f \
  -D "$WORK_DIR/slo-csv.headers" \
  -o "$WORK_DIR/slo.csv" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=notificationSlo&days=3&successRateTarget=0.9&latencySecondsTarget=300&latencyRateTarget=0.75&limit=1000"
grep -qi 'filename="mochat-saas-notificationSlo-' "$WORK_DIR/slo-csv.headers"

python3 - "$WORK_DIR/slo.csv" <<'PY'
import csv
import pathlib
import sys

text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8-sig")
rows = list(csv.DictReader(text.splitlines()))
assert rows, rows
assert rows[0]["section"] == "summary", rows[0]
assert rows[0]["attemptedCount"] == "12", rows[0]
assert rows[0]["closedCount"] == "1", rows[0]
tenant_901 = next(row for row in rows if row["section"] == "tenant" and row["tenantId"] == "901")
assert tenant_901["sloState"] == "breached", tenant_901
assert tenant_901["deliverySuccessRate"] == "0.7000", tenant_901
PY

TENANT_STATUS="$(curl -sS -o "$WORK_DIR/tenant-forbidden.json" -w '%{http_code}' -H "Authorization: Bearer $TENANT_TOKEN" "http://$GO_ADDR/dashboard/saasAdmin/notificationSlo")"
test "$TENANT_STATUS" = "403"

INVALID_STATUS="$(curl -sS -o "$WORK_DIR/invalid.json" -w '%{http_code}' -H "Authorization: Bearer $PLATFORM_TOKEN" "http://$GO_ADDR/dashboard/saasAdmin/notificationSlo?successRateTarget=0.1")"
test "$INVALID_STATUS" = "400"

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/page.html"
grep -q 'id="notificationSloWindowDays"' "$WORK_DIR/page.html"
grep -q 'id="notificationSloDays"' "$WORK_DIR/page.html"
grep -q 'id="notificationSloTenants"' "$WORK_DIR/page.html"
grep -q "fetch('/dashboard/saasAdmin/notificationSlo?'" "$WORK_DIR/page.html"

curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
grep -q "GET /dashboard/saasAdmin/notificationSlo" "$WORK_DIR/routes.json"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/notificationSlo" "$GO_LOG"

echo "SaaS notification SLO smoke passed"
