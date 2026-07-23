#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-storage-reclaim-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13338}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26392}"
GO_ADDR="${MOCHAT_SAAS_STORAGE_RECLAIM_GO_ADDR:-127.0.0.1:18105}"
FAKE_WECOM_PORT="${MOCHAT_SAAS_STORAGE_RECLAIM_FAKE_WECOM_PORT:-18106}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-storage-reclaim.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
STORAGE_ROOT="$WORK_DIR/storage"
GO_PID=""
FAKE_WECOM_PID=""

SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-storage-reclaim-secret}"
TENANT_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_TENANT_ID:-601}"
PHONE="${MOCHAT_SAAS_STORAGE_RECLAIM_PHONE:-13800000601}"
PASSWORD="${MOCHAT_SAAS_STORAGE_RECLAIM_PASSWORD:-secret601}"
CORP_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_CORP_ID:-601}"
EMPLOYEE_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_EMPLOYEE_ID:-601}"
MEDIUM_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_MEDIUM_ID:-601}"
RADAR_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_RADAR_ID:-601}"
LOTTERY_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_LOTTERY_ID:-601}"
LOTTERY_PRIZE_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_LOTTERY_PRIZE_ID:-601}"
LOTTERY_CONTACT_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_LOTTERY_CONTACT_ID:-601}"
LOTTERY_CONTACT_RECORD_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_LOTTERY_CONTACT_RECORD_ID:-601}"
SHOP_CODE_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_SHOP_CODE_ID:-601}"
SHOP_CODE_PAGE_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_SHOP_CODE_PAGE_ID:-601}"
FISSION_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_FISSION_ID:-601}"
ROOM_FISSION_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_ROOM_FISSION_ID:-601}"
ROOM_CLOCK_IN_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_ROOM_CLOCK_IN_ID:-601}"
ROOM_INFINITE_PULL_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_ROOM_INFINITE_PULL_ID:-601}"
CONTACT_BATCH_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_CONTACT_BATCH_ID:-601}"
CONTACT_BATCH_ADD_RECORD_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_CONTACT_BATCH_ADD_RECORD_ID:-601}"
CONTACT_BATCH_ADD_IMPORT_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_CONTACT_BATCH_ADD_IMPORT_ID:-601}"
ROOM_BATCH_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_ROOM_BATCH_ID:-601}"
ROOM_WELCOME_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_ROOM_WELCOME_ID:-601}"
ROOM_TAG_PULL_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_ROOM_TAG_PULL_ID:-601}"
WORK_ROOM_AUTO_PULL_ID="${MOCHAT_SAAS_STORAGE_RECLAIM_WORK_ROOM_AUTO_PULL_ID:-601}"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ -n "$FAKE_WECOM_PID" ] && kill -0 "$FAKE_WECOM_PID" 2>/dev/null; then
    kill "$FAKE_WECOM_PID" 2>/dev/null || true
    wait "$FAKE_WECOM_PID" 2>/dev/null || true
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
  local query="$1"
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat_saas_storage_reclaim -e "$query" | tr -d '\r'
}

