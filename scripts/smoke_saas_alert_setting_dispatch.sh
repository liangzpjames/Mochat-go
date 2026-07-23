#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-alert-setting-dispatch-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13350}"
ALERT_WEBHOOK_ADDR="${MOCHAT_ALERT_WEBHOOK_ADDR:-127.0.0.1:19120}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-alert-setting-dispatch.XXXXXX")"
MAINTENANCE_BIN="$WORK_DIR/mochat-saas-maintenance"
ALERT_WEBHOOK_EVENTS="$WORK_DIR/alert-webhook-events.jsonl"
ALERT_WEBHOOK_PID=""
ALERT_CREDENTIAL_KEY_Q1="6161616161616161616161616161616161616161616161616161616161616161"
ALERT_CREDENTIAL_KEY_Q2="6262626262626262626262626262626262626262626262626262626262626262"

export MOCHAT_GO_SAAS_ALERT_WEBHOOK_REQUIRE_HTTPS=0
export MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS=127.0.0.0/8

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$ALERT_WEBHOOK_PID" ] && kill -0 "$ALERT_WEBHOOK_PID" 2>/dev/null; then
    kill "$ALERT_WEBHOOK_PID" 2>/dev/null || true
    wait "$ALERT_WEBHOOK_PID" 2>/dev/null || true
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

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "$1" | tr -d '\r'
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
  echo "timed out waiting for MySQL query to return $expected" >&2
  echo "$query" >&2
  echo "last value: $(mysql_scalar "$query" || true)" >&2
  exit 1
}

wait_file_line_count() {
  local file="$1"
  local expected="$2"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local count
    count="$(python3 - "$file" <<'PY'
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
if not path.exists():
    print(0)
else:
    print(len([line for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]))
PY
)"
    if [ "$count" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $file to have $expected lines" >&2
  [ -f "$file" ] && cat "$file" >&2 || true
  exit 1
}

assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "$ALERT_WEBHOOK_ADDR"

compose up -d mysql
wait_service_healthy mysql
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = 'mochat' AND table_name = 'mochat_go_saas_alert_settings' AND column_name IN ('webhook_credentials_ciphertext', 'webhook_credentials_key_id')")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = 'mochat' AND table_name = 'mochat_go_saas_alert_settings' AND index_name = 'idx_mochat_go_saas_alert_settings_credential_key'")" = "3"

python3 - "$ALERT_WEBHOOK_ADDR" "$ALERT_WEBHOOK_EVENTS" <<'PY' &
import json
import pathlib
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

host, port_text = sys.argv[1].rsplit(":", 1)
events_path = pathlib.Path(sys.argv[2])

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("content-length", "0") or "0")
        body = self.rfile.read(length)
        event = {
            "headers": {
                "event": self.headers.get("X-Mochat-Go-Event", ""),
                "timestamp": self.headers.get("X-Mochat-Go-Timestamp", ""),
                "signature": self.headers.get("X-Mochat-Go-Signature", ""),
            },
            "body": json.loads(body.decode("utf-8")),
        }
        with events_path.open("a", encoding="utf-8") as f:
            f.write(json.dumps(event, ensure_ascii=False) + "\n")
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"ok")

    def log_message(self, fmt, *args):
        return

ThreadingHTTPServer((host, int(port_text)), Handler).serve_forever()
PY
ALERT_WEBHOOK_PID="$!"
sleep 1

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
DELETE FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-setting-notification';
DELETE FROM mochat_go_saas_alert_settings WHERE tenant_id = 1 AND channel = 'webhook';
INSERT INTO mochat_go_saas_alert_settings
  (tenant_id, channel, enabled, webhook_url, webhook_secret, webhook_timeout_seconds, webhook_retry_attempts, webhook_retry_delay_ms, webhook_title_template, webhook_body_template, notification_max_attempts, notification_retry_delay_seconds, minimum_severity, allowed_alert_types_json, quiet_hours_enabled, quiet_hours_start, quiet_hours_end, timezone, hourly_limit, created_at, updated_at, deleted_at)
