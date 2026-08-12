#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-quota-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13335}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26391}"
GO_ADDR="${MOCHAT_SAAS_QUOTA_GO_ADDR:-127.0.0.1:18103}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19066}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-quota.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
GO_PID=""
WECOM_PID=""

SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-quota-secret}"
TENANT_ID="${MOCHAT_SAAS_QUOTA_TENANT_ID:-301}"
PHONE="${MOCHAT_SAAS_QUOTA_PHONE:-13800000301}"
PASSWORD="${MOCHAT_SAAS_QUOTA_PASSWORD:-secret301}"
CORP_ID="${MOCHAT_SAAS_QUOTA_CORP_ID:-301}"
EMPLOYEE_ID="${MOCHAT_SAAS_QUOTA_EMPLOYEE_ID:-301}"
AGENT_ID="${MOCHAT_SAAS_QUOTA_AGENT_ID:-301}"
CONTACT_ID="${MOCHAT_SAAS_QUOTA_CONTACT_ID:-301}"
ROOM_ID="${MOCHAT_SAAS_QUOTA_ROOM_ID:-301}"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ -n "$WECOM_PID" ] && kill -0 "$WECOM_PID" 2>/dev/null; then
    kill "$WECOM_PID" 2>/dev/null || true
    wait "$WECOM_PID" 2>/dev/null || true
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
  [ -f "$WECOM_LOG" ] && tail -80 "$WECOM_LOG" >&2 || true
  exit 1
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  local query="$1"
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat_saas_quota_check -e "$query" | tr -d '\r'
}

redis_cli() {
  compose exec -T redis redis-cli "$@"
}

assert_quota_response() {
  local file="$1"
  local expected_msg="$2"
  python3 - "$file" "$expected_msg" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
expected_msg = sys.argv[2]
assert payload["code"] == 400, payload
assert payload["msg"] == expected_msg, payload
assert payload["data"]["current"] == payload["data"]["limit"], payload
assert payload["data"]["additional"] == 1, payload
PY
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"
assert_port_free "${WECOM_ADDR##*:}"

cat >"$WORK_DIR/fake_wecom.py" <<'PY'
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def json_response(self, data):
        body = json.dumps(data, ensure_ascii=False).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        path = urlparse(self.path)
        query = parse_qs(path.query)
        if path.path == "/healthz":
            self.json_response({"ok": True})
            return
        if path.path == "/cgi-bin/gettoken":
            self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "fake-token", "expires_in": 7200})
            return
        if path.path == "/cgi-bin/externalcontact/list":
            self.json_response({"errcode": 0, "errmsg": "ok", "external_userid": ["external-quota-301", "external-quota-302"]})
            return
        if path.path == "/cgi-bin/externalcontact/get":
            external_userid = query.get("external_userid", [""])[0]
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "external_contact": {
                    "external_userid": external_userid,
                    "name": "额度客户" + external_userid[-3:],
                    "avatar": "",
                    "type": 1,
                    "gender": 1,
                    "unionid": "union-" + external_userid,
                    "position": "",
                    "corp_name": "",
                    "corp_full_name": "",
                    "external_profile": {},
                    "business_no": "BIZ-" + external_userid,
                },
                "follow_user": [{
                    "userid": "quota-user-301",
                    "remark": "",
                    "description": "",
                    "remark_corp_name": "",
                    "remark_mobiles": [],
                    "add_way": 1,
                    "oper_userid": "quota-user-301",
                    "state": "",
                    "createtime": 1783300000,
                    "tags": [],
                }],
            })
            return
        self.json_response({"errcode": 404, "errmsg": "unknown path " + path.path})

    def do_POST(self):
        path = urlparse(self.path)
        length = int(self.headers.get("Content-Length", "0") or "0")
        raw = self.rfile.read(length) if length else b"{}"
        try:
            payload = json.loads(raw.decode("utf-8")) if raw else {}
        except json.JSONDecodeError:
            payload = {}
        if path.path == "/cgi-bin/externalcontact/groupchat/list":
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "group_chat_list": [
                    {"chat_id": "room-quota-301", "status": 0},
                    {"chat_id": "room-quota-302", "status": 0},
                ],
                "next_cursor": "",
            })
            return
        if path.path == "/cgi-bin/externalcontact/groupchat/get":
            chat_id = payload.get("chat_id", "")
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "group_chat": {
                    "chat_id": chat_id,
                    "name": "额度客户群" + chat_id[-3:],
                    "owner": "quota-user-301",
                    "notice": "",
                    "create_time": 1783300000,
                    "member_list": [],
                },
            })
            return
        if path.path == "/cgi-bin/externalcontact/add_contact_way":
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "config_id": "channel-config-extra",
                "qr_code": "https://wecom.example/channel-extra.png",
            })
            return
        if path.path == "/cgi-bin/component/api_component_token":
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "component_access_token": "component-token",
                "expires_in": 7200,
            })
            return
        if path.path == "/cgi-bin/component/api_query_auth":
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "authorization_info": {
                    "authorizer_appid": "official-quota-302",
                    "authorizer_refresh_token": "authorizer-refresh-token-quota",
                    "func_info": [],
                },
            })
            return
        self.json_response({"errcode": 404, "errmsg": "unknown path " + path.path})