redis_cli() {
  compose exec -T redis redis-cli "$@"
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"
assert_port_free "$FAKE_WECOM_PORT"

mkdir -p "$STORAGE_ROOT/welcome"
printf 'new room welcome image' >"$STORAGE_ROOT/welcome/new.png"

python3 - "$FAKE_WECOM_PORT" <<'PY' &
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import sys

port = int(sys.argv[1])

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path.startswith("/cgi-bin/gettoken"):
            self.send_json({"errcode": 0, "errmsg": "ok", "access_token": "fake-token", "expires_in": 7200})
            return
        self.send_json({"errcode": 404, "errmsg": "not found"}, 404)

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0") or "0")
        if length:
            self.rfile.read(length)
        if self.path.startswith("/cgi-bin/media/uploadimg"):
            self.send_json({"errcode": 0, "errmsg": "ok", "url": "https://wecom.example/welcome-new.png"})
            return
        if self.path.startswith("/cgi-bin/externalcontact/group_welcome_template/edit"):
            self.send_json({"errcode": 0, "errmsg": "ok"})
            return
        if self.path.startswith("/cgi-bin/externalcontact/group_welcome_template/del"):
            self.send_json({"errcode": 0, "errmsg": "ok"})
            return
        if self.path.startswith("/cgi-bin/externalcontact/add_contact_way"):
            self.send_json({"errcode": 40001, "errmsg": "channel rollback expected"})
            return
        if self.path.startswith("/cgi-bin/externalcontact/contact_way/update"):
            self.send_json({"errcode": 0, "errmsg": "ok"})
            return
        if self.path.startswith("/cgi-bin/externalcontact/contact_way/create"):
            self.send_json({"errcode": 40001, "errmsg": "rollback expected"})
            return
        self.send_json({"errcode": 404, "errmsg": "not found"}, 404)

    def log_message(self, *_):
        return

    def send_json(self, payload, status=200):
        raw = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

ThreadingHTTPServer(("127.0.0.1", port), Handler).serve_forever()
PY
FAKE_WECOM_PID="$!"
wait_url "http://127.0.0.1:$FAKE_WECOM_PORT/cgi-bin/gettoken" 200

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

mysql_root <<'SQL'
DROP DATABASE IF EXISTS mochat_saas_storage_reclaim;
CREATE DATABASE mochat_saas_storage_reclaim CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON mochat_saas_storage_reclaim.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat_saas_storage_reclaim?parseTime=true&loc=Local"

"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0004_saas_storage_objects\tapplied_now' "$WORK_DIR/migrate.out"

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id "$TENANT_ID" \
  -tenant-name "SaaS存储回收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "SaaS存储回收管理员" \
  -role-name "SaaS存储回收超级管理员" \
  -package-code "storage-reclaim" \
  -package-name "存储回收版" \
  -storage-mb 32 \
  -shop-codes 20 \
  -radars 20 \
  -lotteries 20 \
  -room-infinite-pulls 20 \
  -contact-message-batches 20 \
  -room-message-batches 20 \
  -room-fissions 20 \
  -room-clock-ins 20 \
  -room-tag-pulls 20 \
  -work-room-auto-pulls 20 \
  -work-fissions 20 >"$WORK_DIR/bootstrap.out"
grep -q $'tenant_id\t'"$TENANT_ID" "$WORK_DIR/bootstrap.out"

USER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '$PHONE' AND tenant_id = $TENANT_ID AND deleted_at IS NULL")"
ROLE_ID="$(mysql_scalar "SELECT id FROM mc_rbac_role WHERE tenant_id = $TENANT_ID AND name = 'SaaS存储回收超级管理员' AND deleted_at IS NULL")"
test -n "$USER_ID"
test -n "$ROLE_ID"

mysql_root mochat_saas_storage_reclaim <<SQL
INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '存储回收企业', 'wwreclaim601', 'employee-secret-reclaim', 'contact-secret-reclaim', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', $TENANT_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, gender, status, log_user_id, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'reclaim-user-601', $CORP_ID, 'SaaS存储回收管理员', '$PHONE', 1, 1, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_medium
  (id, media_id, last_upload_time, type, is_sync, content, corp_id, medium_group_id, user_id, user_name, created_at, updated_at, deleted_at)
VALUES
  ($MEDIUM_ID, '', 0, 2, 1, '{"imagePath":"medium/reclaim.png"}', $CORP_ID, 0, $USER_ID, 'SaaS存储回收管理员', NOW(), NOW(), NULL);

INSERT INTO mc_radar
  (id, type, title, link, link_title, link_description, link_cover, pdf_name, pdf, article_type, article, employee_card, action_notice, dynamic_notice, contact_tags, tag_status, contact_grade, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ($RADAR_ID, 2, '互动雷达存储回收', 'https://example.com/radar', '雷达标题', '雷达描述', 'radar/cover.png', 'radar.pdf', 'radar/file.pdf', 0, JSON_ARRAY(), 0, 0, 0, JSON_ARRAY(), 0, JSON_ARRAY(), $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_lottery
  (id, name, description, type, time_type, start_time, end_time, contact_tags, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ($LOTTERY_ID, '抽奖活动存储回收', '抽奖活动存储回收说明', 'roulette', 1, NULL, NULL, JSON_ARRAY(), $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_lottery_prize
  (id, lottery_id, prize_set, is_show, exchange_set, draw_set, win_set, corp_card, created_at, updated_at, deleted_at)
VALUES
  ($LOTTERY_PRIZE_ID, $LOTTERY_ID,
   JSON_ARRAY(JSON_OBJECT('name', '一等奖', 'image', 'lottery/prize.png', 'total', 1)),
   1,
   JSON_OBJECT('type', 1, 'qrcode', 'lottery/exchange.png'),
   JSON_OBJECT('daily', 1),
   JSON_OBJECT('max', 1),
   JSON_OBJECT('name', '存储回收企业', 'logo', 'https://cdn.example.com/lottery-logo.png'),
   NOW(), NOW(), NULL);

INSERT INTO mc_lottery_contact
  (id, lottery_id, union_id, contact_id, nickname, avatar, employee_ids, city, source, grade, contact_tags, draw_num, win_num, status, write_off, created_at, updated_at, deleted_at)
VALUES
  ($LOTTERY_CONTACT_ID, $LOTTERY_ID, 'lottery-union-601', 0, '抽奖客户', 'https://cdn.example.com/avatar.png', '$EMPLOYEE_ID', '', '', 0, JSON_ARRAY(), 1, 1, 1, 0, NOW(), NOW(), NULL);

INSERT INTO mc_lottery_contact_record
  (id, lottery_id, contact_id, prize_id, prize_name, receive_status, receive_qr, receive_type, receive_code, write_off, created_at, updated_at, deleted_at)
VALUES
  ($LOTTERY_CONTACT_RECORD_ID, $LOTTERY_ID, $LOTTERY_CONTACT_ID, $LOTTERY_PRIZE_ID, '一等奖', 0, 'lottery/receive.png', 1, '', 0, NOW(), NOW(), NULL);

INSERT INTO mc_shop_code
  (id, name, type, employee, employee_qrcode, qw_code, search_keyword, address, country, province, city, district, lat, lng, status, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ($SHOP_CODE_ID, '门店活码存储回收', 1,
   JSON_ARRAY(JSON_OBJECT('id', $EMPLOYEE_ID, 'name', 'SaaS存储回收管理员')),
   JSON_ARRAY(JSON_OBJECT('url', 'shopCode/employee-old.png', 'remoteUrl', 'https://cdn.example.com/employee.png')),
   JSON_ARRAY(JSON_OBJECT('roomQrcodeUrl', 'shopCode/room-old.png')),
   '存储回收', '杭州市西湖区', '中国', '浙江省', '杭州市', '西湖区', '30.1', '120.1', 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_shop_code_page
  (id, type, title, show_type, \`default\`, poster, autoPass, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ($SHOP_CODE_PAGE_ID, 1, '门店活码旧页面设置', 2,
   JSON_OBJECT('guide', '扫码添加店主', 'logo', 'shopCode/page-logo-old.png'),
   'shopCode/page-poster-old.png', 0, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_fission
  (id, corp_id, active_name, service_employees, auto_pass, auto_add_tag, contact_tags, end_time, qr_code_invalid, tasks, new_friend, delete_invalid, receive_prize, receive_prize_employees, receive_links, receive_qrcode, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ($FISSION_ID, $CORP_ID, '存储回收裂变', JSON_ARRAY(JSON_OBJECT('id', $EMPLOYEE_ID, 'wxUserId', 'reclaim-user-601')), 1, 0, JSON_ARRAY(), '2037-01-01 00:00:00', 7, JSON_ARRAY(), 1, 0, 1, JSON_ARRAY(), JSON_ARRAY(), JSON_ARRAY(), $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_fission_poster
  (fission_id, poster_type, cover_pic, wx_cover_pic, foward_text, avatar_show, nickname_show, nickname_color, card_corp_image_name, card_corp_name, card_corp_logo, qrcode_w, qrcode_h, qrcode_x, qrcode_y, qrcode_id, qrcode_url, created_at, updated_at, deleted_at)
VALUES
  ($FISSION_ID, 1, 'fission/poster.png', '', '转发话术', 1, 1, '#000000', '企业形象', '存储回收企业', 'fission/logo.png', '120', '120', '10', '10', '', '', NOW(), NOW(), NULL);

INSERT INTO mc_work_fission_welcome
  (fission_id, msg_text, link_title, link_desc, link_cover_url, link_wx_url, created_at, updated_at, deleted_at)
VALUES
  ($FISSION_ID, '欢迎语', '欢迎链接', '欢迎描述', 'fission/welcome.png', '', NOW(), NOW(), NULL);

INSERT INTO mc_work_fission_push
  (fission_id, push_employee, push_contact, msg_text, msg_complex, msg_complex_type, created_at, updated_at, deleted_at)
VALUES
  ($FISSION_ID, 1, 1, '推送语', '{"image":"fission/push.png","pic_url":"https://cdn.example.com/push.png"}', 'image', NOW(), NOW(), NULL);

INSERT INTO mc_work_fission_invite
  (fission_id, type, text, link_title, link_desc, link_pic, wx_link_pic, created_at, updated_at, deleted_at)
VALUES
  ($FISSION_ID, 1, '邀请文案', '邀请标题', '邀请描述', 'fission/invite.png', '', NOW(), NOW(), NULL);

INSERT INTO mc_room_fission
  (id, official_account_id, active_name, end_time, target_count, new_friend, delete_invalid, receive_employees, auto_pass, status, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_FISSION_ID, 0, '群裂变存储回收', '2037-01-01 00:00:00', 5, 1, 0, JSON_ARRAY($EMPLOYEE_ID), 1, 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_fission_poster
  (fission_id, cover_pic, avatar_show, nickname_show, nickname_color, qrcode_w, qrcode_h, qrcode_x, qrcode_y, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_FISSION_ID, 'roomFission/poster.png', 1, 1, '#000000', '120', '120', '10', '10', NOW(), NOW(), NULL);

INSERT INTO mc_room_fission_room
  (fission_id, room_qrcode, room_wx_qrcode, room, room_max, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_FISSION_ID, 'roomFission/room.png', 'https://wecom.example/room-fission-room.png', JSON_OBJECT('id', 900601, 'name', '群裂变存储回收群'), 200, NOW(), NOW(), NULL);

INSERT INTO mc_room_fission_welcome
  (fission_id, text, link_title, link_desc, link_pic, link_wx_url, template_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_FISSION_ID, '群裂变欢迎语', '欢迎标题', '欢迎描述', 'roomFission/welcome.png', 'https://wecom.example/room-fission-welcome.png', '', NOW(), NOW(), NULL);

INSERT INTO mc_room_fission_invite
  (fission_id, type, employees, choose_contact, text, link_title, link_desc, link_pic, wx_link_pic, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_FISSION_ID, 1, JSON_ARRAY($EMPLOYEE_ID), JSON_OBJECT(), '邀请入群', '邀请标题', '邀请描述', 'roomFission/invite.png', 'https://wecom.example/room-fission-invite.png', NOW(), NOW(), NULL);

INSERT INTO mc_room_clock_in
  (id, official_account_id, active_name, description, type, start_time, end_time, tasks, employee_qrcode, contact_tags, corp_card_status, corp_card, status, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_CLOCK_IN_ID, 0, '群打卡存储回收', '群打卡存储回收说明', 1, NOW(), '2037-01-01 00:00:00', JSON_ARRAY(JSON_OBJECT('count', 3, 'prize', '存储回收奖品')), 'clockIn/employee.png', JSON_ARRAY(), 0, JSON_OBJECT(), 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_infinite
  (id, name, avatar, title_status, title, describe_status, \`describe\`, logo, qw_code, total_num, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_INFINITE_PULL_ID, '无限拉群存储回收', 'roomInfinite/avatar.png', 1, '扫码入群', 1, '无限拉群存储回收说明', 'roomInfinite/logo.png',
   JSON_ARRAY(JSON_OBJECT('qrcode', 'roomInfinite/qrcode.png', 'upper_limit', 200, 'status', 1)),
   0, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_contact_message_batch_send
  (id, corp_id, user_id, user_name, employee_ids, filter_params, filter_params_detail, content,
   send_way, definite_time, send_time, send_employee_total, send_contact_total, send_total,
   not_send_total, received_total, not_received_total, receive_limit_total, not_friend_total,
   send_status, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_BATCH_ID, $CORP_ID, $USER_ID, 'SaaS存储回收管理员', JSON_ARRAY($EMPLOYEE_ID), JSON_OBJECT(),
   JSON_OBJECT('rooms', JSON_ARRAY(), 'tags', JSON_ARRAY(), 'excludeContacts', JSON_ARRAY()),
   JSON_ARRAY(JSON_OBJECT('msgType', 'image', 'pic_url', 'batch/contact.png')),
   2, '2037-01-01 00:00:00', NULL, 1, 1, 0, 1, 0, 1, 0, 0, 0, NOW(), NOW(), NULL);

INSERT INTO mc_contact_batch_add_import_record
  (id, corp_id, title, upload_at, allot_employee, tags, import_num, add_num, file_name, file_url, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_BATCH_ADD_RECORD_ID, $CORP_ID, '批量加好友存储回收', NOW(), JSON_ARRAY($EMPLOYEE_ID), JSON_ARRAY(), 1, 0, 'reclaim.csv', 'contactBatchAdd/import/reclaim.csv', NOW(), NOW(), NULL);

INSERT INTO mc_contact_batch_add_import
  (id, corp_id, record_id, phone, upload_at, status, add_at, employee_id, allot_num, remark, tags, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_BATCH_ADD_IMPORT_ID, $CORP_ID, $CONTACT_BATCH_ADD_RECORD_ID, '13800000602', NOW(), 1, NULL, $EMPLOYEE_ID, 1, '批量加好友存储回收客户', JSON_ARRAY(), NOW(), NOW(), NULL);

INSERT INTO mc_room_message_batch_send
  (id, corp_id, user_id, user_name, employee_ids, batch_title, content,
   send_way, definite_time, send_time, send_room_total, send_employee_total, send_total,
   not_send_total, received_total, not_received_total, send_status, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_BATCH_ID, $CORP_ID, $USER_ID, 'SaaS存储回收管理员', JSON_ARRAY($EMPLOYEE_ID), 'SaaS存储回收客户群群发',
   JSON_ARRAY(JSON_OBJECT('msgType', 'image', 'pic_url', 'batch/room.png')),
   2, '2037-01-01 00:00:00', NULL, 1, 1, 0, 1, 0, 1, 0, NOW(), NOW(), NULL);

INSERT INTO mc_room_welcome_template
  (id, corp_id, msg_text, complex_type, msg_complex, complex_template_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_WELCOME_ID, $CORP_ID, '入群欢迎语存储回收', 'image',
   JSON_OBJECT('pic', 'welcome/old.png', 'pic_url', 'https://wecom.example/welcome-old.png'),
   'tpl-storage-reclaim', $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_tag_pull
  (id, name, employees, choose_contact, guide, rooms, filter_contact, contact_num, wx_tid, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_TAG_PULL_ID, '标签建群存储回收', '$EMPLOYEE_ID', JSON_OBJECT('is_all', 1), '请扫码入群',
   JSON_ARRAY(JSON_OBJECT('id', 900001, 'name', '标签建群客户群', 'num', 50, 'image', 'roomtag/room.png', 'wx_image', 'https://wecom.example/roomtag.png')),
   1, 0, JSON_ARRAY(), $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_room_auto_pull
  (id, corp_id, qrcode_name, qrcode_url, wx_config_id, is_verified, leading_words, tags, employees, rooms, created_at, updated_at, deleted_at)
VALUES
  ($WORK_ROOM_AUTO_PULL_ID, $CORP_ID, '自动拉群存储回收', 'https://wecom.example/auto-pull-qrcode.png',
   'auto-pull-config-storage-reclaim', 1, '欢迎入群', JSON_ARRAY('900001'), JSON_ARRAY('$EMPLOYEE_ID'),
   JSON_ARRAY(JSON_OBJECT('roomId', 900002, 'maxNum', 40, 'roomQrcodeUrl', 'autopull/old.png')),
   NOW(), NOW(), NULL);

INSERT INTO mochat_go_saas_storage_objects
  (tenant_id, user_id, employee_id, corp_id, source, original_name, relative_path, content_type, size_bytes, created_at, updated_at, deleted_at)
VALUES
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'reclaim.png', 'medium/reclaim.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'reclaim-new.png', 'medium/reclaim-new.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.radar.store', 'cover.png', 'radar/cover.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.radar.store', 'file.pdf', 'radar/file.pdf', 'application/pdf', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.lottery.store', 'prize.png', 'lottery/prize.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.lottery.store', 'exchange.png', 'lottery/exchange.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.lottery.record', 'receive.png', 'lottery/receive.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.shopCode.store', 'employee-old.png', 'shopCode/employee-old.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.shopCode.store', 'room-old.png', 'shopCode/room-old.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.shopCode.pageSet', 'page-logo-old.png', 'shopCode/page-logo-old.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.shopCode.pageSet', 'page-poster-old.png', 'shopCode/page-poster-old.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'poster.png', 'fission/poster.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'logo.png', 'fission/logo.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'welcome.png', 'fission/welcome.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'push.png', 'fission/push.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'invite.png', 'fission/invite.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'contact.png', 'batch/contact.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'room.png', 'batch/room.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'old.png', 'welcome/old.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'new.png', 'welcome/new.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'roomtag.png', 'roomtag/room.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'auto-pull-old.png', 'autopull/old.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.common.upload', 'auto-pull-rollback.png', 'autopull/rollback.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.contactBatchAdd.importStore', 'reclaim.csv', 'contactBatchAdd/import/reclaim.csv', 'text/csv', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.roomFission.store', 'poster.png', 'roomFission/poster.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.roomFission.store', 'room.png', 'roomFission/room.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.roomFission.store', 'welcome.png', 'roomFission/welcome.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.roomFission.invite', 'invite.png', 'roomFission/invite.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.roomClockIn.store', 'employee.png', 'clockIn/employee.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.roomInfinitePull.store', 'avatar.png', 'roomInfinite/avatar.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.roomInfinitePull.store', 'logo.png', 'roomInfinite/logo.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.roomInfinitePull.store', 'qrcode.png', 'roomInfinite/qrcode.png', 'image/png', 1048576, NOW(), NOW(), NULL),
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'dashboard.channelCode.store', 'rollback.png', 'channelCode/rollback.png', 'image/png', 1048576, NOW(), NOW(), NULL);

UPDATE mochat_go_saas_usage_counters
SET used_value = 32,
    updated_by = 'seed',
    updated_at = NOW()
WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL;

UPDATE mochat_go_saas_usage_counters
SET used_value = 1,
    updated_by = 'seed',
    updated_at = NOW()
WHERE tenant_id = $TENANT_ID
  AND metric IN ('shop_codes', 'radars', 'lotteries', 'contact_message_batches', 'room_message_batches', 'room_fissions', 'room_clock_ins', 'room_infinite_pulls', 'room_tag_pulls', 'work_room_auto_pulls', 'work_fissions')
  AND deleted_at IS NULL;
SQL

test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "32"
test "$(mysql_scalar "SELECT SUM(used_value) FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric IN ('shop_codes', 'radars', 'lotteries', 'contact_message_batches', 'room_message_batches', 'room_fissions', 'room_clock_ins', 'room_infinite_pulls', 'room_tag_pulls', 'work_room_auto_pulls', 'work_fissions') AND deleted_at IS NULL")" = "11"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'medium/reclaim.png' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'medium/reclaim-new.png' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'radar/%' AND deleted_at IS NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'lottery/%' AND deleted_at IS NULL")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'shopCode/%' AND deleted_at IS NULL")" = "4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'fission/%' AND deleted_at IS NULL")" = "5"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'batch/%' AND deleted_at IS NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'welcome/%' AND deleted_at IS NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'roomtag/%' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'autopull/%' AND deleted_at IS NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'contactBatchAdd/import/reclaim.csv' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'roomFission/%' AND deleted_at IS NULL")" = "4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'clockIn/%' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'roomInfinite/%' AND deleted_at IS NULL")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'channelCode/rollback.png' AND deleted_at IS NULL")" = "1"

redis_cli SET "mc:user.$USER_ID" "$CORP_ID-$EMPLOYEE_ID" >/dev/null

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_FILE_STORAGE_ROOT="$STORAGE_ROOT" \
  MOCHAT_WECOM_API_BASE_URL="http://127.0.0.1:$FAKE_WECOM_PORT" \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_MIGRATE_MEDIUM_UPDATE=1 \
  MOCHAT_GO_MIGRATE_MEDIUM_DESTROY=1 \
  MOCHAT_GO_MIGRATE_WORK_FISSION_DESTROY=1 \
  MOCHAT_GO_MIGRATE_LOTTERY_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_SHOP_CODE_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_DESTROY=1 \
  MOCHAT_GO_MIGRATE_CONTACT_BATCH_ADD_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_ROOM_FISSION_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_RADAR_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_ROOM_CLOCK_IN_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_ROOM_INFINITE_PULL_DASHBOARD=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_DESTROY=1 \
  MOCHAT_GO_MIGRATE_ROOM_WELCOME_UPDATE=1 \
  MOCHAT_GO_MIGRATE_ROOM_WELCOME_DESTROY=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_DESTROY=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_STORE=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_UPDATE=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_STORE=1 \
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

channel_code_store_code="$(curl -sS -o "$WORK_DIR/channel-code-store-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"baseInfo\":{\"groupId\":0,\"name\":\"渠道活码失败回滚\",\"autoAddFriend\":1,\"tags\":[]},\"drainageEmployee\":{\"type\":1,\"employees\":[],\"specialPeriod\":{\"status\":1,\"detail\":[{\"startDate\":\"2026-01-01\",\"endDate\":\"2037-12-31\",\"timeSlot\":[{\"startTime\":\"00:00\",\"endTime\":\"00:00\",\"employeeId\":[${EMPLOYEE_ID}]}]}]},\"addMax\":{\"status\":2,\"employees\":[],\"spareEmployeeIds\":[]}},\"welcomeMessage\":{\"scanCodePush\":2,\"messageDetail\":[{\"type\":2,\"pic_url\":\"channelCode/rollback.png\"}]}}" \
  "http://$GO_ADDR/dashboard/channelCode/store")"
test "$channel_code_store_code" = "400"

python3 - "$WORK_DIR/channel-code-store-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 400, payload
assert "channel rollback expected" in payload["msg"], payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_channel_code WHERE name = '渠道活码失败回滚' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'channelCode/rollback.png' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'channel_codes' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "32"

room_fission_invite_update_code="$(curl -sS -o "$WORK_DIR/room-fission-invite-update-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"fissionId\":$ROOM_FISSION_ID,\"invite\":{\"type\":1,\"employees\":[$EMPLOYEE_ID],\"chooseContact\":{},\"text\":\"邀请配置替换图片\",\"linkTitle\":\"邀请新标题\",\"linkDesc\":\"邀请新描述\",\"linkPic\":\"roomFission/invite-new.png\",\"wxLinkPic\":\"https://wecom.example/room-fission-invite-new.png\"}}" \
  "http://$GO_ADDR/dashboard/roomFission/invite")"
test "$room_fission_invite_update_code" = "200"

python3 - "$WORK_DIR/room-fission-invite-update-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission_invite WHERE fission_id = $ROOM_FISSION_ID AND deleted_at IS NULL AND link_pic = 'roomFission/invite-new.png'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'roomFission/invite.png' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'roomFission/%' AND deleted_at IS NULL")" = "3"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "31"

room_fission_delete_code="$(curl -sS -o "$WORK_DIR/room-fission-delete-response.json" -w '%{http_code}' \
  -X DELETE \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"fissionId\":$ROOM_FISSION_ID}" \
  "http://$GO_ADDR/dashboard/roomFission/destroy")"
test "$room_fission_delete_code" = "200"

python3 - "$WORK_DIR/room-fission-delete-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission WHERE id = $ROOM_FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission_poster WHERE fission_id = $ROOM_FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission_room WHERE fission_id = $ROOM_FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission_welcome WHERE fission_id = $ROOM_FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission_invite WHERE fission_id = $ROOM_FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'roomFission/%' AND deleted_at IS NOT NULL")" = "4"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_fissions' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "28"

radar_delete_code="$(curl -sS -o "$WORK_DIR/radar-delete-response.json" -w '%{http_code}' \
  -X DELETE \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"radarId\":$RADAR_ID}" \
  "http://$GO_ADDR/dashboard/radar/destroy")"
test "$radar_delete_code" = "200"

python3 - "$WORK_DIR/radar-delete-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_radar WHERE id = $RADAR_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'radar/%' AND deleted_at IS NOT NULL")" = "2"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'radars' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "26"

room_clock_in_delete_code="$(curl -sS -o "$WORK_DIR/room-clock-in-delete-response.json" -w '%{http_code}' \
  -X DELETE \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"clockInId\":$ROOM_CLOCK_IN_ID}" \
  "http://$GO_ADDR/dashboard/roomClockIn/destroy")"
test "$room_clock_in_delete_code" = "200"

python3 - "$WORK_DIR/room-clock-in-delete-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_clock_in WHERE id = $ROOM_CLOCK_IN_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'clockIn/%' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_clock_ins' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "25"

room_infinite_pull_delete_code="$(curl -sS -o "$WORK_DIR/room-infinite-pull-delete-response.json" -w '%{http_code}' \
  -X DELETE \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"roomInfinitePullId\":$ROOM_INFINITE_PULL_ID}" \
  "http://$GO_ADDR/dashboard/roomInfinitePull/destroy")"
test "$room_infinite_pull_delete_code" = "200"

python3 - "$WORK_DIR/room-infinite-pull-delete-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_infinite WHERE id = $ROOM_INFINITE_PULL_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'roomInfinite/%' AND deleted_at IS NOT NULL")" = "3"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_infinite_pulls' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "22"

lottery_delete_code="$(curl -sS -o "$WORK_DIR/lottery-delete-response.json" -w '%{http_code}' \
  -X DELETE \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"lotteryId\":$LOTTERY_ID}" \
  "http://$GO_ADDR/dashboard/lottery/destroy")"
test "$lottery_delete_code" = "200"

python3 - "$WORK_DIR/lottery-delete-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_lottery WHERE id = $LOTTERY_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_lottery_prize WHERE lottery_id = $LOTTERY_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_lottery_contact WHERE lottery_id = $LOTTERY_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_lottery_contact_record WHERE lottery_id = $LOTTERY_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'lottery/%' AND deleted_at IS NOT NULL")" = "3"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'lotteries' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "19"

shop_code_page_set_code="$(curl -sS -o "$WORK_DIR/shop-code-page-set-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"type\":1,\"title\":\"门店活码新页面设置\",\"showType\":1,\"default\":{\"guide\":\"扫码添加店主\",\"logo\":\"https://cdn.example.com/new-logo.png\"},\"poster\":\"\",\"autoPass\":0}" \
  "http://$GO_ADDR/dashboard/shopCode/pageSet")"
test "$shop_code_page_set_code" = "200"

python3 - "$WORK_DIR/shop-code-page-set-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_shop_code_page WHERE id = $SHOP_CODE_PAGE_ID AND deleted_at IS NULL AND poster = '' AND JSON_UNQUOTE(JSON_EXTRACT(\`default\`, '$.logo')) = 'https://cdn.example.com/new-logo.png'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path IN ('shopCode/page-logo-old.png', 'shopCode/page-poster-old.png') AND deleted_at IS NOT NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path IN ('shopCode/employee-old.png', 'shopCode/room-old.png') AND deleted_at IS NULL")" = "2"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "17"

shop_code_update_code="$(curl -sS -o "$WORK_DIR/shop-code-update-response.json" -w '%{http_code}' \
  -X PUT \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"shopCodeId\":$SHOP_CODE_ID,\"employeeQrcode\":[]}" \
  "http://$GO_ADDR/dashboard/shopCode/update")"
test "$shop_code_update_code" = "200"

python3 - "$WORK_DIR/shop-code-update-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_shop_code WHERE id = $SHOP_CODE_ID AND deleted_at IS NULL AND JSON_LENGTH(employee_qrcode) = 0")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'shopCode/employee-old.png' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'shopCode/room-old.png' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "16"

shop_code_delete_code="$(curl -sS -o "$WORK_DIR/shop-code-delete-response.json" -w '%{http_code}' \
  -X DELETE \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"shopCodeId\":$SHOP_CODE_ID}" \
  "http://$GO_ADDR/dashboard/shopCode/destroy")"
test "$shop_code_delete_code" = "200"

python3 - "$WORK_DIR/shop-code-delete-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_shop_code WHERE id = $SHOP_CODE_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'shopCode/%' AND deleted_at IS NOT NULL")" = "4"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'shop_codes' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "15"

update_code="$(curl -sS -o "$WORK_DIR/update-response.json" -w '%{http_code}' \
  -X PUT \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"id\":$MEDIUM_ID,\"type\":2,\"mediumGroupId\":0,\"isSync\":1,\"content\":{\"imagePath\":\"medium/reclaim-new.png\"}}" \
  "http://$GO_ADDR/dashboard/medium/update")"
test "$update_code" = "200"

python3 - "$WORK_DIR/update-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'medium/reclaim.png' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'medium/reclaim-new.png' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "14"

delete_code="$(curl -sS -o "$WORK_DIR/delete-response.json" -w '%{http_code}' \
  -X DELETE \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"id\":$MEDIUM_ID}" \
  "http://$GO_ADDR/dashboard/medium/destroy")"
test "$delete_code" = "200"

python3 - "$WORK_DIR/delete-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_medium WHERE id = $MEDIUM_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'medium/reclaim-new.png' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "13"

contact_batch_delete_code="$(curl -sS -o "$WORK_DIR/contact-batch-delete-response.json" -w '%{http_code}' \
  -X DELETE \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"batchId\":$CONTACT_BATCH_ID}" \
  "http://$GO_ADDR/dashboard/contactMessageBatchSend/destroy")"
test "$contact_batch_delete_code" = "200"

python3 - "$WORK_DIR/contact-batch-delete-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_message_batch_send WHERE id = $CONTACT_BATCH_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'batch/contact.png' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'contact_message_batches' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "12"

room_batch_delete_code="$(curl -sS -o "$WORK_DIR/room-batch-delete-response.json" -w '%{http_code}' \
  -X DELETE \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"batchId\":$ROOM_BATCH_ID}" \
  "http://$GO_ADDR/dashboard/roomMessageBatchSend/destroy")"
test "$room_batch_delete_code" = "200"

python3 - "$WORK_DIR/room-batch-delete-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_message_batch_send WHERE id = $ROOM_BATCH_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'batch/room.png' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_message_batches' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "11"

room_welcome_update_code="$(curl -sS -o "$WORK_DIR/room-welcome-update-response.json" -w '%{http_code}' \
  -X PUT \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"id\":$ROOM_WELCOME_ID,\"notice\":1,\"msg_text\":\"入群欢迎语存储回收更新\",\"msg_complex\":{\"type\":\"image\",\"image\":{\"pic\":\"welcome/new.png\"}}}" \
  "http://$GO_ADDR/dashboard/roomWelcome/update")"
test "$room_welcome_update_code" = "200"

python3 - "$WORK_DIR/room-welcome-update-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'welcome/old.png' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'welcome/new.png' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "10"

room_welcome_delete_code="$(curl -sS -o "$WORK_DIR/room-welcome-delete-response.json" -w '%{http_code}' \
  -X DELETE \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"id\":$ROOM_WELCOME_ID}" \
  "http://$GO_ADDR/dashboard/roomWelcome/destroy")"
test "$room_welcome_delete_code" = "200"

python3 - "$WORK_DIR/room-welcome-delete-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_welcome_template WHERE id = $ROOM_WELCOME_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'welcome/new.png' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "9"

room_tag_pull_delete_code="$(curl -sS -o "$WORK_DIR/room-tag-pull-delete-response.json" -w '%{http_code}' \
  -X DELETE \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"id\":$ROOM_TAG_PULL_ID}" \
  "http://$GO_ADDR/dashboard/roomTagPull/destroy")"
test "$room_tag_pull_delete_code" = "200"

python3 - "$WORK_DIR/room-tag-pull-delete-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_tag_pull WHERE id = $ROOM_TAG_PULL_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'roomtag/room.png' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'room_tag_pulls' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "8"

work_room_auto_pull_update_code="$(curl -sS -o "$WORK_DIR/work-room-auto-pull-update-response.json" -w '%{http_code}' \
  -X PUT \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"workRoomAutoPullId\":$WORK_ROOM_AUTO_PULL_ID,\"isVerified\":1,\"employees\":\"$EMPLOYEE_ID\",\"tags\":\"900001\",\"rooms\":[]}" \
  "http://$GO_ADDR/dashboard/workRoomAutoPull/update")"
test "$work_room_auto_pull_update_code" = "200"

python3 - "$WORK_DIR/work-room-auto-pull-update-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_room_auto_pull WHERE id = $WORK_ROOM_AUTO_PULL_ID AND deleted_at IS NULL AND JSON_LENGTH(rooms) = 0")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'autopull/old.png' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "7"

mysql_root mochat_saas_storage_reclaim <<SQL
UPDATE mochat_go_saas_usage_counters
SET used_value = 2,
    updated_by = 'seed-stale',
    updated_at = NOW()
WHERE tenant_id = $TENANT_ID AND metric = 'work_room_auto_pulls' AND deleted_at IS NULL;
SQL

work_room_auto_pull_store_code="$(curl -sS -o "$WORK_DIR/work-room-auto-pull-store-response.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"corpId\":$CORP_ID,\"qrcodeName\":\"自动拉群失败回滚\",\"isVerified\":1,\"leadingWords\":\"欢迎入群\",\"employees\":\"$EMPLOYEE_ID\",\"tags\":\"900001\",\"rooms\":\"[{\\\"roomId\\\":900003,\\\"maxNum\\\":40,\\\"roomQrcodeUrl\\\":\\\"autopull/rollback.png\\\"}]\"}" \
  "http://$GO_ADDR/dashboard/workRoomAutoPull/store")"
test "$work_room_auto_pull_store_code" = "400"

python3 - "$WORK_DIR/work-room-auto-pull-store-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 400, payload
assert "请求微信服务器创建二维码失败" in payload["msg"], payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_room_auto_pull WHERE qrcode_name = '自动拉群失败回滚' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'autopull/rollback.png' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'work_room_auto_pulls' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "6"

contact_batch_add_import_delete_code="$(curl -sS -o "$WORK_DIR/contact-batch-add-import-delete-response.json" -w '%{http_code}' \
  -X DELETE \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"id\":$CONTACT_BATCH_ADD_RECORD_ID}" \
  "http://$GO_ADDR/dashboard/contactBatchAdd/importDestroy")"
test "$contact_batch_add_import_delete_code" = "200"

python3 - "$WORK_DIR/contact-batch-add-import-delete-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
assert payload["data"]["delRecordNum"] == 1, payload
assert payload["data"]["delContactNum"] == 1, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_batch_add_import_record WHERE id = $CONTACT_BATCH_ADD_RECORD_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_batch_add_import WHERE id = $CONTACT_BATCH_ADD_IMPORT_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path = 'contactBatchAdd/import/reclaim.csv' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "5"

fission_delete_code="$(curl -sS -o "$WORK_DIR/fission-delete-response.json" -w '%{http_code}' \
  -X DELETE \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"id\":$FISSION_ID}" \
  "http://$GO_ADDR/dashboard/workFission/destroy")"
test "$fission_delete_code" = "200"

python3 - "$WORK_DIR/fission-delete-response.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission WHERE id = $FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_poster WHERE fission_id = $FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_welcome WHERE fission_id = $FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_push WHERE fission_id = $FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_invite WHERE fission_id = $FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'fission/%' AND deleted_at IS NOT NULL")" = "5"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'work_fissions' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'radar/%' AND deleted_at IS NOT NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'lottery/%' AND deleted_at IS NOT NULL")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'shopCode/%' AND deleted_at IS NOT NULL")" = "4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'clockIn/%' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'roomInfinite/%' AND deleted_at IS NOT NULL")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'batch/%' AND deleted_at IS NOT NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'welcome/%' AND deleted_at IS NOT NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'roomtag/%' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'autopull/%' AND deleted_at IS NOT NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND relative_path LIKE 'contactBatchAdd/import/%' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'storage_mb' AND deleted_at IS NULL")" = "32"

echo "saas storage reclaim smoke passed"