VALUES
  (1, 'webhook', 1, 'http://$ALERT_WEBHOOK_ADDR/tenant-alert', 'tenant-secret', 2, 1, 0, '租户 {{.TenantID}} {{.Metric}} 告警', '配置表投递 {{.CurrentValue}}/{{.LimitValue}} 来源 {{.Source}}', 3, 1, 'warning', '[]', 0, '22:00', '08:00', 'UTC', 0, NOW(), NOW(), NULL);
INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  (
    'smoke-setting-notification',
    'smoke-setting-alert',
    1,
    'webhook',
    'pending',
    0,
    3,
    '{"Status":{"Metric":"async_executions","TenantID":1,"Current":9,"Limit":3,"Additional":0},"AlertType":"quota_exceeded","Severity":"warning","PeriodKey":"lifetime","Source":"setting.outbox","Message":"配置表待发送告警","Context":{"tenantSetting":true}}',
    '',
    NOW(),
    NULL,
    NOW(),
    NOW(),
    NULL
  );
SQL

env -u GOROOT go build -o "$MAINTENANCE_BIN" ./cmd/mochat-saas-maintenance

env -u GOROOT \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  "$MAINTENANCE_BIN" \
  -action dispatch-alert-notifications \
  -alert-limit 5 \
  -alert-notification-retry-delay 1s >"$WORK_DIR/dispatch.out"

grep -q $'action\tdispatch-alert-notifications' "$WORK_DIR/dispatch.out"
grep -q $'scanned\t1' "$WORK_DIR/dispatch.out"
grep -q $'delivered\t1' "$WORK_DIR/dispatch.out"
grep -q $'deferred\t0' "$WORK_DIR/dispatch.out"
grep -q $'suppressed\t0' "$WORK_DIR/dispatch.out"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "1"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts, '/', max_attempts) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-setting-notification' AND tenant_id = 1 AND channel = 'webhook' AND deleted_at IS NULL" "delivered/1/3"

python3 - "$ALERT_WEBHOOK_EVENTS" <<'PY'
import json
import pathlib
import sys

line = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").strip()
event = json.loads(line)
headers = event["headers"]
body = event["body"]
assert headers["event"] == "saas.quota_alert", headers
assert headers["timestamp"], headers
assert headers["signature"].startswith("v1="), headers
assert body["tenantId"] == 1, body
assert body["metric"] == "async_executions", body
assert body["title"] == "租户 1 async_executions 告警", body
assert body["body"] == "配置表投递 9/3 来源 setting.outbox", body
assert body["context"]["tenantSetting"] is True, body
PY

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
UPDATE mochat_go_saas_alert_settings
SET minimum_severity = 'critical',
    allowed_alert_types_json = '["tenant_renewal_reminder"]',
    quiet_hours_enabled = 0,
    hourly_limit = 0
WHERE tenant_id = 1 AND channel = 'webhook' AND deleted_at IS NULL;

INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  (
    'smoke-policy-type-suppressed',
    '1:async_executions:quota_exceeded:smoke-policy-type',
    1,
    'webhook',
    'pending',
    0,
    3,
    '{"Status":{"Metric":"async_executions","TenantID":1,"Current":9,"Limit":3,"Additional":0},"AlertType":"quota_exceeded","Severity":"critical","PeriodKey":"smoke-policy-type","Source":"setting.policy","Message":"事件类型过滤","Context":{}}',
    '',
    NOW(),
    NULL,
    NOW(),
    NOW(),
    NULL
  ),
  (
    'smoke-policy-severity-suppressed',
    '1:tenant_renewal:tenant_renewal_reminder:smoke-policy-severity',
    1,
    'webhook',
    'pending',
    0,
    3,
    '{"Status":{"Metric":"tenant_renewal","TenantID":1,"Current":0,"Limit":0,"Additional":0},"AlertType":"tenant_renewal_reminder","Severity":"warning","PeriodKey":"smoke-policy-severity","Source":"setting.policy","Message":"严重级别过滤","Context":{}}',
    '',
    NOW(),
    NULL,
    NOW(),
    NOW(),
    NULL
  );