host, port = sys.argv[1], int(sys.argv[2])
ThreadingHTTPServer((host, port), Handler).serve_forever()
PY

python3 "$WORK_DIR/fake_wecom.py" "${WECOM_ADDR%:*}" "${WECOM_ADDR##*:}" >"$WECOM_LOG" 2>&1 &
WECOM_PID="$!"
wait_url "http://$WECOM_ADDR/healthz" 200

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

mysql_root <<'SQL'
DROP DATABASE IF EXISTS mochat_saas_quota_check;
CREATE DATABASE mochat_saas_quota_check CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON mochat_saas_quota_check.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat_saas_quota_check?parseTime=true&loc=Local"

"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0003_saas_provisioning\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0004_saas_storage_objects\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0005_background_tasks\tapplied_now' "$WORK_DIR/migrate.out"

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id "$TENANT_ID" \
  -tenant-name "SaaS额度验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "SaaS额度管理员" \
  -role-name "SaaS额度超级管理员" \
  -package-code "quota-tight" \
  -package-name "额度验收版" \
  -max-corps 1 \
  -max-users 1 \
  -max-contacts 1 \
  -max-rooms 1 \
  -max-agents 1 \
  -channel-codes 1 \
  -shop-codes 1 \
  -radars 1 \
  -lotteries 1 \
  -room-infinite-pulls 1 \
  -room-fissions 1 \
  -room-clock-ins 1 \
  -room-qualities 1 \
  -room-calendars 1 \
  -room-reminds 1 \
  -contact-sops 1 \
  -room-sops 1 \
  -sensitive-words 1 \
  -storage-mb 1 \
  -contact-message-batches 1 \
  -room-message-batches 1 \
  -room-tag-pulls 1 \
  -work-room-auto-pulls 1 \
  -work-fissions 1 \
  -official-accounts 1 \
  -async-executions 1 >"$WORK_DIR/bootstrap.out"
grep -q $'tenant_id\t'"$TENANT_ID" "$WORK_DIR/bootstrap.out"
grep -q $'usage_metric_count\t26' "$WORK_DIR/bootstrap.out"

USER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '$PHONE' AND tenant_id = $TENANT_ID AND status = 1 AND isSuperAdmin = 1 AND deleted_at IS NULL")"
ROLE_ID="$(mysql_scalar "SELECT id FROM mc_rbac_role WHERE tenant_id = $TENANT_ID AND name = 'SaaS额度超级管理员' AND status = 1 AND deleted_at IS NULL")"
test -n "$USER_ID"
test -n "$ROLE_ID"

mysql_root mochat_saas_quota_check <<SQL
INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '额度已满企业', 'wwquota301', 'employee-secret-quota', 'contact-secret-quota', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', $TENANT_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, gender, status, log_user_id, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'quota-user-301', $CORP_ID, 'SaaS额度管理员', '$PHONE', 1, 1, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_agent
  (id, corp_id, wx_agent_id, wx_secret, name, created_at, updated_at, deleted_at)
VALUES
  ($AGENT_ID, $CORP_ID, '1000301', 'agent-secret-quota', '额度已满应用', NOW(), NOW(), NULL);

INSERT INTO mc_channel_code
  (corp_id, group_id, name, qrcode_url, wx_config_id, auto_add_friend, tags, type, drainage_employee, welcome_message, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, 0, '额度已满渠道活码', 'https://wecom.example/channel-filled.png', 'channel-config-filled', 1, JSON_ARRAY(), 1, JSON_OBJECT(), JSON_OBJECT(), NOW(), NOW(), NULL);

