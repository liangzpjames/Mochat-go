#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-work-message-archive-sync-cron-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18116}"
BRIDGE_ADDR="${MOCHAT_WORK_MESSAGE_ARCHIVE_BRIDGE_ADDR:-127.0.0.1:18117}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13350}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26470}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-work-message-archive-sync-cron.XXXXXX")"
PROBE_DIR="$PWD/.tmp-work-message-archive-credential-probe-$$"
SAAS_MFA_KEY_FILE="$WORK_DIR/saas-admin-mfa.key"
DASHBOARD_MFA_KEY_FILE="$WORK_DIR/dashboard-mfa.key"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
BRIDGE_LOG="$WORK_DIR/bridge.log"
GO_PID=""
BRIDGE_PID=""
WECOM_CREDENTIAL_KEY="7171717171717171717171717171717171717171717171717171717171717171"
WECOM_CREDENTIAL_KEY_ID="archive-smoke-q1"

printf '%s' '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' >"$SAAS_MFA_KEY_FILE"
printf '%s' 'abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789' >"$DASHBOARD_MFA_KEY_FILE"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" \
    MOCHAT_REDIS_PORT="$REDIS_PORT" \
    MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE="$SAAS_MFA_KEY_FILE" \
    MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE="$DASHBOARD_MFA_KEY_FILE" \
    MOCHAT_SAAS_ADMIN_JWT_SECRET="archive-smoke-saas-jwt" \
    MOCHAT_DASHBOARD_JWT_SECRET="archive-smoke-dashboard-jwt" \
    MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_ID="archive-smoke-saas" \
    MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID="archive-smoke-dashboard" \
    docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ -n "$BRIDGE_PID" ] && kill -0 "$BRIDGE_PID" 2>/dev/null; then
    kill "$BRIDGE_PID" 2>/dev/null || true
    wait "$BRIDGE_PID" 2>/dev/null || true
  fi
  rm -rf "$PROBE_DIR" "$WORK_DIR"
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
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  [ -f "$BRIDGE_LOG" ] && tail -120 "$BRIDGE_LOG" >&2 || true
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
  [ -f "$GO_LOG" ] && tail -160 "$GO_LOG" >&2 || true
  [ -f "$BRIDGE_LOG" ] && tail -160 "$BRIDGE_LOG" >&2 || true
  exit 1
}

assert_port_free "$GO_ADDR"
assert_port_free "$BRIDGE_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"

cat >"$WORK_DIR/fake_archive_bridge.py" <<'PY'
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

host, port = sys.argv[1].rsplit(":", 1)

MESSAGES = [
    {
        "seq": 11,
        "msgid": "archive-smoke-11",
        "action": "send",
        "from": "archive-employee",
        "tolist": ["external-archive"],
        "msgtype": "text",
        "msgtime": 1783342800000,
        "text": {"content": "客户需要报价"},
    },
    {
        "seq": 12,
        "msgid": "archive-smoke-12",
        "action": "send",
        "from": "external-archive",
        "tolist": ["archive-employee"],
        "msgtype": "text",
        "msgtime": 1783342860000,
        "text": {"content": "我马上下单"},
    },
]

class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def do_GET(self):
        if self.path == "/healthz":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok")
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        if self.path != "/work-message/archive/messages":
            self.send_response(404)
            self.end_headers()
            return
        if self.headers.get("Authorization") != "Bearer bridge-token":
            self.send_response(401)
            self.end_headers()
            return
        raw = self.rfile.read(int(self.headers.get("Content-Length", "0") or "0"))
        body = json.loads(raw.decode("utf-8"))
        assert body["wx_corpid"] == "ww-archive-sync", body
        assert int(body["corp_id"]) == 901, body
        for forbidden in ("chat_secret", "rsa_public_key", "rsa_private_key"):
            assert forbidden not in body, body
        seq = int(body.get("seq") or 0)
        limit = int(body.get("limit") or 100)
        messages = [item for item in MESSAGES if int(item["seq"]) > seq][:limit]
        payload = {"errcode": 0, "errmsg": "ok", "messages": messages}
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

ThreadingHTTPServer((host, int(port)), Handler).serve_forever()
PY

python3 "$WORK_DIR/fake_archive_bridge.py" "$BRIDGE_ADDR" >"$BRIDGE_LOG" 2>&1 &
BRIDGE_PID="$!"
wait_url "http://$BRIDGE_ADDR/healthz" 200

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
DELETE FROM mc_sensitive_words_monitor WHERE corp_id = 901;
DELETE FROM mc_sensitive_word WHERE id = 910301;
DELETE FROM mc_sensitive_word_group WHERE id = 910301;
DELETE FROM mc_work_message_id WHERE corp_id = 901 AND type IN (40, 21, 22);
DELETE FROM mc_work_message_1 WHERE corp_id = 901 OR msgid LIKE 'archive-smoke-%';
DELETE FROM mc_work_message_2 WHERE corp_id = 901 OR msgid LIKE 'archive-smoke-%';
DELETE FROM mc_work_contact_employee WHERE id = 910102;
DELETE FROM mc_work_contact WHERE id = 910101;
DELETE FROM mc_work_employee WHERE id = 910001;