SQL

env -u GOROOT \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  "$MAINTENANCE_BIN" \
  -action dispatch-alert-notifications \
  -alert-limit 5 \
  -alert-notification-retry-delay 1s >"$WORK_DIR/dispatch-policy-suppressed.out"

grep -q $'scanned\t2' "$WORK_DIR/dispatch-policy-suppressed.out"
grep -q $'delivered\t0' "$WORK_DIR/dispatch-policy-suppressed.out"
grep -q $'deferred\t0' "$WORK_DIR/dispatch-policy-suppressed.out"
grep -q $'suppressed\t2' "$WORK_DIR/dispatch-policy-suppressed.out"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "1"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-policy-type-suppressed'" "suppressed/0"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-policy-severity-suppressed'" "suppressed/0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-policy-type-suppressed' AND last_error LIKE '%not subscribed%'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-policy-severity-suppressed' AND last_error LIKE '%below minimum%'")" = "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
UPDATE mochat_go_saas_alert_settings
SET minimum_severity = 'warning',
    allowed_alert_types_json = '[]',
    quiet_hours_enabled = 1,
    quiet_hours_start = DATE_FORMAT(DATE_SUB(UTC_TIMESTAMP(), INTERVAL 1 MINUTE), '%H:%i'),
    quiet_hours_end = DATE_FORMAT(DATE_ADD(UTC_TIMESTAMP(), INTERVAL 10 MINUTE), '%H:%i'),
    timezone = 'UTC',
    hourly_limit = 0
WHERE tenant_id = 1 AND channel = 'webhook' AND deleted_at IS NULL;

INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  ('smoke-policy-quiet-deferred', '1:async_executions:quota_exceeded:smoke-policy-quiet', 1, 'webhook', 'pending', 0, 3, '{"Status":{"Metric":"async_executions","TenantID":1,"Current":9,"Limit":3,"Additional":0},"AlertType":"quota_exceeded","Severity":"warning","PeriodKey":"smoke-policy-quiet","Source":"setting.policy","Message":"免打扰延期","Context":{}}', '', NOW(), NULL, NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  "$MAINTENANCE_BIN" \
  -action dispatch-alert-notifications \
  -alert-limit 5 \
  -alert-notification-retry-delay 1s >"$WORK_DIR/dispatch-policy-quiet.out"

grep -q $'scanned\t1' "$WORK_DIR/dispatch-policy-quiet.out"
grep -q $'delivered\t0' "$WORK_DIR/dispatch-policy-quiet.out"
grep -q $'deferred\t1' "$WORK_DIR/dispatch-policy-quiet.out"
grep -q $'suppressed\t0' "$WORK_DIR/dispatch-policy-quiet.out"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "1"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts, '/', next_retry_at > NOW()) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-policy-quiet-deferred'" "pending/0/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-policy-quiet-deferred' AND last_error LIKE '%quiet hours%'")" = "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
UPDATE mochat_go_saas_alert_settings
SET quiet_hours_enabled = 0,
    hourly_limit = 1
WHERE tenant_id = 1 AND channel = 'webhook' AND deleted_at IS NULL;

INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  ('smoke-policy-rate-deferred', '1:async_executions:quota_exceeded:smoke-policy-rate', 1, 'webhook', 'pending', 0, 3, '{"Status":{"Metric":"async_executions","TenantID":1,"Current":9,"Limit":3,"Additional":0},"AlertType":"quota_exceeded","Severity":"warning","PeriodKey":"smoke-policy-rate","Source":"setting.policy","Message":"小时频控延期","Context":{}}', '', NOW(), NULL, NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  "$MAINTENANCE_BIN" \
  -action dispatch-alert-notifications \
  -alert-limit 5 \
  -alert-notification-retry-delay 1s >"$WORK_DIR/dispatch-policy-rate.out"

grep -q $'scanned\t1' "$WORK_DIR/dispatch-policy-rate.out"
grep -q $'delivered\t0' "$WORK_DIR/dispatch-policy-rate.out"
grep -q $'deferred\t1' "$WORK_DIR/dispatch-policy-rate.out"
grep -q $'suppressed\t0' "$WORK_DIR/dispatch-policy-rate.out"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "1"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts, '/', next_retry_at > NOW()) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-policy-rate-deferred'" "pending/0/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-policy-rate-deferred' AND last_error LIKE '%hourly limit 1 reached%'")" = "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
UPDATE mochat_go_saas_alert_settings
SET minimum_severity = 'critical',
    allowed_alert_types_json = '["tenant_renewal_reminder"]',
    quiet_hours_enabled = 1,
    quiet_hours_start = DATE_FORMAT(DATE_SUB(UTC_TIMESTAMP(), INTERVAL 1 MINUTE), '%H:%i'),
    quiet_hours_end = DATE_FORMAT(DATE_ADD(UTC_TIMESTAMP(), INTERVAL 10 MINUTE), '%H:%i'),
    timezone = 'UTC',
    hourly_limit = 1
WHERE tenant_id = 1 AND channel = 'webhook' AND deleted_at IS NULL;

INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  ('smoke-policy-test-bypass', '1:notification_policy:notification_policy_test:smoke-policy-test', 1, 'webhook', 'pending', 0, 3, '{"Status":{"Metric":"notification_policy","TenantID":1,"Current":0,"Limit":0,"Additional":0},"AlertType":"notification_policy_test","Severity":"warning","PeriodKey":"smoke-policy-test","Source":"saas_admin","Message":"通知策略测试旁路","Context":{"policyTest":true}}', '', NOW(), NULL, NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  "$MAINTENANCE_BIN" \
  -action dispatch-alert-notifications \
  -alert-limit 5 \
  -alert-notification-retry-delay 1s >"$WORK_DIR/dispatch-policy-test.out"

grep -q $'scanned\t1' "$WORK_DIR/dispatch-policy-test.out"
grep -q $'delivered\t1' "$WORK_DIR/dispatch-policy-test.out"
grep -q $'deferred\t0' "$WORK_DIR/dispatch-policy-test.out"
grep -q $'suppressed\t0' "$WORK_DIR/dispatch-policy-test.out"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "2"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-policy-test-bypass'" "delivered/1"

python3 - "$ALERT_WEBHOOK_EVENTS" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
assert len(events) == 2, events
test_event = events[-1]
assert test_event["headers"]["signature"].startswith("v1="), test_event
body = test_event["body"]
assert body["tenantId"] == 1, body
assert body["metric"] == "notification_policy", body
assert body["alertType"] == "notification_policy_test", body
assert body["context"]["policyTest"] is True, body
PY

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
UPDATE mochat_go_saas_alert_settings
SET minimum_severity = 'warning',
    allowed_alert_types_json = '[]',
    quiet_hours_enabled = 0,
    hourly_limit = 0
WHERE tenant_id = 1 AND channel = 'webhook' AND deleted_at IS NULL;

INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  ('smoke-webhook-ssrf-guard', '1:notification_policy:notification_policy_test:smoke-webhook-ssrf-guard', 1, 'webhook', 'pending', 0, 3, '{"Status":{"Metric":"notification_policy","TenantID":1,"Current":0,"Limit":0,"Additional":0},"AlertType":"notification_policy_test","Severity":"warning","PeriodKey":"smoke-webhook-ssrf-guard","Source":"saas_admin","Message":"Webhook SSRF 防护验收","Context":{"ssrfGuard":true}}', '', NOW(), NULL, NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  "$MAINTENANCE_BIN" \
  -action dispatch-alert-notifications \
  -alert-limit 5 \
  -alert-webhook-require-https=false \
  -alert-webhook-allowed-cidrs '' \
  -alert-notification-retry-delay 1s >"$WORK_DIR/dispatch-ssrf-blocked.out"

grep -q $'scanned\t1' "$WORK_DIR/dispatch-ssrf-blocked.out"
grep -q $'delivered\t0' "$WORK_DIR/dispatch-ssrf-blocked.out"
grep -q $'failed\t1' "$WORK_DIR/dispatch-ssrf-blocked.out"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "2"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-webhook-ssrf-guard'" "failed/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-webhook-ssrf-guard' AND last_error LIKE '%not globally routable%'")" = "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
UPDATE mochat_go_saas_alert_notifications
SET status = 'pending', attempts = 0, last_error = '', next_retry_at = NOW(), updated_at = NOW()
WHERE notification_key = 'smoke-webhook-ssrf-guard';
SQL

env -u GOROOT \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  "$MAINTENANCE_BIN" \
  -action dispatch-alert-notifications \
  -alert-limit 5 \
  -alert-notification-retry-delay 1s >"$WORK_DIR/dispatch-ssrf-allowed.out"

grep -q $'scanned\t1' "$WORK_DIR/dispatch-ssrf-allowed.out"
grep -q $'delivered\t1' "$WORK_DIR/dispatch-ssrf-allowed.out"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "3"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-webhook-ssrf-guard'" "delivered/1"

MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY="$ALERT_CREDENTIAL_KEY_Q1" \
MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID=alert-smoke-q1 \
MOCHAT_GO_SAAS_ALERT_CREDENTIAL_REQUIRE_ENCRYPTION=1 \
MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  "$MAINTENANCE_BIN" \
  -action rotate-alert-credentials \
  -tenant-id 1 \
  -alert-credential-rotation-limit 10 >"$WORK_DIR/rotate-q1.out"

grep -q $'action\trotate-alert-credentials' "$WORK_DIR/rotate-q1.out"
grep -q $'scanned\t1' "$WORK_DIR/rotate-q1.out"
grep -q $'rotated\t1' "$WORK_DIR/rotate-q1.out"
grep -q $'legacy_plaintext\t1' "$WORK_DIR/rotate-q1.out"
grep -q $'reencrypted\t0' "$WORK_DIR/rotate-q1.out"
grep -q $'active_key_id\talert-smoke-q1' "$WORK_DIR/rotate-q1.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_settings WHERE tenant_id = 1 AND channel = 'webhook' AND webhook_url = '' AND webhook_secret = '' AND webhook_credentials_key_id = 'alert-smoke-q1' AND CHAR_LENGTH(webhook_credentials_ciphertext) > 80 AND INSTR(webhook_credentials_ciphertext, 'tenant-secret') = 0")" = "1"
CIPHERTEXT_Q1="$(mysql_scalar "SELECT webhook_credentials_ciphertext FROM mochat_go_saas_alert_settings WHERE tenant_id = 1 AND channel = 'webhook'")"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  ('smoke-encrypted-q1', '1:notification_policy:notification_policy_test:smoke-encrypted-q1', 1, 'webhook', 'pending', 0, 3, '{"Status":{"Metric":"notification_policy","TenantID":1,"Current":0,"Limit":0,"Additional":0},"AlertType":"notification_policy_test","Severity":"warning","PeriodKey":"smoke-encrypted-q1","Source":"saas_admin","Message":"q1 密文投递","Context":{"credentialKey":"q1"}}', '', NOW(), NULL, NOW(), NOW(), NULL);
SQL

MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY="$ALERT_CREDENTIAL_KEY_Q1" \
MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID=alert-smoke-q1 \
MOCHAT_GO_SAAS_ALERT_CREDENTIAL_REQUIRE_ENCRYPTION=1 \
MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  "$MAINTENANCE_BIN" \
  -action dispatch-alert-notifications \
  -alert-limit 5 \
  -alert-notification-retry-delay 1s >"$WORK_DIR/dispatch-encrypted-q1.out"

grep -q $'scanned\t1' "$WORK_DIR/dispatch-encrypted-q1.out"
grep -q $'delivered\t1' "$WORK_DIR/dispatch-encrypted-q1.out"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "4"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-encrypted-q1'" "delivered/1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  ('smoke-missing-q1', '1:notification_policy:notification_policy_test:smoke-missing-q1', 1, 'webhook', 'pending', 0, 3, '{"Status":{"Metric":"notification_policy","TenantID":1,"Current":0,"Limit":0,"Additional":0},"AlertType":"notification_policy_test","Severity":"warning","PeriodKey":"smoke-missing-q1","Source":"saas_admin","Message":"缺失历史密钥拒绝","Context":{"credentialKey":"missing-q1"}}', '', NOW(), NULL, NOW(), NOW(), NULL);
SQL

if MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY="$ALERT_CREDENTIAL_KEY_Q2" \
  MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID=alert-smoke-q2 \
  MOCHAT_GO_SAAS_ALERT_CREDENTIAL_REQUIRE_ENCRYPTION=1 \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  "$MAINTENANCE_BIN" \
  -action dispatch-alert-notifications \
  -alert-limit 5 \
  -alert-notification-retry-delay 1s >"$WORK_DIR/dispatch-missing-q1.out" 2>&1; then
  echo "dispatch unexpectedly accepted a ciphertext without its historical key" >&2
  exit 1
fi
grep -q 'encryption key "alert-smoke-q1" is unavailable' "$WORK_DIR/dispatch-missing-q1.out"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "4"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-missing-q1'" "pending/0"

MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEYS="{\"alert-smoke-q1\":\"$ALERT_CREDENTIAL_KEY_Q1\",\"alert-smoke-q2\":\"$ALERT_CREDENTIAL_KEY_Q2\"}" \
MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID=alert-smoke-q2 \
MOCHAT_GO_SAAS_ALERT_CREDENTIAL_REQUIRE_ENCRYPTION=1 \
MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  "$MAINTENANCE_BIN" \
  -action rotate-alert-credentials \
  -tenant-id 1 \
  -alert-credential-rotation-limit 10 >"$WORK_DIR/rotate-q2.out"

grep -q $'scanned\t1' "$WORK_DIR/rotate-q2.out"
grep -q $'rotated\t1' "$WORK_DIR/rotate-q2.out"
grep -q $'legacy_plaintext\t0' "$WORK_DIR/rotate-q2.out"
grep -q $'reencrypted\t1' "$WORK_DIR/rotate-q2.out"
grep -q $'active_key_id\talert-smoke-q2' "$WORK_DIR/rotate-q2.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_settings WHERE tenant_id = 1 AND channel = 'webhook' AND webhook_url = '' AND webhook_secret = '' AND webhook_credentials_key_id = 'alert-smoke-q2' AND CHAR_LENGTH(webhook_credentials_ciphertext) > 80")" = "1"
test "$(mysql_scalar "SELECT webhook_credentials_ciphertext <> '$CIPHERTEXT_Q1' FROM mochat_go_saas_alert_settings WHERE tenant_id = 1 AND channel = 'webhook'")" = "1"

MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY="$ALERT_CREDENTIAL_KEY_Q2" \
MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID=alert-smoke-q2 \
MOCHAT_GO_SAAS_ALERT_CREDENTIAL_REQUIRE_ENCRYPTION=1 \
MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  "$MAINTENANCE_BIN" \
  -action dispatch-alert-notifications \
  -alert-limit 5 \
  -alert-notification-retry-delay 1s >"$WORK_DIR/dispatch-encrypted-q2.out"

grep -q $'scanned\t1' "$WORK_DIR/dispatch-encrypted-q2.out"
grep -q $'delivered\t1' "$WORK_DIR/dispatch-encrypted-q2.out"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "5"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-missing-q1'" "delivered/1"

if rg -q 'tenant-secret|/tenant-alert' "$WORK_DIR/rotate-q1.out" "$WORK_DIR/rotate-q2.out" "$WORK_DIR/dispatch-missing-q1.out"; then
  echo "alert credential maintenance output exposed credential material" >&2
  exit 1
fi

echo "SaaS alert setting dispatch smoke passed"
