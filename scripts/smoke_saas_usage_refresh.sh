#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-usage-refresh-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13337}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-usage-refresh.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
MAINTENANCE_BIN="$WORK_DIR/mochat-saas-maintenance"

SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-usage-refresh-secret}"
TENANT_ID="${MOCHAT_SAAS_USAGE_REFRESH_TENANT_ID:-501}"
PHONE="${MOCHAT_SAAS_USAGE_REFRESH_PHONE:-13800000501}"
PASSWORD="${MOCHAT_SAAS_USAGE_REFRESH_PASSWORD:-secret501}"
CORP_ID="${MOCHAT_SAAS_USAGE_REFRESH_CORP_ID:-501}"
EMPLOYEE_ID="${MOCHAT_SAAS_USAGE_REFRESH_EMPLOYEE_ID:-501}"
AGENT_ID="${MOCHAT_SAAS_USAGE_REFRESH_AGENT_ID:-501}"
CONTACT_ID="${MOCHAT_SAAS_USAGE_REFRESH_CONTACT_ID:-501}"
ROOM_ID="${MOCHAT_SAAS_USAGE_REFRESH_ROOM_ID:-501}"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
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

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  local query="$1"
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat_saas_usage_refresh -e "$query" | tr -d '\r'
}

assert_metric() {
  local metric="$1"
  local used="$2"
  local limit="$3"
  test "$(mysql_scalar "SELECT CONCAT(used_value, '/', limit_value, '/', updated_by) FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = '$metric' AND period_key = 'lifetime' AND deleted_at IS NULL")" = "$used/$limit/maintenance"
}

assert_port_free "$MYSQL_PORT"

compose up -d mysql
wait_service_healthy mysql

mysql_root <<'SQL'
DROP DATABASE IF EXISTS mochat_saas_usage_refresh;
CREATE DATABASE mochat_saas_usage_refresh CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON mochat_saas_usage_refresh.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$MAINTENANCE_BIN" ./cmd/mochat-saas-maintenance

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat_saas_usage_refresh?parseTime=true&loc=Local"

"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0004_saas_storage_objects\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0005_background_tasks\tapplied_now' "$WORK_DIR/migrate.out"

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id "$TENANT_ID" \
  -tenant-name "SaaS用量重刷租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "SaaS用量管理员" \
  -role-name "SaaS用量超级管理员" \
  -package-code "usage-refresh" \
  -package-name "用量重刷版" \
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

USER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '$PHONE' AND tenant_id = $TENANT_ID AND deleted_at IS NULL")"
test -n "$USER_ID"

mysql_root mochat_saas_usage_refresh <<SQL
INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '用量重刷企业', 'wwusage501', 'employee-secret-usage', 'contact-secret-usage', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', $TENANT_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, gender, status, log_user_id, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'usage-user-501', $CORP_ID, 'SaaS用量管理员', '$PHONE', 1, 1, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_agent
  (id, corp_id, wx_agent_id, wx_secret, name, created_at, updated_at, deleted_at)
VALUES
  ($AGENT_ID, $CORP_ID, '1000501', 'agent-secret-usage', '用量重刷应用', NOW(), NOW(), NULL);

INSERT INTO mc_channel_code
  (corp_id, group_id, name, qrcode_url, wx_config_id, auto_add_friend, tags, type, drainage_employee, welcome_message, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, 0, '用量重刷渠道活码', 'https://wecom.example/usage-channel.png', 'channel-config-usage', 1, JSON_ARRAY(), 1, JSON_OBJECT(), JSON_OBJECT(), NOW(), NOW(), NULL);