INSERT INTO mc_corp (
  id, name, wx_corpid, employee_secret, contact_secret,
  token, encoding_aes_key, tenant_id, chat_status, chat_secret,
  created_at, updated_at, deleted_at
) VALUES (
  901, 'Go会话存档同步企业', 'ww-archive-sync', 'employee-secret', 'contact-secret',
  'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 901, 1, 'archive-secret',
  NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  wx_corpid = VALUES(wx_corpid),
  chat_status = VALUES(chat_status),
  chat_secret = VALUES(chat_secret),
  tenant_id = VALUES(tenant_id),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_work_employee (
  id, wx_user_id, corp_id, name, mobile, position, gender, email,
  avatar, thumb_avatar, telephone, alias, extattr, status, qr_code,
  external_profile, external_position, address, open_user_id,
  wx_main_department_id, main_department_id, log_user_id, contact_auth,
  audit_status, created_at, updated_at, deleted_at
) VALUES (
  910001, 'archive-employee', 901, '存档员工', '13900010001', '', 1, '',
  '', '', '', '', JSON_OBJECT(), 1, '',
  JSON_OBJECT(), '', '', 'open-archive-employee',
  1, 0, 0, 1, 1, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  corp_id = VALUES(corp_id),
  wx_user_id = VALUES(wx_user_id),
  name = VALUES(name),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_work_contact (
  id, corp_id, wx_external_userid, name, nick_name, avatar,
  follow_up_status, type, gender, unionid, position, corp_name,
  corp_full_name, external_profile, business_no, created_at, updated_at, deleted_at
) VALUES (
  910101, 901, 'external-archive', '存档客户', '客户', '',
  1, 1, 0, 'union-archive', '', '', '', JSON_OBJECT(), 'ARCHIVE-001', NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  corp_id = VALUES(corp_id),
  wx_external_userid = VALUES(wx_external_userid),
  name = VALUES(name),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_work_contact_employee (
  id, employee_id, contact_id, remark, description, remark_corp_name,
  remark_mobiles, add_way, oper_userid, state, corp_id, status,
  create_time, created_at, updated_at, deleted_at
) VALUES (
  910102, 910001, 910101, '', '', '', JSON_ARRAY(), 0, '',
  '', 901, 1, NOW(), NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  employee_id = VALUES(employee_id),
  contact_id = VALUES(contact_id),
  corp_id = VALUES(corp_id),
  status = VALUES(status),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_sensitive_word_group (id, corp_id, name, created_at, updated_at, deleted_at)
VALUES (910301, 901, '会话存档敏感词分组', NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE corp_id = VALUES(corp_id), name = VALUES(name), updated_at = NOW(), deleted_at = NULL;

INSERT INTO mc_sensitive_word (id, corp_id, group_id, name, status, created_at, updated_at, deleted_at)
VALUES (910301, 901, 910301, '报价', 1, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE corp_id = VALUES(corp_id), group_id = VALUES(group_id), name = VALUES(name), status = VALUES(status), updated_at = NOW(), deleted_at = NULL;
SQL

sed -n '1,14p' deploy/standalone/migrations/0143_group_conversation_workspace.up.sql \
  | compose exec -T mysql mariadb -umochat -pmochat_pass mochat
compose exec -T mysql mariadb -umochat -pmochat_pass mochat <deploy/standalone/migrations/0146_risk_warning_scan_states.up.sql

mkdir -p "$PROBE_DIR"
cat >"$PROBE_DIR/main.go" <<'GO'
package main

import (
	"fmt"
	"os"

	"jiyi/mochat-go/internal/wecomcredentials"
)

func main() {
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey: os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY"),
		EncryptionKeyID: os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID"),
		RequireEncryption: true,
	})
	if err != nil {
		panic(err)
	}
	ciphertext, _, err := manager.EncryptCorp(901, "ww-archive-sync", wecomcredentials.CorpCredential{
		ChatSecret: "archive-secret", ArchiveRSAPublicKey: "archive-public", ArchiveRSAPrivateKey: "archive-private",
	})
	if err != nil {
		panic(err)
	}
	fmt.Print(ciphertext)
}
GO
CREDENTIAL_CIPHERTEXT="$(MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY="$WECOM_CREDENTIAL_KEY" MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID="$WECOM_CREDENTIAL_KEY_ID" env -u GOROOT go run "$PROBE_DIR/main.go")"
printf "UPDATE mc_corp SET employee_secret='', contact_secret='', token='', encoding_aes_key='', chat_secret='', wecom_credentials_ciphertext='%s', wecom_credentials_key_id='%s' WHERE id=901;\n" "$CREDENTIAL_CIPHERTEXT" "$WECOM_CREDENTIAL_KEY_ID" \
  | compose exec -T mysql mariadb -umochat -pmochat_pass mochat

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_GO_ENABLE_ALL_MIGRATED_ROUTES=0 \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_SIMPLE_JWT_SECRET="archive-smoke-legacy-jwt" \
  MOCHAT_SAAS_ADMIN_JWT_SECRET="archive-smoke-saas-jwt" \
  MOCHAT_SAAS_ADMIN_JWT_ISSUER="mochat-go/archive-smoke-saas" \
  MOCHAT_SAAS_ADMIN_JWT_AUDIENCE="mochat-archive-smoke-saas" \
  MOCHAT_DASHBOARD_JWT_SECRET="archive-smoke-dashboard-jwt" \
  MOCHAT_DASHBOARD_JWT_ISSUER="mochat-go/archive-smoke-dashboard" \
  MOCHAT_DASHBOARD_JWT_AUDIENCE="mochat-archive-smoke-dashboard" \
  MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE="$SAAS_MFA_KEY_FILE" \
  MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE="$DASHBOARD_MFA_KEY_FILE" \
  MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_ID="archive-smoke-saas" \
  MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID="archive-smoke-dashboard" \
  MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY="$WECOM_CREDENTIAL_KEY" \
  MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID="$WECOM_CREDENTIAL_KEY_ID" \
  MOCHAT_GO_WECOM_CREDENTIAL_REQUIRE_ENCRYPTION=1 \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON=1 \
  MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_RUN_ON_START=1 \
  MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_INTERVAL_SECONDS=2 \
  MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_LIMIT=10 \
  MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL="http://$BRIDGE_ADDR" \
  MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_TOKEN="bridge-token" \
  MOCHAT_GO_ENABLE_SENSITIVE_WORD_MONITOR_CRON=1 \
  MOCHAT_GO_SENSITIVE_WORD_MONITOR_CRON_RUN_ON_START=1 \
  MOCHAT_GO_SENSITIVE_WORD_MONITOR_CRON_INTERVAL_SECONDS=2 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
curl -sS -f "http://$GO_ADDR/compat/status" >"$WORK_DIR/status.json"
python3 - "$WORK_DIR/status.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as fh:
    status = json.load(fh)
tasks = {task.get("name"): task for task in status.get("background_tasks", [])}
for name in ("cron-work-message-archive-sync", "cron-sensitive-word-monitor"):
    task = tasks.get(name)
    assert task, status
    assert task.get("status") == "running", task
    assert task.get("started_at"), task
    assert task.get("run_id"), task
PY

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_message_1 WHERE corp_id = 901 AND msgid = 'archive-smoke-11' AND seq = 11 AND work_employee_id = 910001 AND to_user_type = 1 AND to_user_id = 910101 AND sender_type = 0 AND content_text = '客户需要报价' AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_message_2 WHERE corp_id = 901 AND msgid = 'archive-smoke-12' AND seq = 12 AND work_employee_id = 910001 AND to_user_type = 1 AND to_user_id = 910101 AND sender_type = 1 AND content_text = '我马上下单' AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT last_id FROM mc_work_message_id WHERE corp_id = 901 AND type = 40 AND deleted_at IS NULL ORDER BY id DESC LIMIT 1" "12"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_sensitive_words_monitor WHERE corp_id = 901 AND sensitive_word_id = 910301 AND trigger_user_id = 910001 AND source = 2 AND deleted_at IS NULL" "1"
sleep 4
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_message_1 WHERE corp_id = 901 AND msgid = 'archive-smoke-11' AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_message_2 WHERE corp_id = 901 AND msgid = 'archive-smoke-12' AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_sensitive_words_monitor WHERE corp_id = 901 AND sensitive_word_id = 910301 AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT CONCAT(status, ',', start_count, ',', failure_count) FROM mochat_go_background_tasks WHERE name = 'cron-work-message-archive-sync'" "running,1,0"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 1, 1, 0) FROM mochat_go_background_task_executions WHERE task_name = 'cron-work-message-archive-sync' AND kind = 'periodic_tick' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"

grep -q "go cron enabled: workMessageArchive" "$GO_LOG"
grep -q "workMessageArchive sync cron finished: corps=1 fetched=2 inserted=2" "$GO_LOG"
grep -q "sensitiveWordsMonitor cron finished: words=1 corps=1 tables=10" "$GO_LOG"

echo "workMessageArchive sync cron smoke passed"