INSERT INTO mc_shop_code
  (name, type, employee, employee_qrcode, qw_code, search_keyword, address, country, province, city, district, lat, lng, status, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('额度已满门店活码', 1, JSON_ARRAY(JSON_OBJECT('id', $EMPLOYEE_ID, 'name', 'SaaS额度管理员')), JSON_ARRAY(), JSON_ARRAY(), '额度门店', '额度街道1号', '中国', '浙江省', '杭州市', '西湖区', '30.1', '120.1', 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_radar
  (type, title, link, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  (1, '额度已满互动雷达', 'https://example.com/quota-radar', $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_lottery
  (name, description, type, time_type, contact_tags, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('额度已满抽奖活动', '额度已满抽奖活动说明', 'roulette', 1, JSON_ARRAY(), $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_infinite
  (name, avatar, title_status, title, describe_status, \`describe\`, logo, qw_code, total_num, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('额度已满无限拉群', '', 1, '', 1, '扫码入群', '', JSON_ARRAY(), 0, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_fission
  (official_account_id, active_name, end_time, target_count, new_friend, delete_invalid, receive_employees, auto_pass, status, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  (0, '额度已满群裂变', '2037-01-01 00:00:00', 1, 0, 0, JSON_ARRAY(), 1, 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_clock_in
  (official_account_id, active_name, description, type, start_time, end_time, tasks, employee_qrcode, contact_tags, corp_card_status, corp_card, status, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  (0, '额度已满群打卡', '额度已满群打卡说明', 1, NULL, '2037-01-01 00:00:00', JSON_ARRAY(), '', JSON_ARRAY(), 0, JSON_OBJECT(), 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_quality
  (name, description, \`rule\`, rooms, status, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('额度已满群质检', '额度已满群质检说明', JSON_ARRAY(), JSON_ARRAY(), 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_calendar
  (name, rooms, on_off, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('额度已满群日历', JSON_ARRAY(), 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_remind
  (name, rooms, is_qrcode, is_link, is_miniprogram, is_card, is_keyword, keyword, status, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('额度已满客户群提醒', JSON_ARRAY(), 0, 0, 0, 0, 0, '', 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_contact_sop
  (corp_id, creator_id, name, setting, employee_ids, state, contact_ids, created_at, updated_at)
VALUES
  ($CORP_ID, $USER_ID, '额度已满个人SOP', JSON_ARRAY(JSON_OBJECT('name', '额度提醒', 'content', JSON_ARRAY(JSON_OBJECT('type', 'text', 'value', '个人SOP额度已满')))), JSON_ARRAY($EMPLOYEE_ID), 1, JSON_ARRAY($CONTACT_ID), NOW(), NOW());

INSERT INTO mc_room_sop
  (corp_id, creator_id, name, setting, room_ids, state, created_at, updated_at)
VALUES
  ($CORP_ID, $USER_ID, '额度已满群SOP', JSON_ARRAY(JSON_OBJECT('name', '额度提醒', 'content', JSON_ARRAY(JSON_OBJECT('type', 'text', 'value', '群SOP额度已满')))), JSON_ARRAY($ROOM_ID), 1, NOW(), NOW());

INSERT INTO mc_sensitive_word_group
  (id, corp_id, name, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, $CORP_ID, '额度已满敏感词分组', NOW(), NOW(), NULL);

INSERT INTO mc_sensitive_word
  (corp_id, group_id, name, status, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, $CORP_ID, '额度已满敏感词', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, type, gender, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ID, $CORP_ID, 'external-quota-301', '额度已满客户', 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_room
  (id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_ID, $CORP_ID, 'room-quota-301', '额度已满客户群', $EMPLOYEE_ID, '', 0, NOW(), 200, NOW(), NOW(), NULL);

INSERT INTO mc_contact_message_batch_send
  (corp_id, user_id, user_name, employee_ids, filter_params, filter_params_detail, content, send_way, definite_time, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, $USER_ID, 'SaaS额度管理员', JSON_ARRAY($EMPLOYEE_ID), JSON_OBJECT(), JSON_OBJECT('rooms', JSON_ARRAY(), 'tags', JSON_ARRAY(), 'excludeContacts', JSON_ARRAY()), JSON_ARRAY(JSON_OBJECT('msgType', 'text', 'content', '额度已满客户群发')), 2, '2026-07-04 10:00:00', NOW(), NOW(), NULL);

INSERT INTO mc_room_message_batch_send
  (corp_id, user_id, user_name, employee_ids, batch_title, content, send_way, definite_time, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, $USER_ID, 'SaaS额度管理员', JSON_ARRAY($EMPLOYEE_ID), '额度已满客户群群发', JSON_ARRAY(JSON_OBJECT('msgType', 'text', 'content', '额度已满客户群群发')), 2, '2026-07-04 10:00:00', NOW(), NOW(), NULL);

INSERT INTO mc_room_tag_pull
  (name, employees, choose_contact, guide, rooms, filter_contact, contact_num, wx_tid, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('额度已满标签建群', CAST($EMPLOYEE_ID AS CHAR), JSON_OBJECT('is_all', 1), '扫码入群', JSON_ARRAY(JSON_OBJECT('id', $ROOM_ID, 'name', '额度已满客户群', 'num', 50)), 1, 1, JSON_ARRAY(), $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_room_auto_pull
  (corp_id, qrcode_name, qrcode_url, wx_config_id, is_verified, leading_words, tags, employees, rooms, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '额度已满自动拉群', 'https://wecom.example/auto.png', 'auto-config-quota', 2, '欢迎入群', JSON_ARRAY(), JSON_ARRAY(CAST($EMPLOYEE_ID AS CHAR)), JSON_ARRAY(JSON_OBJECT('roomId', $ROOM_ID, 'maxNum', 50)), NOW(), NOW(), NULL);

INSERT INTO mc_work_fission
  (corp_id, active_name, service_employees, auto_pass, auto_add_tag, contact_tags, end_time, qr_code_invalid, tasks, new_friend, delete_invalid, receive_prize, receive_prize_employees, receive_links, receive_qrcode, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '额度已满裂变', JSON_ARRAY(JSON_OBJECT('id', $EMPLOYEE_ID, 'wxUserId', 'quota-user-301')), 1, 0, JSON_ARRAY(), '2037-01-01 00:00:00', 7, JSON_ARRAY(), 1, 0, 0, JSON_ARRAY(), JSON_ARRAY(), JSON_ARRAY(), $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_official_account
  (appid, authorized_status, authorizer_appid, nickname, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('component-appid', 1, 'official-quota-301', '额度已满公众号', $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mochat_go_saas_storage_objects
  (tenant_id, user_id, employee_id, corp_id, source, original_name, relative_path, content_type, size_bytes, created_at, updated_at, deleted_at)
VALUES
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'seed.quota', 'filled.bin', 'quota/filled.bin', 'application/octet-stream', 1048576, NOW(), NOW(), NULL);
SQL

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_corp WHERE tenant_id = $TENANT_ID AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_user WHERE tenant_id = $TENANT_ID AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_agent a JOIN mc_corp c ON c.id = a.corp_id WHERE c.tenant_id = $TENANT_ID AND a.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_channel_code code JOIN mc_corp c ON c.id = code.corp_id WHERE c.tenant_id = $TENANT_ID AND code.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_shop_code code JOIN mc_corp c ON c.id = code.corp_id WHERE c.tenant_id = $TENANT_ID AND code.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_radar radar JOIN mc_corp c ON c.id = radar.corp_id WHERE c.tenant_id = $TENANT_ID AND radar.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_lottery lottery JOIN mc_corp c ON c.id = lottery.corp_id WHERE c.tenant_id = $TENANT_ID AND lottery.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_infinite activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_clock_in activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_quality activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_calendar activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_remind activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_sop sop JOIN mc_corp c ON c.id = sop.corp_id WHERE c.tenant_id = $TENANT_ID AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_sop sop JOIN mc_corp c ON c.id = sop.corp_id WHERE c.tenant_id = $TENANT_ID AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_sensitive_word word JOIN mc_corp c ON c.id = word.corp_id WHERE c.tenant_id = $TENANT_ID AND word.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_contact contact JOIN mc_corp c ON c.id = contact.corp_id WHERE c.tenant_id = $TENANT_ID AND contact.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_room room JOIN mc_corp c ON c.id = room.corp_id WHERE c.tenant_id = $TENANT_ID AND room.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_message_batch_send batch JOIN mc_corp c ON c.id = batch.corp_id WHERE c.tenant_id = $TENANT_ID AND batch.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_message_batch_send batch JOIN mc_corp c ON c.id = batch.corp_id WHERE c.tenant_id = $TENANT_ID AND batch.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_tag_pull activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_room_auto_pull activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_official_account account JOIN mc_corp c ON c.id = account.corp_id WHERE c.tenant_id = $TENANT_ID AND account.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'corps' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'users' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'contacts' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'rooms' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'agents' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'channel_codes' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'shop_codes' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'radars' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'lotteries' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_infinite_pulls' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_fissions' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_clock_ins' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_qualities' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_calendars' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_reminds' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'contact_sops' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_sops' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'sensitive_words' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'contact_message_batches' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_message_batches' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_tag_pulls' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'work_room_auto_pulls' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'work_fissions' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'official_accounts' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'async_executions' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND deleted_at IS NULL")" = "1"

redis_cli SET "mc:user.$USER_ID" "$CORP_ID-$EMPLOYEE_ID" >/dev/null

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_WECHAT_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_WECHAT_OPEN_PLATFORM_APP_ID="component-appid" \
  MOCHAT_WECHAT_OPEN_PLATFORM_SECRET="component-secret" \
  MOCHAT_WECHAT_OPEN_PLATFORM_TOKEN="component-token-for-callback" \
  MOCHAT_WECHAT_OPEN_PLATFORM_AES_KEY="abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG" \
  MOCHAT_WECHAT_COMPONENT_VERIFY_TICKET="component-ticket" \
  MOCHAT_FILE_STORAGE_ROOT="$WORK_DIR/upload" \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_MIGRATE_USER_STORE=1 \
  MOCHAT_GO_MIGRATE_CORP_STORE=1 \
  MOCHAT_GO_MIGRATE_AGENT_STORE=1 \
  MOCHAT_GO_MIGRATE_COMMON_UPLOAD=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_SYNC=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_SYNC=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_STORE=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_STORE=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_STORE=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_STORE=1 \
  MOCHAT_GO_MIGRATE_SHOP_CODE_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_RADAR_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_LOTTERY_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_ROOM_INFINITE_PULL_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_ROOM_FISSION_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_ROOM_CLOCK_IN_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_ROOM_QUALITY_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_ROOM_CALENDAR_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_ROOM_REMIND_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_CONTACT_SOP_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_ROOM_SOP_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_SENSITIVE_WORDS_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_STORE=1 \
  MOCHAT_GO_MIGRATE_WORK_FISSION_STORE=1 \
  MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_AUTH_REDIRECT=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200

curl -sS -f \
  -H "Content-Type: application/json" \
  -d "{\"phone\":\"$PHONE\",\"password\":\"$PASSWORD\"}" \
  "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/auth.json"

TOKEN="$(python3 - "$WORK_DIR/auth.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
assert payload["data"]["token"], payload
print(payload["data"]["token"])
PY
)"

cat >"$WORK_DIR/corp.json" <<'JSON'
{"corpName":"额度外企业","wxCorpId":"wwquota302","employeeSecret":"employee-secret-quota-extra","contactSecret":"contact-secret-quota-extra"}
JSON
corp_code="$(curl -sS -o "$WORK_DIR/corp-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/corp.json" \
  "http://$GO_ADDR/dashboard/corp/store")"
test "$corp_code" = "400"
assert_quota_response "$WORK_DIR/corp-response.json" "套餐额度已达上限：企业数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_corp WHERE tenant_id = $TENANT_ID AND deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/user.json" <<JSON
{"userName":"额度外子账号","phone":"13800000302","gender":1,"status":1,"roleId":$ROLE_ID,"password":"secret302","department":"额度测试部"}
JSON
user_code="$(curl -sS -o "$WORK_DIR/user-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/user.json" \
  "http://$GO_ADDR/dashboard/user/store")"
test "$user_code" = "400"
assert_quota_response "$WORK_DIR/user-response.json" "套餐额度已达上限：子账号数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_user WHERE tenant_id = $TENANT_ID AND deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/agent.json" <<'JSON'
{"wxAgentId":"1000302","wxSecret":"agent-secret-quota-extra","type":1}
JSON
agent_code="$(curl -sS -o "$WORK_DIR/agent-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/agent.json" \
  "http://$GO_ADDR/dashboard/agent/store")"
test "$agent_code" = "400"
assert_quota_response "$WORK_DIR/agent-response.json" "套餐额度已达上限：应用数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_agent a JOIN mc_corp c ON c.id = a.corp_id WHERE c.tenant_id = $TENANT_ID AND a.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/channel-code.json" <<JSON
{"baseInfo":{"groupId":0,"name":"额度外渠道活码","autoAddFriend":1,"tags":[]},"drainageEmployee":{"type":1,"employees":[],"specialPeriod":{"status":1,"detail":[{"startDate":"2026-01-01","endDate":"2026-12-31","timeSlot":[{"startTime":"00:00","endTime":"00:00","employeeId":[$EMPLOYEE_ID]}]}]},"addMax":{"status":2,"employees":[],"spareEmployeeIds":[]}},"welcomeMessage":{"scanCodePush":2,"messageDetail":[]}}
JSON
channel_code_code="$(curl -sS -o "$WORK_DIR/channel-code-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/channel-code.json" \
  "http://$GO_ADDR/dashboard/channelCode/store")"
test "$channel_code_code" = "400"
assert_quota_response "$WORK_DIR/channel-code-response.json" "套餐额度已达上限：渠道活码数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_channel_code code JOIN mc_corp c ON c.id = code.corp_id WHERE c.tenant_id = $TENANT_ID AND code.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/shop-code.json" <<JSON
{"name":"额度外门店活码","type":1,"employee":[{"id":$EMPLOYEE_ID,"name":"SaaS额度管理员"}],"searchKeyword":"额度门店","address":"额度街道2号","country":"中国","province":"浙江省","city":"杭州市","district":"西湖区","lat":"30.2","lng":"120.2"}
JSON
shop_code_code="$(curl -sS -o "$WORK_DIR/shop-code-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/shop-code.json" \
  "http://$GO_ADDR/dashboard/shopCode/store")"
test "$shop_code_code" = "400"
assert_quota_response "$WORK_DIR/shop-code-response.json" "套餐额度已达上限：门店活码数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_shop_code code JOIN mc_corp c ON c.id = code.corp_id WHERE c.tenant_id = $TENANT_ID AND code.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/radar.json" <<'JSON'
{"type":1,"title":"额度外互动雷达","link":"https://example.com/quota-extra-radar"}
JSON
radar_code="$(curl -sS -o "$WORK_DIR/radar-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/radar.json" \
  "http://$GO_ADDR/dashboard/radar/store")"
test "$radar_code" = "400"
assert_quota_response "$WORK_DIR/radar-response.json" "套餐额度已达上限：互动雷达数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_radar radar JOIN mc_corp c ON c.id = radar.corp_id WHERE c.tenant_id = $TENANT_ID AND radar.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/lottery.json" <<'JSON'
{"name":"额度外抽奖活动"}
JSON
lottery_code="$(curl -sS -o "$WORK_DIR/lottery-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/lottery.json" \
  "http://$GO_ADDR/dashboard/lottery/store")"
test "$lottery_code" = "400"
assert_quota_response "$WORK_DIR/lottery-response.json" "套餐额度已达上限：抽奖活动数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_lottery lottery JOIN mc_corp c ON c.id = lottery.corp_id WHERE c.tenant_id = $TENANT_ID AND lottery.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/room-infinite-pull.json" <<'JSON'
{"name":"额度外无限拉群"}
JSON
room_infinite_pull_code="$(curl -sS -o "$WORK_DIR/room-infinite-pull-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/room-infinite-pull.json" \
  "http://$GO_ADDR/dashboard/roomInfinitePull/store")"
test "$room_infinite_pull_code" = "400"
assert_quota_response "$WORK_DIR/room-infinite-pull-response.json" "套餐额度已达上限：无限拉群数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_infinite activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/room-fission.json" <<'JSON'
{"fission":{"active_name":"额度外群裂变"}}
JSON
room_fission_code="$(curl -sS -o "$WORK_DIR/room-fission-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/room-fission.json" \
  "http://$GO_ADDR/dashboard/roomFission/store")"
test "$room_fission_code" = "400"
assert_quota_response "$WORK_DIR/room-fission-response.json" "套餐额度已达上限：群裂变数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/room-clock-in.json" <<'JSON'
{"clockIn":{"active_name":"额度外群打卡"}}
JSON
room_clock_in_code="$(curl -sS -o "$WORK_DIR/room-clock-in-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/room-clock-in.json" \
  "http://$GO_ADDR/dashboard/roomClockIn/store")"
test "$room_clock_in_code" = "400"
assert_quota_response "$WORK_DIR/room-clock-in-response.json" "套餐额度已达上限：群打卡数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_clock_in activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/room-quality.json" <<'JSON'
{"quality":{"name":"额度外群质检"}}
JSON
room_quality_code="$(curl -sS -o "$WORK_DIR/room-quality-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/room-quality.json" \
  "http://$GO_ADDR/dashboard/roomQuality/store")"
test "$room_quality_code" = "400"
assert_quota_response "$WORK_DIR/room-quality-response.json" "套餐额度已达上限：群质检规则数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_quality activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/room-calendar.json" <<'JSON'
{"name":"额度外群日历"}
JSON
room_calendar_code="$(curl -sS -o "$WORK_DIR/room-calendar-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/room-calendar.json" \
  "http://$GO_ADDR/dashboard/roomCalendar/store")"
test "$room_calendar_code" = "400"
assert_quota_response "$WORK_DIR/room-calendar-response.json" "套餐额度已达上限：群日历数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_calendar activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/room-remind.json" <<'JSON'
{"name":"额度外客户群提醒"}
JSON
room_remind_code="$(curl -sS -o "$WORK_DIR/room-remind-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/room-remind.json" \
  "http://$GO_ADDR/dashboard/roomRemind/store")"
test "$room_remind_code" = "400"
assert_quota_response "$WORK_DIR/room-remind-response.json" "套餐额度已达上限：客户群提醒数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_remind activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/contact-sop.json" <<JSON
{"name":"额度外个人SOP","setting":[{"name":"额度外规则","content":[{"type":"text","value":"额度外个人SOP"}]}],"employeeIds":[$EMPLOYEE_ID],"contactIds":[$CONTACT_ID]}
JSON
contact_sop_code="$(curl -sS -o "$WORK_DIR/contact-sop-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/contact-sop.json" \
  "http://$GO_ADDR/dashboard/contactSop/store")"
test "$contact_sop_code" = "400"
assert_quota_response "$WORK_DIR/contact-sop-response.json" "套餐额度已达上限：个人SOP规则数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_sop sop JOIN mc_corp c ON c.id = sop.corp_id WHERE c.tenant_id = $TENANT_ID AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/room-sop.json" <<JSON
{"name":"额度外群SOP","setting":[{"name":"额度外规则","content":[{"type":"text","value":"额度外群SOP"}]}],"roomIds":[$ROOM_ID]}
JSON
room_sop_code="$(curl -sS -o "$WORK_DIR/room-sop-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/room-sop.json" \
  "http://$GO_ADDR/dashboard/roomSop/store")"
test "$room_sop_code" = "400"
assert_quota_response "$WORK_DIR/room-sop-response.json" "套餐额度已达上限：群SOP规则数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_sop sop JOIN mc_corp c ON c.id = sop.corp_id WHERE c.tenant_id = $TENANT_ID AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/sensitive-word.json" <<JSON
{"groupId":$CORP_ID,"name":"额度外敏感词"}
JSON
sensitive_word_code="$(curl -sS -o "$WORK_DIR/sensitive-word-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/sensitive-word.json" \
  "http://$GO_ADDR/dashboard/sensitiveWord/store")"
test "$sensitive_word_code" = "400"
assert_quota_response "$WORK_DIR/sensitive-word-response.json" "套餐额度已达上限：敏感词词库数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_sensitive_word word JOIN mc_corp c ON c.id = word.corp_id WHERE c.tenant_id = $TENANT_ID AND word.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

contact_code="$(curl -sS -o "$WORK_DIR/contact-response.json" -w '%{http_code}' \
  -X PUT \
  -H "Authorization: Bearer $TOKEN" \
  "http://$GO_ADDR/dashboard/workContact/synContact")"
test "$contact_code" = "400"
assert_quota_response "$WORK_DIR/contact-response.json" "套餐额度已达上限：客户数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_contact contact JOIN mc_corp c ON c.id = contact.corp_id WHERE c.tenant_id = $TENANT_ID AND contact.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

room_code="$(curl -sS -o "$WORK_DIR/room-response.json" -w '%{http_code}' \
  -X PUT \
  -H "Authorization: Bearer $TOKEN" \
  "http://$GO_ADDR/dashboard/workRoom/syn")"
test "$room_code" = "400"
assert_quota_response "$WORK_DIR/room-response.json" "套餐额度已达上限：客户群数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_room room JOIN mc_corp c ON c.id = room.corp_id WHERE c.tenant_id = $TENANT_ID AND room.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/contact-batch.json" <<JSON
{"employeeIds":[$EMPLOYEE_ID],"filterParams":{},"content":[{"msgType":"text","content":"额度外客户群发"}],"sendWay":2,"definiteTime":"2026-07-04 11:00:00"}
JSON
contact_batch_code="$(curl -sS -o "$WORK_DIR/contact-batch-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/contact-batch.json" \
  "http://$GO_ADDR/dashboard/contactMessageBatchSend/store")"
test "$contact_batch_code" = "400"
assert_quota_response "$WORK_DIR/contact-batch-response.json" "套餐额度已达上限：客户群发任务数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_message_batch_send batch JOIN mc_corp c ON c.id = batch.corp_id WHERE c.tenant_id = $TENANT_ID AND batch.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/room-batch.json" <<JSON
{"batchTitle":"额度外客户群群发","employeeIds":[$EMPLOYEE_ID],"content":[{"msgType":"text","content":"额度外客户群群发"}],"sendWay":2,"definiteTime":"2026-07-04 11:00:00"}
JSON
room_batch_code="$(curl -sS -o "$WORK_DIR/room-batch-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/room-batch.json" \
  "http://$GO_ADDR/dashboard/roomMessageBatchSend/store")"
test "$room_batch_code" = "400"
assert_quota_response "$WORK_DIR/room-batch-response.json" "套餐额度已达上限：客户群群发任务数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_message_batch_send batch JOIN mc_corp c ON c.id = batch.corp_id WHERE c.tenant_id = $TENANT_ID AND batch.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/room-tag-pull.json" <<JSON
{"name":"额度外标签建群","employees":[$EMPLOYEE_ID],"choose_contact":{"is_all":1},"guide":"扫码入群","rooms":[{"id":$ROOM_ID,"name":"额度已满客户群","num":50,"image":"image/local.jpg"}],"filter_contact":1}
JSON
room_tag_pull_code="$(curl -sS -o "$WORK_DIR/room-tag-pull-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/room-tag-pull.json" \
  "http://$GO_ADDR/dashboard/roomTagPull/store")"
test "$room_tag_pull_code" = "400"
assert_quota_response "$WORK_DIR/room-tag-pull-response.json" "套餐额度已达上限：标签建群任务数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_tag_pull activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/work-room-auto-pull.json" <<JSON
{"corpId":$CORP_ID,"qrcodeName":"额度外自动拉群","isVerified":2,"leadingWords":"欢迎入群","employees":"$EMPLOYEE_ID","tags":"900001","rooms":"[{\"roomId\":$ROOM_ID,\"maxNum\":50}]"}
JSON
work_room_auto_pull_code="$(curl -sS -o "$WORK_DIR/work-room-auto-pull-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/work-room-auto-pull.json" \
  "http://$GO_ADDR/dashboard/workRoomAutoPull/store")"
test "$work_room_auto_pull_code" = "400"
assert_quota_response "$WORK_DIR/work-room-auto-pull-response.json" "套餐额度已达上限：自动拉群活码数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_room_auto_pull activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

cat >"$WORK_DIR/work-fission.json" <<'JSON'
{"fission":{"active_name":"额度外裂变"},"welcome":{},"poster":{},"push":{},"invite":{}}
JSON
work_fission_code="$(curl -sS -o "$WORK_DIR/work-fission-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @"$WORK_DIR/work-fission.json" \
  "http://$GO_ADDR/dashboard/workFission/store")"
test "$work_fission_code" = "400"
assert_quota_response "$WORK_DIR/work-fission-response.json" "套餐额度已达上限：裂变活动数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission activity JOIN mc_corp c ON c.id = activity.corp_id WHERE c.tenant_id = $TENANT_ID AND activity.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

official_account_code="$(curl -sS -o "$WORK_DIR/official-account-response.json" -w '%{http_code}' \
  "http://$GO_ADDR/dashboard/officialAccount/authRedirect/?auth_code=quota-official-code&corp_id=$CORP_ID")"
test "$official_account_code" = "400"
assert_quota_response "$WORK_DIR/official-account-response.json" "套餐额度已达上限：公众号授权数 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_official_account account JOIN mc_corp c ON c.id = account.corp_id WHERE c.tenant_id = $TENANT_ID AND account.deleted_at IS NULL AND c.deleted_at IS NULL")" = "1"

printf '%s' 'quota-upload' >"$WORK_DIR/quota.png"
storage_code="$(curl -sS -o "$WORK_DIR/storage-response.json" -w '%{http_code}' \
  -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@$WORK_DIR/quota.png;type=image/png;filename=quota.png" \
  "http://$GO_ADDR/dashboard/common/upload")"
test "$storage_code" = "400"
assert_quota_response "$WORK_DIR/storage-response.json" "套餐额度已达上限：素材存储 1/1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND deleted_at IS NULL")" = "1"
if [ -d "$WORK_DIR/upload" ] && find "$WORK_DIR/upload" -type f | grep -q .; then
  echo "storage quota rejected the request after writing a file" >&2
  find "$WORK_DIR/upload" -type f >&2
  exit 1
fi

echo "saas quota enforcement smoke passed"