INSERT INTO mc_shop_code
  (name, type, employee, employee_qrcode, qw_code, search_keyword, address, country, province, city, district, lat, lng, status, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('用量重刷门店活码', 1, JSON_ARRAY(JSON_OBJECT('id', $EMPLOYEE_ID, 'name', 'SaaS用量管理员')), JSON_ARRAY(), JSON_ARRAY(), '用量门店', '用量街道1号', '中国', '浙江省', '杭州市', '西湖区', '30.1', '120.1', 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_radar
  (type, title, link, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  (1, '用量重刷互动雷达', 'https://example.com/usage-radar', $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_lottery
  (name, description, type, time_type, contact_tags, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('用量重刷抽奖活动', '用量重刷抽奖活动说明', 'roulette', 1, JSON_ARRAY(), $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_infinite
  (name, avatar, title_status, title, describe_status, \`describe\`, logo, qw_code, total_num, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('用量重刷无限拉群', '', 1, '', 1, '扫码入群', '', JSON_ARRAY(), 0, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_fission
  (official_account_id, active_name, end_time, target_count, new_friend, delete_invalid, receive_employees, auto_pass, status, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  (0, '用量重刷群裂变', '2037-01-01 00:00:00', 1, 0, 0, JSON_ARRAY(), 1, 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_clock_in
  (official_account_id, active_name, description, type, start_time, end_time, tasks, employee_qrcode, contact_tags, corp_card_status, corp_card, status, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  (0, '用量重刷群打卡', '用量重刷群打卡说明', 1, NULL, '2037-01-01 00:00:00', JSON_ARRAY(), '', JSON_ARRAY(), 0, JSON_OBJECT(), 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_quality
  (name, description, \`rule\`, rooms, status, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('用量重刷群质检', '用量重刷群质检说明', JSON_ARRAY(), JSON_ARRAY(), 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_calendar
  (name, rooms, on_off, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('用量重刷群日历', JSON_ARRAY(), 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_room_remind
  (name, rooms, is_qrcode, is_link, is_miniprogram, is_card, is_keyword, keyword, status, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('用量重刷客户群提醒', JSON_ARRAY(), 0, 0, 0, 0, 0, '', 1, $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_contact_sop
  (corp_id, creator_id, name, setting, employee_ids, state, contact_ids, created_at, updated_at)
VALUES
  ($CORP_ID, $USER_ID, '用量重刷个人SOP', JSON_ARRAY(JSON_OBJECT('name', '用量规则', 'content', JSON_ARRAY(JSON_OBJECT('type', 'text', 'value', '用量重刷个人SOP')))), JSON_ARRAY($EMPLOYEE_ID), 1, JSON_ARRAY($CONTACT_ID), NOW(), NOW());

INSERT INTO mc_room_sop
  (corp_id, creator_id, name, setting, room_ids, state, created_at, updated_at)
VALUES
  ($CORP_ID, $USER_ID, '用量重刷群SOP', JSON_ARRAY(JSON_OBJECT('name', '用量规则', 'content', JSON_ARRAY(JSON_OBJECT('type', 'text', 'value', '用量重刷群SOP')))), JSON_ARRAY($ROOM_ID), 1, NOW(), NOW());

INSERT INTO mc_sensitive_word_group
  (id, corp_id, name, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, $CORP_ID, '用量重刷敏感词分组', NOW(), NOW(), NULL);

INSERT INTO mc_sensitive_word
  (corp_id, group_id, name, status, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, $CORP_ID, '用量重刷敏感词', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, type, gender, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ID, $CORP_ID, 'external-usage-501', '用量重刷客户', 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_room
  (id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_ID, $CORP_ID, 'room-usage-501', '用量重刷客户群', $EMPLOYEE_ID, '', 0, NOW(), 200, NOW(), NOW(), NULL);

INSERT INTO mc_contact_message_batch_send
  (corp_id, user_id, user_name, employee_ids, filter_params, filter_params_detail, content, send_way, definite_time, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, $USER_ID, 'SaaS用量管理员', JSON_ARRAY($EMPLOYEE_ID), JSON_OBJECT(), JSON_OBJECT('rooms', JSON_ARRAY(), 'tags', JSON_ARRAY(), 'excludeContacts', JSON_ARRAY()), JSON_ARRAY(JSON_OBJECT('msgType', 'text', 'content', '用量重刷客户群发')), 2, '2026-07-04 10:00:00', NOW(), NOW(), NULL);

INSERT INTO mc_room_message_batch_send
  (corp_id, user_id, user_name, employee_ids, batch_title, content, send_way, definite_time, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, $USER_ID, 'SaaS用量管理员', JSON_ARRAY($EMPLOYEE_ID), '用量重刷客户群群发', JSON_ARRAY(JSON_OBJECT('msgType', 'text', 'content', '用量重刷客户群群发')), 2, '2026-07-04 10:00:00', NOW(), NOW(), NULL);

INSERT INTO mc_room_tag_pull
  (name, employees, choose_contact, guide, rooms, filter_contact, contact_num, wx_tid, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('用量重刷标签建群', CAST($EMPLOYEE_ID AS CHAR), JSON_OBJECT('is_all', 1), '扫码入群', JSON_ARRAY(JSON_OBJECT('id', $ROOM_ID, 'name', '用量重刷客户群', 'num', 50)), 1, 1, JSON_ARRAY(), $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_room_auto_pull
  (corp_id, qrcode_name, qrcode_url, wx_config_id, is_verified, leading_words, tags, employees, rooms, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '用量重刷自动拉群', 'https://wecom.example/usage-auto.png', 'auto-config-usage', 2, '欢迎入群', JSON_ARRAY(), JSON_ARRAY(CAST($EMPLOYEE_ID AS CHAR)), JSON_ARRAY(JSON_OBJECT('roomId', $ROOM_ID, 'maxNum', 50)), NOW(), NOW(), NULL);

INSERT INTO mc_work_fission
  (corp_id, active_name, service_employees, auto_pass, auto_add_tag, contact_tags, end_time, qr_code_invalid, tasks, new_friend, delete_invalid, receive_prize, receive_prize_employees, receive_links, receive_qrcode, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '用量重刷裂变', JSON_ARRAY(JSON_OBJECT('id', $EMPLOYEE_ID, 'wxUserId', 'usage-user-501')), 1, 0, JSON_ARRAY(), '2037-01-01 00:00:00', 7, JSON_ARRAY(), 1, 0, 0, JSON_ARRAY(), JSON_ARRAY(), JSON_ARRAY(), $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_official_account
  (appid, authorized_status, authorizer_appid, nickname, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('component-appid', 1, 'official-usage-501', '用量重刷公众号', $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mochat_go_saas_storage_objects
  (tenant_id, user_id, employee_id, corp_id, source, original_name, relative_path, content_type, size_bytes, created_at, updated_at, deleted_at)
VALUES
  ($TENANT_ID, $USER_ID, 0, $CORP_ID, 'seed.usage-refresh', 'usage.bin', 'usage/usage.bin', 'application/octet-stream', 3145728, NOW(), NOW(), NULL);

INSERT INTO mochat_go_background_task_executions
  (tenant_id, execution_id, task_run_id, task_name, kind, status, started_at, stopped_at, duration_ms, created_at, updated_at)
VALUES
  ($TENANT_ID, 'usage-refresh-queue-1', 'usage-refresh-run-1', 'wework-callback', 'queue_item', 'succeeded', NOW(), NOW(), 10, NOW(), NOW());

UPDATE mochat_go_saas_tenant_packages
SET limits_json = '{"maxCorps":5,"maxUsers":6,"maxContacts":7,"maxRooms":8,"maxAgents":9,"channelCodes":17,"shopCodes":19,"radars":21,"lotteries":22,"roomInfinitePulls":23,"roomFissions":24,"roomClockIns":25,"roomQualities":26,"roomCalendars":27,"roomReminds":28,"contactSops":29,"roomSops":30,"sensitiveWords":31,"storageMb":10,"contactMessageBatches":11,"roomMessageBatches":12,"roomTagPulls":13,"workRoomAutoPulls":14,"workFissions":15,"officialAccounts":16,"asyncExecutions":18}',
    updated_at = NOW()
WHERE tenant_id = $TENANT_ID;

UPDATE mochat_go_saas_usage_counters
SET used_value = 0,
    limit_value = 99,
    updated_by = 'corrupted',
    updated_at = NOW()
WHERE tenant_id = $TENANT_ID;

DELETE FROM mochat_go_saas_usage_counters
WHERE tenant_id = $TENANT_ID AND metric = 'agents';
SQL

test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND deleted_at IS NULL")" = "25"

"$MAINTENANCE_BIN" \
  -dsn "$DSN" \
  -action refresh-usage \
  -tenant-id "$TENANT_ID" >"$WORK_DIR/refresh.out"

grep -q $'action\trefresh-usage' "$WORK_DIR/refresh.out"
grep -q $'tenant_id\t'"$TENANT_ID" "$WORK_DIR/refresh.out"
grep -q $'tenants_scanned\t1' "$WORK_DIR/refresh.out"
grep -q $'metrics_refreshed\t26' "$WORK_DIR/refresh.out"
grep -q $'refreshed_tenants\t'"$TENANT_ID" "$WORK_DIR/refresh.out"

test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND deleted_at IS NULL")" = "26"
assert_metric "corps" "1" "5"
assert_metric "users" "1" "6"
assert_metric "contacts" "1" "7"
assert_metric "rooms" "1" "8"
assert_metric "agents" "1" "9"
assert_metric "channel_codes" "1" "17"
assert_metric "shop_codes" "1" "19"
assert_metric "radars" "1" "21"
assert_metric "lotteries" "1" "22"
assert_metric "room_infinite_pulls" "1" "23"
assert_metric "room_fissions" "1" "24"
assert_metric "room_clock_ins" "1" "25"
assert_metric "room_qualities" "1" "26"
assert_metric "room_calendars" "1" "27"
assert_metric "room_reminds" "1" "28"
assert_metric "contact_sops" "1" "29"
assert_metric "room_sops" "1" "30"
assert_metric "sensitive_words" "1" "31"
assert_metric "storage_mb" "3" "10"
assert_metric "contact_message_batches" "1" "11"
assert_metric "room_message_batches" "1" "12"
assert_metric "room_tag_pulls" "1" "13"
assert_metric "work_room_auto_pulls" "1" "14"
assert_metric "work_fissions" "1" "15"
assert_metric "official_accounts" "1" "16"
assert_metric "async_executions" "1" "18"

mysql_root mochat_saas_usage_refresh <<SQL
UPDATE mochat_go_saas_usage_counters
SET used_value = 0,
    limit_value = 42,
    updated_by = 'corrupted-all',
    updated_at = NOW()
WHERE tenant_id = $TENANT_ID;
SQL

"$MAINTENANCE_BIN" \
  -dsn "$DSN" \
  -action refresh-usage >"$WORK_DIR/refresh-all.out"

grep -q $'action\trefresh-usage' "$WORK_DIR/refresh-all.out"
grep -q $'tenant_id\t0' "$WORK_DIR/refresh-all.out"
grep -q $'tenants_scanned\t1' "$WORK_DIR/refresh-all.out"
grep -q $'metrics_refreshed\t26' "$WORK_DIR/refresh-all.out"
grep -q $'refreshed_tenants\t'"$TENANT_ID" "$WORK_DIR/refresh-all.out"

assert_metric "corps" "1" "5"
assert_metric "users" "1" "6"
assert_metric "contacts" "1" "7"
assert_metric "rooms" "1" "8"
assert_metric "agents" "1" "9"
assert_metric "channel_codes" "1" "17"
assert_metric "shop_codes" "1" "19"
assert_metric "radars" "1" "21"
assert_metric "lotteries" "1" "22"
assert_metric "room_infinite_pulls" "1" "23"
assert_metric "room_fissions" "1" "24"
assert_metric "room_clock_ins" "1" "25"
assert_metric "room_qualities" "1" "26"
assert_metric "room_calendars" "1" "27"
assert_metric "room_reminds" "1" "28"
assert_metric "contact_sops" "1" "29"
assert_metric "room_sops" "1" "30"
assert_metric "sensitive_words" "1" "31"
assert_metric "storage_mb" "3" "10"
assert_metric "contact_message_batches" "1" "11"
assert_metric "room_message_batches" "1" "12"
assert_metric "room_tag_pulls" "1" "13"
assert_metric "work_room_auto_pulls" "1" "14"
assert_metric "work_fissions" "1" "15"
assert_metric "official_accounts" "1" "16"
assert_metric "async_executions" "1" "18"

echo "saas usage refresh smoke passed"
