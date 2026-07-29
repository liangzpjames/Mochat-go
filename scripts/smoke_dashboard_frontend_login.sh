#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/local/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-front-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18080}"
WECHAT_ADDR="${MOCHAT_WECHAT_ADDR:-127.0.0.1:19084}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13306}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26379}"
FRONT_BASE_URL="${MOCHAT_FRONT_BASE_URL:-http://$GO_ADDR}"
DASHBOARD_DIST="${MOCHAT_DASHBOARD_DIST:-web/apps/dashboard/dist}"
PLAYWRIGHT_NODE_PATH="${PLAYWRIGHT_NODE_PATH:-$(npm root -g 2>/dev/null || true)}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-front-e2e.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
HASH_GO="$WORK_DIR/hash.go"
FAKE_WECHAT="$WORK_DIR/fake_wechat.py"
GO_LOG="$WORK_DIR/go.log"
WECHAT_LOG="$WORK_DIR/wechat.log"
PLAYWRIGHT_RESULT="$WORK_DIR/playwright-result.txt"
CONSOLE_ERRORS="$WORK_DIR/console-errors.txt"
REQUESTS_LOG="$WORK_DIR/requests.txt"
PLAYWRIGHT_SCRIPT="$WORK_DIR/dashboard-e2e.js"
SCREENSHOT_PATH="../output/playwright/dashboard-e2e.png"
SCREENSHOT_PATH_OUT="../output/playwright/dashboard-e2e-screenshot-path.txt"
GO_PID=""
WECHAT_PID=""

export MOCHAT_MYSQL_PORT="$MYSQL_PORT"
export MOCHAT_REDIS_PORT="$REDIS_PORT"

compose() {
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  local rc=$?
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ -n "$WECHAT_PID" ] && kill -0 "$WECHAT_PID" 2>/dev/null; then
    kill "$WECHAT_PID" 2>/dev/null || true
    wait "$WECHAT_PID" 2>/dev/null || true
  fi
  compose down -v --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$WORK_DIR"
  exit "$rc"
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

wait_url() {
  local url="$1"
  local expected="$2"
  local deadline=$((SECONDS + 90))
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
    local health_status
    health_status="$(compose ps --format json "$service" 2>/dev/null | python3 -c 'import json,sys; data=sys.stdin.read().strip(); print(json.loads(data).get("Health", "")) if data else print("")' 2>/dev/null || true)"
    if [ "$health_status" = "healthy" ]; then
      return 0
    fi
    sleep 2
  done
  echo "$service did not become healthy" >&2
  compose ps >&2 || true
  compose logs --tail=120 "$service" >&2 || true
  exit 1
}

if ! NODE_PATH="$PLAYWRIGHT_NODE_PATH${NODE_PATH:+:$NODE_PATH}" node -e 'require("playwright")' >/dev/null 2>&1; then
  echo "Node Playwright module is not available; set PLAYWRIGHT_NODE_PATH or install playwright" >&2
  exit 1
fi
if [ ! -f "$DASHBOARD_DIST/index.html" ]; then
  echo "dashboard dist not found: $DASHBOARD_DIST" >&2
  exit 1
fi

mkdir -p ../output/playwright
assert_port_free "$GO_ADDR"
assert_port_free "$WECHAT_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis
compose exec -T redis redis-cli FLUSHDB >/dev/null

cat >"$HASH_GO" <<'GO'
package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	sum := md5.Sum([]byte("secret123" + "3S6ybWbSy&23fFeq8"))
	hash, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(sum[:])), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(hash))
}
GO
password_hash="$(go run "$HASH_GO")"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_tenant (id, name, status, logo, login_background, url, copyright, created_at, updated_at)
VALUES (1, '前端联调租户', 1, '', '', '', '', NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), status=VALUES(status), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_corp (id, name, wx_corpid, social_code, employee_secret, event_callback, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at)
VALUES (1, '前端联调企业', 'wx-front-corp', '', '', '', '', '', '', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), tenant_id=VALUES(tenant_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_corp (id, name, wx_corpid, social_code, employee_secret, event_callback, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at)
VALUES (2, '前端切换企业', 'wx-front-corp-switch', '', '', '', '', '', '', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), tenant_id=VALUES(tenant_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_user (id, phone, password, name, gender, department, position, login_time, status, tenant_id, created_at, updated_at, isSuperAdmin)
VALUES (1, '13800000000', '$password_hash', '前端联调用户', 1, '迁移组', '管理员', NOW(), 1, 1, NOW(), NOW(), 1)
ON DUPLICATE KEY UPDATE phone=VALUES(phone), password=VALUES(password), name=VALUES(name), status=VALUES(status), tenant_id=VALUES(tenant_id), isSuperAdmin=VALUES(isSuperAdmin), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_employee (id, wx_user_id, corp_id, name, mobile, position, gender, email, avatar, thumb_avatar, telephone, alias, extattr, status, qr_code, external_profile, external_position, address, open_user_id, wx_main_department_id, main_department_id, log_user_id, contact_auth, audit_status, created_at, updated_at)
VALUES (1, 'front-migrate-user', 1, '前端联调员工', '13800000000', '管理员', 1, 'front-migrate@example.com', '', '', '', '', NULL, 1, '', NULL, '管理员', '', '', 0, 0, 1, 2, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), name=VALUES(name), mobile=VALUES(mobile), status=VALUES(status), log_user_id=VALUES(log_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_employee (id, wx_user_id, corp_id, name, mobile, position, gender, email, avatar, thumb_avatar, telephone, alias, extattr, status, qr_code, external_profile, external_position, address, open_user_id, wx_main_department_id, main_department_id, log_user_id, contact_auth, audit_status, created_at, updated_at)
VALUES (2, 'front-switch-user', 2, '前端切换员工', '13800000000', '管理员', 1, 'front-switch@example.com', '', '', '', '', NULL, 1, '', NULL, '管理员', '', '', 0, 0, 1, 2, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), name=VALUES(name), mobile=VALUES(mobile), status=VALUES(status), log_user_id=VALUES(log_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_department (id, wx_department_id, corp_id, name, parent_id, wx_parentid, \`order\`, level, path, created_at, updated_at)
VALUES (900001, 1, 1, '前端联调总部', 0, 0, 100, 1, '#900001#', NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_department_id=VALUES(wx_department_id), corp_id=VALUES(corp_id), name=VALUES(name), parent_id=VALUES(parent_id), wx_parentid=VALUES(wx_parentid), \`order\`=VALUES(\`order\`), level=VALUES(level), path=VALUES(path), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_department (id, wx_department_id, corp_id, name, parent_id, wx_parentid, \`order\`, level, path, created_at, updated_at)
VALUES (900002, 1, 2, '前端切换总部', 0, 0, 100, 1, '#900002#', NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_department_id=VALUES(wx_department_id), corp_id=VALUES(corp_id), name=VALUES(name), parent_id=VALUES(parent_id), wx_parentid=VALUES(wx_parentid), \`order\`=VALUES(\`order\`), level=VALUES(level), path=VALUES(path), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_employee_department (id, employee_id, department_id, is_leader_in_dept, \`order\`, created_at, updated_at)
VALUES (900001, 1, 900001, 0, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE employee_id=VALUES(employee_id), department_id=VALUES(department_id), is_leader_in_dept=VALUES(is_leader_in_dept), \`order\`=VALUES(\`order\`), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_employee_department (id, employee_id, department_id, is_leader_in_dept, \`order\`, created_at, updated_at)
VALUES (900002, 2, 900002, 0, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE employee_id=VALUES(employee_id), department_id=VALUES(department_id), is_leader_in_dept=VALUES(is_leader_in_dept), \`order\`=VALUES(\`order\`), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_corp_day_data (id, corp_id, add_contact_num, add_room_num, add_into_room_num, loss_contact_num, quit_room_num, date, created_at, updated_at)
VALUES
  (900001, 1, 5, 2, 3, 1, 1, CURDATE(), NOW(), NOW()),
  (900002, 1, 4, 1, 2, 0, 0, DATE_SUB(CURDATE(), INTERVAL 1 DAY), NOW(), NOW()),
  (900003, 2, 7, 3, 4, 1, 0, CURDATE(), NOW(), NOW()),
  (900004, 2, 6, 2, 3, 0, 1, DATE_SUB(CURDATE(), INTERVAL 1 DAY), NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), add_contact_num=VALUES(add_contact_num), add_room_num=VALUES(add_room_num), add_into_room_num=VALUES(add_into_room_num), loss_contact_num=VALUES(loss_contact_num), quit_room_num=VALUES(quit_room_num), date=VALUES(date), updated_at=NOW();
INSERT INTO mc_work_update_time (id, corp_id, type, last_update_time, created_at, updated_at)
VALUES (900001, 1, 6, '2026-07-02 12:00:00', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), type=VALUES(type), last_update_time=VALUES(last_update_time), updated_at=NOW();
INSERT INTO mc_work_update_time (id, corp_id, type, last_update_time, created_at, updated_at)
VALUES (900002, 1, 1, '2026-07-02 09:00:00', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), type=VALUES(type), last_update_time=VALUES(last_update_time), updated_at=NOW();
INSERT INTO mc_work_update_time (id, corp_id, type, last_update_time, created_at, updated_at)
VALUES
  (900003, 2, 6, '2026-07-02 12:30:00', NOW(), NOW()),
  (900004, 2, 1, '2026-07-02 09:30:00', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), type=VALUES(type), last_update_time=VALUES(last_update_time), updated_at=NOW();
INSERT INTO mc_work_contact (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, position, corp_name, corp_full_name, external_profile, business_no, created_at, updated_at)
VALUES (910001, 2, 'front-external-910001', '前端联调客户', '前端联调客户昵称', '', 1, 1, 1, 'front-union-910001', '', '', '', NULL, 'FRONT-910001', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), wx_external_userid=VALUES(wx_external_userid), name=VALUES(name), nick_name=VALUES(nick_name), avatar=VALUES(avatar), follow_up_status=VALUES(follow_up_status), type=VALUES(type), gender=VALUES(gender), unionid=VALUES(unionid), business_no=VALUES(business_no), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_contact_employee (id, employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles, add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at)
VALUES (910001, 2, 910001, '前端联调备注', '前端联调描述', '', NULL, 1, 'front-switch-user', 'channelCode-910001', 2, 1, NOW(), NOW(), NOW())
ON DUPLICATE KEY UPDATE employee_id=VALUES(employee_id), contact_id=VALUES(contact_id), remark=VALUES(remark), description=VALUES(description), remark_corp_name=VALUES(remark_corp_name), remark_mobiles=VALUES(remark_mobiles), add_way=VALUES(add_way), oper_userid=VALUES(oper_userid), state=VALUES(state), corp_id=VALUES(corp_id), status=VALUES(status), create_time=VALUES(create_time), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_room (id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, room_group_id, created_at, updated_at)
VALUES (910001, 2, 'front-room-910001', '前端联调客户群', 2, '', 0, NOW(), 500, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), wx_chat_id=VALUES(wx_chat_id), name=VALUES(name), owner_id=VALUES(owner_id), notice=VALUES(notice), status=VALUES(status), create_time=VALUES(create_time), room_max=VALUES(room_max), room_group_id=VALUES(room_group_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_contact_room (id, wx_user_id, contact_id, employee_id, unionid, room_id, join_scene, type, status, join_time, out_time, created_at, updated_at)
VALUES (910001, 'front-external-910001', 910001, 2, '', 910001, 3, 2, 1, NOW(), '', NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_user_id=VALUES(wx_user_id), contact_id=VALUES(contact_id), employee_id=VALUES(employee_id), unionid=VALUES(unionid), room_id=VALUES(room_id), join_scene=VALUES(join_scene), type=VALUES(type), status=VALUES(status), join_time=VALUES(join_time), out_time=VALUES(out_time), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_auto_tag (
  id, type, name, employees, fuzzy_match_keyword, exact_match_keyword,
  tag_rule, tags, on_off, mark_tag_count, tenant_id, corp_id, create_user_id,
  created_at, updated_at
)
VALUES
  (
    918001, 1, '前端联调关键词自动标签',
    JSON_ARRAY(JSON_OBJECT('id', 2, 'name', '前端切换员工', 'wxUserId', 'front-switch-user')),
    JSON_ARRAY('前端报价'), JSON_ARRAY('前端下单'),
    JSON_ARRAY(JSON_OBJECT(
      'time_type', 1,
      'trigger_count', 1,
      'tags', JSON_ARRAY(JSON_OBJECT('tagid', 918001, 'tagname', '前端关键词标签'))
    )),
    JSON_ARRAY('前端关键词标签'), 1, 1, 1, 2, 1, NOW(), NOW()
  ),
  (
    918002, 2, '前端联调入群自动标签',
    JSON_ARRAY(),
    JSON_ARRAY(), JSON_ARRAY(),
    JSON_ARRAY(JSON_OBJECT(
      'rooms', JSON_ARRAY(JSON_OBJECT('id', 910001, 'name', '前端联调客户群')),
      'tags', JSON_ARRAY(JSON_OBJECT('tagid', 918002, 'tagname', '前端入群标签')),
      'is_grade', false,
      'grade', 0
    )),
    JSON_ARRAY('前端入群标签'), 1, 1, 1, 2, 1, NOW(), NOW()
  ),
  (
    918003, 3, '前端联调分时段自动标签',
    JSON_ARRAY(JSON_OBJECT('id', 2, 'name', '前端切换员工', 'wxUserId', 'front-switch-user')),
    JSON_ARRAY(), JSON_ARRAY(),
    JSON_ARRAY(JSON_OBJECT(
      'time_type', 1,
      'start_time', '00:00:00',
      'end_time', '23:59:59',
      'schedule', JSON_ARRAY(),
      'tags', JSON_ARRAY(JSON_OBJECT('tagid', 918003, 'tagname', '前端分时段标签'))
    )),
    JSON_ARRAY('前端分时段标签'), 1, 1, 1, 2, 1, NOW(), NOW()
  )
ON DUPLICATE KEY UPDATE type=VALUES(type), name=VALUES(name), employees=VALUES(employees), fuzzy_match_keyword=VALUES(fuzzy_match_keyword), exact_match_keyword=VALUES(exact_match_keyword), tag_rule=VALUES(tag_rule), tags=VALUES(tags), on_off=VALUES(on_off), mark_tag_count=VALUES(mark_tag_count), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_auto_tag_record (
  id, auto_tag_id, contact_id, tag_rule_id, wx_external_userid, employee_id,
  keyword, contact_room_id, tags, corp_id, trigger_count, status,
  created_at, updated_at
)
VALUES
  (918001, 918001, 910001, 1, 'front-external-910001', 2, '前端报价', 0, JSON_ARRAY(JSON_OBJECT('tagid', 918001, 'tagname', '前端关键词标签')), 2, 1, 1, NOW(), NOW()),
  (918002, 918002, 910001, 1, 'front-external-910001', 2, '', 910001, JSON_ARRAY(JSON_OBJECT('tagid', 918002, 'tagname', '前端入群标签')), 2, 1, 1, NOW(), NOW()),
  (918003, 918003, 910001, 1, 'front-external-910001', 2, '', 0, JSON_ARRAY(JSON_OBJECT('tagid', 918003, 'tagname', '前端分时段标签')), 2, 1, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE auto_tag_id=VALUES(auto_tag_id), contact_id=VALUES(contact_id), tag_rule_id=VALUES(tag_rule_id), wx_external_userid=VALUES(wx_external_userid), employee_id=VALUES(employee_id), keyword=VALUES(keyword), contact_room_id=VALUES(contact_room_id), tags=VALUES(tags), corp_id=VALUES(corp_id), trigger_count=VALUES(trigger_count), status=VALUES(status), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_rbac_role (id, tenant_id, name, remarks, status, operate_id, operate_name, data_permission, created_at, updated_at)
VALUES (910001, 1, '前端联调角色', '用于 dashboard 真实浏览器权限页回归', 1, 1, '前端联调用户', '[{"corpId":2,"permissionType":2}]', NOW(), NOW())
ON DUPLICATE KEY UPDATE tenant_id=VALUES(tenant_id), name=VALUES(name), remarks=VALUES(remarks), status=VALUES(status), operate_id=VALUES(operate_id), operate_name=VALUES(operate_name), data_permission=VALUES(data_permission), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_rbac_role_menu (id, role_id, menu_id, created_at, updated_at)
VALUES
  (910001, 910001, 110, NOW(), NOW()),
  (910002, 910001, 111, NOW(), NOW())
ON DUPLICATE KEY UPDATE role_id=VALUES(role_id), menu_id=VALUES(menu_id), updated_at=NOW();
INSERT INTO mc_rbac_user_role (id, user_id, role_id, created_at, updated_at)
VALUES (910001, 1, 910001, NOW(), NOW())
ON DUPLICATE KEY UPDATE user_id=VALUES(user_id), role_id=VALUES(role_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_contact_field_pivot (id, contact_id, contact_field_id, value, created_at, updated_at)
VALUES (910001, 910001, 2, '前端联调客户姓名扩展', NOW(), NOW())
ON DUPLICATE KEY UPDATE contact_id=VALUES(contact_id), contact_field_id=VALUES(contact_field_id), value=VALUES(value), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_contact_message_batch_send (
  id, corp_id, user_id, user_name, employee_ids, filter_params, filter_params_detail, content,
  send_way, definite_time, send_time, send_employee_total, send_contact_total, send_total,
  not_send_total, received_total, not_received_total, receive_limit_total, not_friend_total,
  send_status, created_at, updated_at
)
VALUES (
  915001, 2, 1, '前端联调用户', '[2]', '{}', '{"employees":[{"id":2,"name":"前端切换员工"}]}',
  '[{"msgType":"text","content":"前端联调客户群发"}]', 1, NOW(), NOW(), 1, 1, 1, 0, 1, 0, 0, 0, 1, NOW(), NOW()
)
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), user_id=VALUES(user_id), user_name=VALUES(user_name), employee_ids=VALUES(employee_ids), filter_params=VALUES(filter_params), filter_params_detail=VALUES(filter_params_detail), content=VALUES(content), send_way=VALUES(send_way), definite_time=VALUES(definite_time), send_time=VALUES(send_time), send_employee_total=VALUES(send_employee_total), send_contact_total=VALUES(send_contact_total), send_total=VALUES(send_total), not_send_total=VALUES(not_send_total), received_total=VALUES(received_total), not_received_total=VALUES(not_received_total), receive_limit_total=VALUES(receive_limit_total), not_friend_total=VALUES(not_friend_total), send_status=VALUES(send_status), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_contact_message_batch_send_employee (id, batch_id, employee_id, wx_user_id, send_contact_total, err_code, err_msg, msg_id, send_time, last_sync_time, status, receive_status, created_at, updated_at)
VALUES (915001, 915001, 2, 'front-switch-user', 1, '0', '', 'front-contact-batch-msg', NOW(), NOW(), 1, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE batch_id=VALUES(batch_id), employee_id=VALUES(employee_id), wx_user_id=VALUES(wx_user_id), send_contact_total=VALUES(send_contact_total), err_code=VALUES(err_code), err_msg=VALUES(err_msg), msg_id=VALUES(msg_id), send_time=VALUES(send_time), last_sync_time=VALUES(last_sync_time), status=VALUES(status), receive_status=VALUES(receive_status), updated_at=NOW();
INSERT INTO mc_contact_message_batch_send_result (id, batch_id, employee_id, contact_id, external_user_id, user_id, status, send_time, created_at, updated_at)
VALUES (915001, 915001, 2, 910001, 'front-external-910001', 'front-switch-user', 1, UNIX_TIMESTAMP(), NOW(), NOW())
ON DUPLICATE KEY UPDATE batch_id=VALUES(batch_id), employee_id=VALUES(employee_id), contact_id=VALUES(contact_id), external_user_id=VALUES(external_user_id), user_id=VALUES(user_id), status=VALUES(status), send_time=VALUES(send_time), updated_at=NOW();
INSERT INTO mc_room_message_batch_send (
  id, corp_id, user_id, user_name, employee_ids, batch_title, content, send_way,
  definite_time, send_time, send_room_total, send_employee_total, send_total,
  not_send_total, received_total, not_received_total, send_status, created_at, updated_at
)
VALUES (
  916001, 2, 1, '前端联调用户', '[2]', '前端联调客户群群发',
  '[{"msgType":"text","content":"前端联调客户群群发内容"}]', 1, NOW(), NOW(), 1, 1, 1, 0, 1, 0, 1, NOW(), NOW()
)
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), user_id=VALUES(user_id), user_name=VALUES(user_name), employee_ids=VALUES(employee_ids), batch_title=VALUES(batch_title), content=VALUES(content), send_way=VALUES(send_way), definite_time=VALUES(definite_time), send_time=VALUES(send_time), send_room_total=VALUES(send_room_total), send_employee_total=VALUES(send_employee_total), send_total=VALUES(send_total), not_send_total=VALUES(not_send_total), received_total=VALUES(received_total), not_received_total=VALUES(not_received_total), send_status=VALUES(send_status), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_message_batch_send_employee (id, batch_id, employee_id, wx_user_id, send_room_total, err_code, err_msg, msg_id, send_time, last_sync_time, status, receive_status, created_at, updated_at)
VALUES (916001, 916001, 2, 'front-switch-user', 1, '0', '', 'front-room-batch-msg', NOW(), NOW(), 1, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE batch_id=VALUES(batch_id), employee_id=VALUES(employee_id), wx_user_id=VALUES(wx_user_id), send_room_total=VALUES(send_room_total), err_code=VALUES(err_code), err_msg=VALUES(err_msg), msg_id=VALUES(msg_id), send_time=VALUES(send_time), last_sync_time=VALUES(last_sync_time), status=VALUES(status), receive_status=VALUES(receive_status), updated_at=NOW();
INSERT INTO mc_room_message_batch_send_result (id, batch_id, employee_id, room_id, room_name, room_employee_num, room_create_time, chat_id, user_id, status, send_time, created_at, updated_at)
VALUES (916001, 916001, 2, 910001, '前端联调客户群', 1, NOW(), 'front-room-910001', 'front-switch-user', 1, UNIX_TIMESTAMP(), NOW(), NOW())
ON DUPLICATE KEY UPDATE batch_id=VALUES(batch_id), employee_id=VALUES(employee_id), room_id=VALUES(room_id), room_name=VALUES(room_name), room_employee_num=VALUES(room_employee_num), room_create_time=VALUES(room_create_time), chat_id=VALUES(chat_id), user_id=VALUES(user_id), status=VALUES(status), send_time=VALUES(send_time), updated_at=NOW();
INSERT INTO mc_room_tag_pull (id, name, employees, choose_contact, guide, rooms, filter_contact, contact_num, wx_tid, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (917001, '前端联调标签建群', '2', '{"employees":[2],"is_all":1}', '前端联调标签建群引导语', '[{"id":910001,"name":"前端联调客户群"}]', 1, 1, '[{"wxUserId":"front-switch-user","tid":"front-room-tag-tid","status":1}]', 1, 2, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), employees=VALUES(employees), choose_contact=VALUES(choose_contact), guide=VALUES(guide), rooms=VALUES(rooms), filter_contact=VALUES(filter_contact), contact_num=VALUES(contact_num), wx_tid=VALUES(wx_tid), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_tag_pull_contact (id, room_tag_pull_id, contact_id, wx_external_userid, contact_name, employee_id, wx_user_id, send_status, is_join_room, room_id, created_at, updated_at)
VALUES (917001, 917001, 910001, 'front-external-910001', '前端联调客户', 2, 'front-switch-user', 1, 1, 910001, NOW(), NOW())
ON DUPLICATE KEY UPDATE room_tag_pull_id=VALUES(room_tag_pull_id), contact_id=VALUES(contact_id), wx_external_userid=VALUES(wx_external_userid), contact_name=VALUES(contact_name), employee_id=VALUES(employee_id), wx_user_id=VALUES(wx_user_id), send_status=VALUES(send_status), is_join_room=VALUES(is_join_room), room_id=VALUES(room_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_fission (
  id, corp_id, active_name, service_employees, auto_pass, auto_add_tag,
  contact_tags, end_time, qr_code_invalid, tasks, new_friend, delete_invalid,
  receive_prize, receive_prize_employees, receive_links, create_user_id,
  created_at, updated_at
)
VALUES (
  985001, 2, '前端联调任务宝',
  JSON_ARRAY(JSON_OBJECT('id', 2, 'employeeId', 2, 'name', '前端切换员工', 'wxUserId', 'front-switch-user')),
  1, 0, JSON_ARRAY(), DATE_FORMAT(DATE_ADD(NOW(), INTERVAL 7 DAY), '%Y-%m-%d %H:%i:%s'),
  7, JSON_ARRAY(JSON_OBJECT('count', 3, 'prizeLink', 'https://example.com/front-prize')),
  1, 1, 0,
  JSON_ARRAY(JSON_OBJECT('id', 2, 'employeeId', 2, 'name', '前端切换员工', 'wxUserId', 'front-switch-user')),
  JSON_ARRAY(), 1, NOW(), NOW()
)
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), active_name=VALUES(active_name), service_employees=VALUES(service_employees), auto_pass=VALUES(auto_pass), auto_add_tag=VALUES(auto_add_tag), contact_tags=VALUES(contact_tags), end_time=VALUES(end_time), qr_code_invalid=VALUES(qr_code_invalid), tasks=VALUES(tasks), new_friend=VALUES(new_friend), delete_invalid=VALUES(delete_invalid), receive_prize=VALUES(receive_prize), receive_prize_employees=VALUES(receive_prize_employees), receive_links=VALUES(receive_links), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_fission_poster (
  id, fission_id, poster_type, cover_pic, wx_cover_pic, foward_text,
  avatar_show, nickname_show, nickname_color, card_corp_image_name,
  card_corp_name, card_corp_logo, qrcode_w, qrcode_h, qrcode_x, qrcode_y,
  qrcode_id, qrcode_url, created_at, updated_at
)
VALUES (
  985001, 985001, 1, '', '', '前端联调任务宝转发话术',
  1, 1, '#17202a', '前端联调企业形象', '前端联调企业',
  '', '82', '82', '120', '300', 'front-work-fission-qrcode', '', NOW(), NOW()
)
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), poster_type=VALUES(poster_type), cover_pic=VALUES(cover_pic), wx_cover_pic=VALUES(wx_cover_pic), foward_text=VALUES(foward_text), avatar_show=VALUES(avatar_show), nickname_show=VALUES(nickname_show), nickname_color=VALUES(nickname_color), card_corp_image_name=VALUES(card_corp_image_name), card_corp_name=VALUES(card_corp_name), card_corp_logo=VALUES(card_corp_logo), qrcode_w=VALUES(qrcode_w), qrcode_h=VALUES(qrcode_h), qrcode_x=VALUES(qrcode_x), qrcode_y=VALUES(qrcode_y), qrcode_id=VALUES(qrcode_id), qrcode_url=VALUES(qrcode_url), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_fission_welcome (id, fission_id, msg_text, link_title, link_desc, link_cover_url, link_wx_url, created_at, updated_at)
VALUES (985001, 985001, '前端联调任务宝欢迎语', '前端联调任务宝链接', '邀请好友添加企业微信', '', '', NOW(), NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), msg_text=VALUES(msg_text), link_title=VALUES(link_title), link_desc=VALUES(link_desc), link_cover_url=VALUES(link_cover_url), link_wx_url=VALUES(link_wx_url), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_fission_push (id, fission_id, push_employee, push_contact, msg_text, msg_complex, msg_complex_type, created_at, updated_at)
VALUES (985001, 985001, 1, 1, '前端联调任务宝推送', JSON_OBJECT('image', ''), 'image', NOW(), NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), push_employee=VALUES(push_employee), push_contact=VALUES(push_contact), msg_text=VALUES(msg_text), msg_complex=VALUES(msg_complex), msg_complex_type=VALUES(msg_complex_type), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_fission_invite (id, fission_id, type, text, link_title, link_desc, link_pic, wx_link_pic, created_at, updated_at)
VALUES (985001, 985001, 1, '前端联调任务宝邀请文案', '前端联调任务宝邀请链接', '邀请好友完成任务', '', '', NOW(), NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), type=VALUES(type), text=VALUES(text), link_title=VALUES(link_title), link_desc=VALUES(link_desc), link_pic=VALUES(link_pic), wx_link_pic=VALUES(wx_link_pic), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_channel_code_group (id, corp_id, name, created_at, updated_at)
VALUES (910001, 2, '前端联调渠道分组', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), name=VALUES(name), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_channel_code (id, corp_id, group_id, name, qrcode_url, wx_config_id, auto_add_friend, tags, type, drainage_employee, welcome_message, created_at, updated_at)
VALUES (910001, 2, 910001, '前端联调渠道码', 'qrcode/front-channel.png', '', 1, '[]', 1, '[]', '[]', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), group_id=VALUES(group_id), name=VALUES(name), qrcode_url=VALUES(qrcode_url), auto_add_friend=VALUES(auto_add_friend), tags=VALUES(tags), type=VALUES(type), drainage_employee=VALUES(drainage_employee), welcome_message=VALUES(welcome_message), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_shop_code (id, name, type, employee, employee_qrcode, qw_code, search_keyword, address, country, province, city, district, lat, lng, status, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (975001, '前端联调门店活码', 1, '[{"id":2,"name":"前端切换员工"}]', '[{"url":"qrcode/front-shop-owner.png"}]', '[{"id":910001,"name":"前端联调客户群"}]', '前端门店', '杭州市西湖区前端路 1 号', '中国', '浙江省', '杭州市', '西湖区', '30.25', '120.12', 1, 1, 2, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), type=VALUES(type), employee=VALUES(employee), employee_qrcode=VALUES(employee_qrcode), qw_code=VALUES(qw_code), search_keyword=VALUES(search_keyword), address=VALUES(address), country=VALUES(country), province=VALUES(province), city=VALUES(city), district=VALUES(district), lat=VALUES(lat), lng=VALUES(lng), status=VALUES(status), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_shop_code_page (id, type, title, show_type, \`default\`, poster, \`autoPass\`, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (975001, 1, '前端联调门店页', 1, '{"guide":"扫码添加店主","description":"前端联调门店说明"}', '', 1, 1, 2, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE type=VALUES(type), title=VALUES(title), show_type=VALUES(show_type), \`default\`=VALUES(\`default\`), poster=VALUES(poster), \`autoPass\`=VALUES(\`autoPass\`), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_shop_code_record (id, type, corp_id, shop_id, created_at, updated_at)
VALUES (975001, 1, 2, 975001, NOW(), NOW())
ON DUPLICATE KEY UPDATE type=VALUES(type), corp_id=VALUES(corp_id), shop_id=VALUES(shop_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_contact_sop (id, corp_id, creator_id, name, setting, employee_ids, state, contact_ids, created_at, updated_at)
VALUES (965001, 2, 1, '前端联调个人 SOP', '[{"name":"前端联调个人规则","content":[{"type":"text","value":"前端联调个人 SOP 提醒"}]}]', '[2]', 1, '[910001]', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), creator_id=VALUES(creator_id), name=VALUES(name), setting=VALUES(setting), employee_ids=VALUES(employee_ids), state=VALUES(state), contact_ids=VALUES(contact_ids), updated_at=NOW();
INSERT INTO mc_room_sop (id, corp_id, creator_id, name, setting, room_ids, state, created_at, updated_at)
VALUES (965002, 2, 1, '前端联调群 SOP', '[{"name":"前端联调群规则","content":[{"type":"text","value":"前端联调群 SOP 提醒"}]}]', '[910001]', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), creator_id=VALUES(creator_id), name=VALUES(name), setting=VALUES(setting), room_ids=VALUES(room_ids), state=VALUES(state), updated_at=NOW();
INSERT INTO mc_sensitive_word_group (id, corp_id, name, created_at, updated_at)
VALUES (920001, 2, '前端联调敏感词分组', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), name=VALUES(name), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_sensitive_word (id, corp_id, group_id, name, status, created_at, updated_at)
VALUES (920001, 2, 920001, '前端敏感词', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), group_id=VALUES(group_id), name=VALUES(name), status=VALUES(status), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_sensitive_words_monitor (id, corp_id, sensitive_word_id, sensitive_word_name, source, trigger_user_id, trigger_name, work_room_id, trigger_scenario, sender, msg_type, send_time, content, conversation_json, created_at, updated_at)
VALUES (920001, 2, 920001, '前端敏感词', 2, 2, '前端切换员工', 910001, '客户群会话', '前端切换员工', 1, NOW(), '{"content":"这条消息包含前端敏感词"}', '[{"sender":"前端切换员工","msgType":1,"isTrigger":1,"msgContent":{"content":"这条消息包含前端敏感词"}}]', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), sensitive_word_id=VALUES(sensitive_word_id), sensitive_word_name=VALUES(sensitive_word_name), source=VALUES(source), trigger_user_id=VALUES(trigger_user_id), trigger_name=VALUES(trigger_name), work_room_id=VALUES(work_room_id), trigger_scenario=VALUES(trigger_scenario), sender=VALUES(sender), msg_type=VALUES(msg_type), send_time=VALUES(send_time), content=VALUES(content), conversation_json=VALUES(conversation_json), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_lottery (id, name, description, type, time_type, start_time, end_time, contact_tags, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (980001, '前端联调抽奖活动', '前端联调抽奖活动说明', 'roulette', 2, DATE_FORMAT(NOW(), '%Y-%m-%d %H:%i:%s'), DATE_FORMAT(DATE_ADD(NOW(), INTERVAL 7 DAY), '%Y-%m-%d %H:%i:%s'), '[]', 1, 2, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), description=VALUES(description), type=VALUES(type), time_type=VALUES(time_type), start_time=VALUES(start_time), end_time=VALUES(end_time), contact_tags=VALUES(contact_tags), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_lottery_prize (id, lottery_id, prize_set, is_show, exchange_set, draw_set, win_set, corp_card, created_at, updated_at)
VALUES (980001, 980001, '[{"name":"前端联调奖品","total":10,"probability":100}]', 1, '{"type":2,"code":"FRONT-LOTTERY"}', '{"daily":1,"total":2}', '{"max":1}', '{}', NOW(), NOW())
ON DUPLICATE KEY UPDATE lottery_id=VALUES(lottery_id), prize_set=VALUES(prize_set), is_show=VALUES(is_show), exchange_set=VALUES(exchange_set), draw_set=VALUES(draw_set), win_set=VALUES(win_set), corp_card=VALUES(corp_card), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_lottery_contact (id, lottery_id, union_id, contact_id, nickname, avatar, employee_ids, city, source, grade, contact_tags, draw_num, win_num, status, write_off, created_at, updated_at)
VALUES (980001, 980001, 'front-union-910001', 910001, '前端联调抽奖客户', '', '[2]', '杭州', '前端联调', 90, '[]', 2, 1, 1, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE lottery_id=VALUES(lottery_id), union_id=VALUES(union_id), contact_id=VALUES(contact_id), nickname=VALUES(nickname), avatar=VALUES(avatar), employee_ids=VALUES(employee_ids), city=VALUES(city), source=VALUES(source), grade=VALUES(grade), contact_tags=VALUES(contact_tags), draw_num=VALUES(draw_num), win_num=VALUES(win_num), status=VALUES(status), write_off=VALUES(write_off), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_lottery_contact_record (id, lottery_id, contact_id, prize_id, prize_name, receive_status, receive_qr, receive_type, receive_code, write_off, created_at, updated_at)
VALUES (980001, 980001, 980001, 980001, '前端联调奖品', 1, '', 2, 'FRONT-LOTTERY', 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE lottery_id=VALUES(lottery_id), contact_id=VALUES(contact_id), prize_id=VALUES(prize_id), prize_name=VALUES(prize_name), receive_status=VALUES(receive_status), receive_qr=VALUES(receive_qr), receive_type=VALUES(receive_type), receive_code=VALUES(receive_code), write_off=VALUES(write_off), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_radar (id, type, title, link, link_title, link_description, link_cover, pdf_name, pdf, article_type, article, employee_card, action_notice, dynamic_notice, contact_tags, tag_status, contact_grade, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (990001, 1, '前端联调互动雷达', '/static/radar/front.html', '前端联调雷达链接', '前端联调雷达说明', '', '', '', 0, '[]', 1, 1, 1, '[]', 0, '[]', 1, 2, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE type=VALUES(type), title=VALUES(title), link=VALUES(link), link_title=VALUES(link_title), link_description=VALUES(link_description), link_cover=VALUES(link_cover), pdf_name=VALUES(pdf_name), pdf=VALUES(pdf), article_type=VALUES(article_type), article=VALUES(article), employee_card=VALUES(employee_card), action_notice=VALUES(action_notice), dynamic_notice=VALUES(dynamic_notice), contact_tags=VALUES(contact_tags), tag_status=VALUES(tag_status), contact_grade=VALUES(contact_grade), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_radar_channel (id, name, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (990001, '前端联调雷达渠道', 1, 2, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_radar_channel_link (id, radar_id, channel_id, link, employee_id, click_num, click_person_num, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (990001, 990001, 990001, '/auth/radar?id=990001&target=front', 2, 3, 1, 1, 2, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE radar_id=VALUES(radar_id), channel_id=VALUES(channel_id), link=VALUES(link), employee_id=VALUES(employee_id), click_num=VALUES(click_num), click_person_num=VALUES(click_person_num), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_radar_record (id, radar_id, channel_id, type, union_id, nickname, avatar, contact_id, employee_id, content, corp_id, created_at, updated_at)
VALUES (990001, 990001, 990001, 1, 'front-union-910001', '前端联调雷达客户', '', 910001, 2, '前端联调雷达点击', 2, NOW(), NOW())
ON DUPLICATE KEY UPDATE radar_id=VALUES(radar_id), channel_id=VALUES(channel_id), type=VALUES(type), union_id=VALUES(union_id), nickname=VALUES(nickname), avatar=VALUES(avatar), contact_id=VALUES(contact_id), employee_id=VALUES(employee_id), content=VALUES(content), corp_id=VALUES(corp_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_fission (id, official_account_id, active_name, end_time, target_count, new_friend, delete_invalid, receive_employees, auto_pass, status, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (985001, 0, '前端联调群裂变', DATE_FORMAT(DATE_ADD(NOW(), INTERVAL 7 DAY), '%Y-%m-%d %H:%i:%s'), 3, 1, 1, '[2]', 1, 1, 1, 2, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE official_account_id=VALUES(official_account_id), active_name=VALUES(active_name), end_time=VALUES(end_time), target_count=VALUES(target_count), new_friend=VALUES(new_friend), delete_invalid=VALUES(delete_invalid), receive_employees=VALUES(receive_employees), auto_pass=VALUES(auto_pass), status=VALUES(status), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_fission_poster (id, fission_id, cover_pic, avatar_show, nickname_show, nickname_color, qrcode_w, qrcode_h, qrcode_x, qrcode_y, created_at, updated_at)
VALUES (985001, 985001, '', 1, 1, '#17202a', '120', '120', '40', '40', NOW(), NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), cover_pic=VALUES(cover_pic), avatar_show=VALUES(avatar_show), nickname_show=VALUES(nickname_show), nickname_color=VALUES(nickname_color), qrcode_w=VALUES(qrcode_w), qrcode_h=VALUES(qrcode_h), qrcode_x=VALUES(qrcode_x), qrcode_y=VALUES(qrcode_y), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_fission_room (id, fission_id, room_qrcode, room_wx_qrcode, room, room_max, created_at, updated_at)
VALUES (985001, 985001, 'qrcode/front-room-fission.png', '', '{"id":910001,"name":"前端联调客户群"}', 200, NOW(), NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), room_qrcode=VALUES(room_qrcode), room_wx_qrcode=VALUES(room_wx_qrcode), room=VALUES(room), room_max=VALUES(room_max), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_fission_welcome (id, fission_id, text, link_title, link_desc, link_pic, link_wx_url, template_id, created_at, updated_at)
VALUES (985001, 985001, '前端联调群裂变欢迎语', '前端联调群裂变', '邀请好友加入客户群', '', '', '', NOW(), NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), text=VALUES(text), link_title=VALUES(link_title), link_desc=VALUES(link_desc), link_pic=VALUES(link_pic), link_wx_url=VALUES(link_wx_url), template_id=VALUES(template_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_fission_invite (id, fission_id, type, employees, choose_contact, text, link_title, link_desc, link_pic, wx_link_pic, created_at, updated_at)
VALUES (985001, 985001, 1, '[2]', '{}', '前端联调邀请好友进群', '前端联调群裂变邀请', '邀请好友加入客户群', '', '', NOW(), NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), type=VALUES(type), employees=VALUES(employees), choose_contact=VALUES(choose_contact), text=VALUES(text), link_title=VALUES(link_title), link_desc=VALUES(link_desc), link_pic=VALUES(link_pic), wx_link_pic=VALUES(wx_link_pic), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_fission_contact (id, fission_id, union_id, nickname, avatar, parent_union_id, level, contact_id, employee, invite_count, loss, status, receive_status, is_new, external_user_id, room_id, join_status, write_off, created_at, updated_at)
VALUES (985001, 985001, 'front-union-910001', '前端联调群裂变客户', '', '0', 1, 910001, '前端切换员工', 3, 0, 1, 0, 1, 'front-external-910001', 910001, 1, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), union_id=VALUES(union_id), nickname=VALUES(nickname), avatar=VALUES(avatar), parent_union_id=VALUES(parent_union_id), level=VALUES(level), contact_id=VALUES(contact_id), employee=VALUES(employee), invite_count=VALUES(invite_count), loss=VALUES(loss), status=VALUES(status), receive_status=VALUES(receive_status), is_new=VALUES(is_new), external_user_id=VALUES(external_user_id), room_id=VALUES(room_id), join_status=VALUES(join_status), write_off=VALUES(write_off), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_quality (id, name, description, rule, rooms, status, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (940001, '前端联调群质检', '前端联调触发群质检规则', '[{"type":"keyword","keyword":"前端质检关键词"}]', '[910001]', 1, 1, 2, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), description=VALUES(description), rule=VALUES(rule), rooms=VALUES(rooms), status=VALUES(status), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_quality_contact (id, quality_id, room_id, room_name, contact_id, external_user_id, nickname, avatar, employee_ids, content, msg_type, trigger_at, status, created_at, updated_at)
VALUES (940001, 940001, 910001, '前端联调客户群', 910001, 'front-external-910001', '前端联调客户', '', '[2]', '这条群消息包含前端质检关键词', 'text', NOW(), 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE quality_id=VALUES(quality_id), room_id=VALUES(room_id), room_name=VALUES(room_name), contact_id=VALUES(contact_id), external_user_id=VALUES(external_user_id), nickname=VALUES(nickname), avatar=VALUES(avatar), employee_ids=VALUES(employee_ids), content=VALUES(content), msg_type=VALUES(msg_type), trigger_at=VALUES(trigger_at), status=VALUES(status), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_calendar (id, name, rooms, on_off, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (950001, '前端联调群日历', '[{"id":910001,"name":"前端联调客户群"}]', 1, 1, 2, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), rooms=VALUES(rooms), on_off=VALUES(on_off), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_calendar_push (id, room_calendar_id, name, day, push_content, on_off, status, created_at, updated_at)
VALUES (950001, '950001', '前端联调群日历推送', DATE_FORMAT(DATE_ADD(NOW(), INTERVAL 1 DAY), '%Y-%m-%d 09:30:00'), '[{"type":"text","content":"前端联调群日历提醒"}]', 1, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE room_calendar_id=VALUES(room_calendar_id), name=VALUES(name), day=VALUES(day), push_content=VALUES(push_content), on_off=VALUES(on_off), status=VALUES(status), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_remind (id, name, rooms, is_qrcode, is_link, is_miniprogram, is_card, is_keyword, keyword, status, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (930001, '前端联调群提醒', '[{"id":910001,"name":"前端联调客户群"}]', 1, 1, 0, 0, 1, '前端提醒关键词', 1, 1, 2, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), rooms=VALUES(rooms), is_qrcode=VALUES(is_qrcode), is_link=VALUES(is_link), is_miniprogram=VALUES(is_miniprogram), is_card=VALUES(is_card), is_keyword=VALUES(is_keyword), keyword=VALUES(keyword), status=VALUES(status), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_remind_record (id, remind_id, message_id, room_id, type, content, keyword, corp_id, created_at, updated_at)
VALUES (930001, 930001, 910001, 910001, 5, '这条群消息包含前端提醒关键词', '前端提醒关键词', 2, NOW(), NOW())
ON DUPLICATE KEY UPDATE remind_id=VALUES(remind_id), message_id=VALUES(message_id), room_id=VALUES(room_id), type=VALUES(type), content=VALUES(content), keyword=VALUES(keyword), corp_id=VALUES(corp_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_infinite (id, name, avatar, title_status, title, describe_status, \`describe\`, logo, qw_code, total_num, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (960001, '前端联调无限拉群', '', 1, '前端联调福利群', 1, '扫码进入前端联调福利群', '', '[{"qrcode":"qrcode/front-room-infinite.png","upper_limit":200,"status":1}]', 12, 1, 2, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), avatar=VALUES(avatar), title_status=VALUES(title_status), title=VALUES(title), describe_status=VALUES(describe_status), \`describe\`=VALUES(\`describe\`), logo=VALUES(logo), qw_code=VALUES(qw_code), total_num=VALUES(total_num), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_clock_in (id, official_account_id, active_name, description, type, start_time, end_time, tasks, employee_qrcode, contact_tags, corp_card_status, corp_card, status, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (970001, 0, '前端联调群打卡', '前端联调群打卡说明', 1, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 7 DAY), '[{"day":1,"name":"连续打卡1天","prize":"前端联调资料包"}]', '', '[]', 0, '{}', 1, 1, 2, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE official_account_id=VALUES(official_account_id), active_name=VALUES(active_name), description=VALUES(description), type=VALUES(type), start_time=VALUES(start_time), end_time=VALUES(end_time), tasks=VALUES(tasks), employee_qrcode=VALUES(employee_qrcode), contact_tags=VALUES(contact_tags), corp_card_status=VALUES(corp_card_status), corp_card=VALUES(corp_card), status=VALUES(status), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_clock_in_contact (id, clock_in_id, union_id, openid, nickname, avatar, city, contact_id, employee_ids, contact_tags, day_count, status, receive_level, write_off, first_clock_at, last_clock_at, created_at, updated_at)
VALUES (970001, 970001, 'front-union-910001', 'front-openid-970001', '前端联调打卡客户', '', '杭州', 910001, '[2]', '[]', 1, 1, 1, 0, NOW(), NOW(), NOW(), NOW())
ON DUPLICATE KEY UPDATE clock_in_id=VALUES(clock_in_id), union_id=VALUES(union_id), openid=VALUES(openid), nickname=VALUES(nickname), avatar=VALUES(avatar), city=VALUES(city), contact_id=VALUES(contact_id), employee_ids=VALUES(employee_ids), contact_tags=VALUES(contact_tags), day_count=VALUES(day_count), status=VALUES(status), receive_level=VALUES(receive_level), write_off=VALUES(write_off), first_clock_at=VALUES(first_clock_at), last_clock_at=VALUES(last_clock_at), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_clock_in_record (id, clock_in_id, contact_id, union_id, day, created_at, updated_at)
VALUES (970001, 970001, 970001, 'front-union-910001', CURDATE(), NOW(), NOW())
ON DUPLICATE KEY UPDATE clock_in_id=VALUES(clock_in_id), contact_id=VALUES(contact_id), union_id=VALUES(union_id), day=VALUES(day), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mochat_go_saas_alerts (alert_key, tenant_id, alert_type, severity, status, metric, period_key, current_value, limit_value, additional_value, occurrence_count, source, message, context_json, first_seen_at, last_seen_at, created_at, updated_at)
VALUES ('front-e2e-saas-alert', 1, 'quota_exceeded', 'warning', 'open', 'async_executions', 'lifetime', 9, 3, 1, 2, 'frontend.e2e', '前端联调 SaaS 告警 9/3', NULL, NOW(), NOW(), NOW(), NOW())
ON DUPLICATE KEY UPDATE tenant_id=VALUES(tenant_id), alert_type=VALUES(alert_type), severity=VALUES(severity), status=VALUES(status), metric=VALUES(metric), period_key=VALUES(period_key), current_value=VALUES(current_value), limit_value=VALUES(limit_value), additional_value=VALUES(additional_value), occurrence_count=VALUES(occurrence_count), source=VALUES(source), message=VALUES(message), context_json=VALUES(context_json), first_seen_at=VALUES(first_seen_at), last_seen_at=VALUES(last_seen_at), resolved_at=NULL, updated_at=NOW(), deleted_at=NULL;
INSERT INTO mochat_go_saas_alert_settings (tenant_id, channel, enabled, webhook_url, webhook_secret, webhook_timeout_seconds, webhook_retry_attempts, webhook_retry_delay_ms, webhook_title_template, webhook_body_template, notification_max_attempts, notification_retry_delay_seconds, created_at, updated_at)
VALUES (1, 'webhook', 1, 'https://example.com/mochat-saas-alert', 'front-e2e-secret', 5, 2, 250, '前端联调告警 {{.Metric}}', '前端联调 SaaS 告警正文 {{.CurrentValue}}/{{.LimitValue}}', 3, 300, NOW(), NOW())
ON DUPLICATE KEY UPDATE enabled=VALUES(enabled), webhook_url=VALUES(webhook_url), webhook_secret=VALUES(webhook_secret), webhook_timeout_seconds=VALUES(webhook_timeout_seconds), webhook_retry_attempts=VALUES(webhook_retry_attempts), webhook_retry_delay_ms=VALUES(webhook_retry_delay_ms), webhook_title_template=VALUES(webhook_title_template), webhook_body_template=VALUES(webhook_body_template), notification_max_attempts=VALUES(notification_max_attempts), notification_retry_delay_seconds=VALUES(notification_retry_delay_seconds), updated_at=NOW(), deleted_at=NULL;
SQL

go build -o "$GO_BIN" ./cmd/mochat-go
mkdir -p "$WORK_DIR/upload/qrcode"
python3 - "$WORK_DIR/upload/qrcode/front-channel.png" <<'PY'
import base64
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
path.write_bytes(base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII="))
PY
cat >"$FAKE_WECHAT" <<'PY'
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

addr = sys.argv[1]
host, port = addr.rsplit(":", 1)
log_path = sys.argv[2]

def append(event):
    with open(log_path, "a", encoding="utf-8") as fh:
        fh.write(json.dumps(event, ensure_ascii=False, sort_keys=True) + "\n")

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok")
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        parsed = urlparse(self.path)
        raw = self.rfile.read(int(self.headers.get("Content-Length", "0") or "0"))
        payload = json.loads(raw.decode("utf-8") or "{}")
        if parsed.path == "/cgi-bin/component/api_component_token":
            append({"path": parsed.path, "payload": payload})
            self.send_json({"errcode": 0, "component_access_token": "front-component-token", "expires_in": 7200})
            return
        if parsed.path == "/cgi-bin/component/api_create_preauthcode":
            append({"path": parsed.path, "query": parse_qs(parsed.query), "payload": payload})
            self.send_json({"errcode": 0, "pre_auth_code": "front-pre-auth-code", "expires_in": 600})
            return
        append({"path": parsed.path, "payload": payload})
        self.send_response(404)
        self.end_headers()

    def send_json(self, payload):
        raw = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def log_message(self, fmt, *args):
        return

ThreadingHTTPServer((host, int(port)), Handler).serve_forever()
PY
python3 "$FAKE_WECHAT" "$WECHAT_ADDR" "$WECHAT_LOG" &
WECHAT_PID="$!"
wait_url "http://$WECHAT_ADDR/health" 200
env \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_DASHBOARD_DIST="$DASHBOARD_DIST" \
  MOCHAT_FILE_STORAGE_ROOT="$WORK_DIR/upload" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET='3S6ybWbSy&23fFeq8' \
  MOCHAT_SIMPLE_JWT_PREFIX=mc_jwt_ \
  MOCHAT_SIDEBAR_JWT_SECRET='Br3LXhp&Ysha1zRDh' \
  MOCHAT_SIDEBAR_JWT_PREFIX=default \
  MOCHAT_WECHAT_API_BASE_URL="http://$WECHAT_ADDR" \
  MOCHAT_WECHAT_OPEN_PLATFORM_APP_ID="front-component-app" \
  MOCHAT_WECHAT_OPEN_PLATFORM_SECRET="front-component-secret" \
  MOCHAT_WECHAT_COMPONENT_VERIFY_TICKET="front-component-ticket" \
  MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200

wait_url "$FRONT_BASE_URL/login" 200

cat >"$PLAYWRIGHT_SCRIPT" <<'JS'
const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");

const [
  frontBaseURL,
  resultPath,
  consoleErrorsPath,
  requestsPath,
  screenshotPath,
  screenshotPathOut,
] = process.argv.slice(2);

(async () => {
  const dashboardResponses = [];
  const consoleErrors = [];
  const localRequestErrors = [];
  const requests = [];
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext();
  const page = await context.newPage();

  page.on("console", msg => {
    if (msg.type() === "error") {
      consoleErrors.push(msg.text());
    }
  });
  page.on("response", resp => {
    const row = `${resp.request().method()} ${resp.url()} => [${resp.status()}]`;
    requests.push(row);
    if (resp.url().startsWith(frontBaseURL) && resp.status() >= 400) {
      localRequestErrors.push(row);
    }
    if (resp.url().includes("/dashboard/")) {
      dashboardResponses.push({
        method: resp.request().method(),
        url: resp.url(),
        status: resp.status()
      });
    }
  });
  page.on("requestfailed", req => {
    const failureText = req.failure()?.errorText || "";
    const row = `${req.method()} ${req.url()} => [failed] ${failureText}`;
    requests.push(row);
    if (req.url().startsWith(frontBaseURL) && failureText !== "net::ERR_ABORTED") {
      localRequestErrors.push(row);
    }
  });

  await page.goto(`${frontBaseURL}/login`, { waitUntil: "domcontentloaded" });
  await page.locator('input[placeholder="手机号"]').fill("13800000000");
  await page.locator('input[placeholder="密码"]').fill("secret123");
  const authRespPromise = page.waitForResponse(
    resp => resp.url().includes("/dashboard/user/auth"),
    { timeout: 15000 }
  ).catch(err => ({ error: String(err) }));
  const permissionRespPromise = page.waitForResponse(
    resp => resp.url().includes("/dashboard/role/permissionByUser"),
    { timeout: 15000 }
  ).catch(err => ({ error: String(err) }));
  const tenantIndexRespPromise = page.waitForResponse(
    resp => resp.url().includes("/dashboard/external/tenantIndex"),
    { timeout: 15000 }
  ).catch(err => ({ error: String(err) }));
  await page.locator("button.login-button").click();
  const authResp = await authRespPromise;
  let permissionResp = await permissionRespPromise;
  let tenantIndexResp = await tenantIndexRespPromise;
  await page.waitForTimeout(3000);
  const bindRespPromise = page.waitForResponse(
    resp => resp.url().includes("/dashboard/corp/bind"),
    { timeout: 15000 }
  ).catch(err => ({ error: String(err) }));
  const postBindDashboardResponseStart = dashboardResponses.length;
  const postBindLoginShowPromise = page.waitForResponse(
    resp => resp.url().includes("/dashboard/user/loginShow"),
    { timeout: 15000 }
  ).catch(err => ({ error: String(err) }));
  await page.locator(".ant-pro-global-header-index-right .ant-select-selection").click();
  await page.locator(".ant-select-dropdown-menu-item").filter({ hasText: "前端切换企业" }).click();
  const bindResp = await bindRespPromise;
  const postBindLoginShowResp = await postBindLoginShowPromise;
  const selectedCorpVisible = await page.waitForFunction(() => {
    const selected = document.querySelector(".ant-pro-global-header-index-right .ant-select-selection-selected-value");
    return selected && selected.textContent.includes("前端切换企业");
  }, { timeout: 15000 }).then(() => true).catch(() => false);
  const selectedCorp = await page.locator(".ant-pro-global-header-index-right .ant-select-selection-selected-value").innerText().catch(() => "");
  const dashboardToken = await page.evaluate(() => {
    const raw = localStorage.getItem("ACCESS_TOKEN");
    if (!raw) return "";
    try {
      return JSON.parse(raw);
    } catch (_) {
      return raw;
    }
  });
  if (dashboardToken) {
    await page.evaluate(token => localStorage.setItem("mochat_go_saas_alert_token", token), dashboardToken);
  }
  if (typeof permissionResp.status !== "function" && dashboardToken) {
    permissionResp = await context.request.get(`${frontBaseURL}/dashboard/role/permissionByUser`, {
      headers: { Authorization: dashboardToken }
    });
  }
  if (typeof tenantIndexResp.status !== "function") {
    tenantIndexResp = await context.request.get(`${frontBaseURL}/dashboard/external/tenantIndex?domain=mo.chat`);
  }
  const pageVisits = [];

  async function visitBusinessPage(name, routePath, endpointFragments) {
    let endpointResults = endpointFragments.map(fragment => ({ fragment, status: null, error: "" }));
    for (let attempt = 0; attempt < 3; attempt += 1) {
      const responseStart = dashboardResponses.length;
      const waits = endpointFragments.map(fragment => page.waitForResponse(
        resp => resp.url().includes(fragment),
        { timeout: 15000 }
      ).catch(err => ({ error: String(err), fragment })));
      const targetURL = new URL(`${frontBaseURL}${routePath}`);
      if (attempt > 0) {
        targetURL.searchParams.set("__smoke_retry", String(attempt));
      }
      await page.goto(targetURL.toString(), { waitUntil: "domcontentloaded", timeout: 30000 });
      const responses = await Promise.all(waits);
      const recentResponses = dashboardResponses.slice(responseStart);
      endpointResults = responses.map((resp, index) => {
        const fragment = endpointFragments[index];
        if (typeof resp.status === "function") {
          return { fragment, status: resp.status(), error: "" };
        }
        const observed = recentResponses.find(item => item.url.includes(fragment));
        if (observed) {
          return { fragment, status: observed.status, error: "" };
        }
        return { fragment, status: null, error: resp.error || "" };
      });
      if (endpointResults.every(endpoint => endpoint.status === 200)) {
        break;
      }
      if (attempt < 2) {
        await page.goto(`${frontBaseURL}/corpData/index?__smoke_recovery=${attempt + 1}`, {
          waitUntil: "commit",
          timeout: 15000
        }).catch(() => null);
        await page.waitForTimeout(800);
      }
    }
    await page.waitForTimeout(1200);
    pageVisits.push({
      name,
      routePath,
      currentURL: page.url(),
      endpoints: endpointResults
    });
  }

  await visitBusinessPage("system-home", "/corpData/index", [
    "/dashboard/corpData/index",
    "/dashboard/corpData/lineChat"
  ]);
  await visitBusinessPage("corps", "/corp/index", [
    "/dashboard/corp/index"
  ]);
  await visitBusinessPage("users", "/user/index", [
    "/dashboard/user/index"
  ]);
  await visitBusinessPage("password-update", "/passwordUpdate/index", []);
  await page.getByText("旧密码").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("roles", "/role/index", [
    "/dashboard/role/index"
  ]);
  await visitBusinessPage("role-permission", "/role/permissionShow?roleId=910001", [
    "/dashboard/role/permissionShow"
  ]);
  await page.getByText("权限设置").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("menus", "/menu/index", [
    "/dashboard/menu/index"
  ]);
  await visitBusinessPage("departments", "/department/index", [
    "/dashboard/workDepartment/pageIndex"
  ]);
  await visitBusinessPage("employees", "/workEmployee/index", [
    "/dashboard/workEmployee/index"
  ]);
  await visitBusinessPage("contacts", "/workContact/index", [
    "/dashboard/workContact/index"
  ]);
  await visitBusinessPage("contact-field-pivot", "/workContact/contactFieldPivot?contactId=910001&employeeId=2&isContact=1", [
    "/dashboard/workContact/show",
    "/dashboard/workContact/track",
    "/dashboard/contactFieldPivot/index"
  ]);
  await page.getByText("前端联调客户").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("loss-contacts", "/lossContact/index", [
    "/dashboard/workContact/lossContact"
  ]);
  await visitBusinessPage("contact-tags", "/workContactTag/index", [
    "/dashboard/workContactTag/index",
    "/dashboard/workContactTagGroup/index"
  ]);
  await visitBusinessPage("rooms", "/workRoom/index", [
    "/dashboard/workRoom/index"
  ]);
  await visitBusinessPage("work-room-detail", "/workRoom/detail?workRoomId=910001", [
    "/dashboard/workContactRoom/index"
  ]);
  await page.getByText("前端联调客户群").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("work-room-statistics", "/workRoom/statistics?workRoomId=910001", [
    "/dashboard/workRoom/statistics",
    "/dashboard/workRoom/statisticsIndex"
  ]);
  await visitBusinessPage("channel-codes", "/channelCode/index", [
    "/dashboard/channelCode/index"
  ]);
  await visitBusinessPage("channel-code-statistics", "/channelCode/statistics?channelCodeId=910001", [
    "/dashboard/channelCode/statistics",
    "/dashboard/channelCode/statisticsIndex"
  ]);
  await visitBusinessPage("channel-code-store", "/channelCode/store", [
    "/dashboard/channelCodeGroup/index",
    "/dashboard/workContactTagGroup/index",
    "/dashboard/workDepartment/index"
  ]);
  await page.getByText("基础设置").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("mediums", "/mediumGroup/index", [
    "/dashboard/medium/index",
    "/dashboard/mediumGroup/index"
  ]);
  await visitBusinessPage("greetings", "/greeting/index", [
    "/dashboard/greeting/index"
  ]);
  await visitBusinessPage("greeting-store", "/greeting/store", []);
  await page.getByText("创建欢迎语").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("room-welcomes", "/roomWelcome/index", [
    "/dashboard/roomWelcome/index"
  ]);
  await visitBusinessPage("room-welcome-create", "/roomWelcome/create", []);
  await page.getByText("设置入群欢迎语").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("auto-pulls", "/workRoomAutoPull/index", [
    "/dashboard/workRoomAutoPull/index"
  ]);
  await visitBusinessPage("work-room-auto-pull-store", "/workRoomAutoPull/store", [
    "/dashboard/workContactTagGroup/index",
    "/dashboard/workRoomGroup/index"
  ]);
  await page.getByText("扫码名称").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("room-tag-pulls", "/roomTagPull/index", [
    "/dashboard/roomTagPull/index"
  ]);
  await visitBusinessPage("room-tag-pull-create", "/roomTagPull/create", []);
  await page.getByText("任务名称").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("room-tag-pull-detail", "/roomTagPull/detail?id=917001", [
    "/dashboard/roomTagPull/show",
    "/dashboard/roomTagPull/showContact"
  ]);
  await page.getByText("前端联调客户").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("room-tag-pull-contact-detail", "/roomTagPull/contactDetail?id=917001", [
    "/dashboard/roomTagPull/show?id=917001",
    "/dashboard/roomTagPull/showContact?id=917001&type=1",
    "/dashboard/roomTagPull/showContact?id=917001&type=2"
  ]);
  await page.getByText("MoChat Go 标签建群客户明细").waitFor({ timeout: 15000 });
  await page.getByText("前端联调客户").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("auto-tag-keyword", "/autoTag/keywordIndex", [
    "/dashboard/autoTag/index"
  ]);
  await visitBusinessPage("auto-tag-keyword-create", "/autoTag/keywordCreate", []);
  await page.getByText("关键词打标签").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("auto-tag-keyword-show", "/autoTag/keywordShow?idRow=918001", [
    "/dashboard/autoTag/show",
    "/dashboard/autoTag/showContactKeyWord"
  ]);
  await page.getByText("前端关键词标签").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("auto-tag-join-room", "/autoTag/joinRoomIndex", [
    "/dashboard/autoTag/index"
  ]);
  await visitBusinessPage("auto-tag-join-room-create", "/autoTag/joinRoomCreate", []);
  await page.getByText("用户加入群聊").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("auto-tag-join-room-show", "/autoTag/joinRoomShow?idRow=918002", [
    "/dashboard/autoTag/show",
    "/dashboard/autoTag/showContactRoom"
  ]);
  await page.getByText("前端联调入群自动标签").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("auto-tag-day-part", "/autoTag/dayPartIndex", [
    "/dashboard/autoTag/index"
  ]);
  await visitBusinessPage("auto-tag-day-part-create", "/autoTag/dayPartCreate", []);
  await page.getByText("设置打标签规则").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("auto-tag-day-part-show", "/autoTag/dayPartShow?idRow=918003", [
    "/dashboard/autoTag/show",
    "/dashboard/autoTag/showContactTime"
  ]);
  await page.getByText("前端联调分时段自动标签").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("contact-batches", "/contactMessageBatchSend/index", [
    "/dashboard/contactMessageBatchSend/index"
  ]);
  await visitBusinessPage("contact-batch-store", "/contactMessageBatchSend/store", []);
  await page.getByText("选择群发账号").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("contact-batch-detail", "/contactMessageBatchSend/show?batchId=915001", [
    "/dashboard/contactMessageBatchSend/show",
    "/dashboard/contactMessageBatchSend/employeeSendIndex",
    "/dashboard/contactMessageBatchSend/contactReceiveIndex"
  ]);
  await page.getByText("前端联调客户群发").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("room-batches", "/roomMessageBatchSend/index", [
    "/dashboard/roomMessageBatchSend/index"
  ]);
  await visitBusinessPage("room-batch-store", "/roomMessageBatchSend/store", []);
  await page.getByText("群发名称").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("room-batch-detail", "/roomMessageBatchSend/show?batchId=916001", [
    "/dashboard/roomMessageBatchSend/show",
    "/dashboard/roomMessageBatchSend/roomReceiveIndex"
  ]);
  await page.getByText("前端联调客户群群发").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("work-fissions", "/workFission/taskpage", [
    "/dashboard/workFission/index"
  ]);
  await visitBusinessPage("work-fission-create", "/workFission/create", []);
  await page.getByText("活动基本信息").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("work-fission-edit", "/workFission/edit?id=985001", [
    "/dashboard/workFission/info"
  ]);
  await page.waitForFunction(() => {
    return Array.from(document.querySelectorAll("input")).some(input => input.value === "前端联调任务宝");
  }, { timeout: 15000 });
  await visitBusinessPage("work-fission-invite", "/workFission/invite?id=985001", []);
  await page.getByText("邀请信息").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("work-fission-data", "/workFission/dataShow?id=985001", [
    "/dashboard/workFission/index"
  ]);
  await visitBusinessPage("official-accounts", "/officialAccount/index", [
    "/dashboard/officialAccount/index"
  ]);
  await visitBusinessPage("official-account-create", "/officialAccount/create", [
    "/dashboard/officialAccount/getPreAuthUrl"
  ]);
  await page.getByText("授权公众号").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("contact-fields", "/contactField/index", [
    "/dashboard/contactField/index"
  ]);
  await visitBusinessPage("chat-tool-customer", "/chatTool/customer", [
    "/dashboard/chatTool/config"
  ]);
  await visitBusinessPage("chat-tool-enhance", "/chatTool/enhance", [
    "/dashboard/chatTool/config"
  ]);
  await visitBusinessPage("statistics-contact", "/statistics/contact", [
    "/dashboard/statistic/index"
  ]);
  await visitBusinessPage("statistics-employee", "/statistics/employee", [
    "/dashboard/statistic/employees",
    "/dashboard/statistic/employeesTrend",
    "/dashboard/statistic/employeeCounts",
    "/dashboard/statistic/topList",
    "/dashboard/workDepartment/index"
  ]);
  await visitBusinessPage("sensitive-words-console", "/dashboard/sensitiveWords/page", [
    "/dashboard/sensitiveWordGroup/select",
    "/dashboard/sensitiveWord/index",
    "/dashboard/sensitiveWordsMonitor/index"
  ]);
  await page.getByText("MoChat Go 敏感词管理").waitFor({ timeout: 30000 });
  await page.getByText("前端敏感词").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("lottery-console", "/dashboard/lottery/page", [
    "/dashboard/lottery/index",
    "/dashboard/lottery/show?id=980001",
    "/dashboard/lottery/share?id=980001",
    "/dashboard/lottery/showContact"
  ]);
  await page.getByText("MoChat Go 抽奖活动").waitFor({ timeout: 30000 });
  await page.getByText("前端联调抽奖活动").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("radar-console", "/dashboard/radar/page", [
    "/dashboard/radar/index",
    "/dashboard/radar/show?id=990001",
    "/dashboard/radar/indexChannel",
    "/dashboard/radar/indexChannelLink?radarId=990001",
    "/dashboard/radar/showChannel?radarId=990001",
    "/dashboard/radar/showContact?radarId=990001"
  ]);
  await page.getByText("MoChat Go 互动雷达").waitFor({ timeout: 30000 });
  await page.getByText("前端联调互动雷达").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("shop-code-console", "/dashboard/shopCode/page", [
    "/dashboard/shopCode/index",
    "/dashboard/shopCode/info?id=975001",
    "/dashboard/shopCode/share?id=975001",
    "/dashboard/shopCode/location?id=975001",
    "/dashboard/shopCode/pageInfo?type=1",
    "/dashboard/shopCode/show?type=1",
    "/dashboard/shopCode/showContact?shopCodeId=975001",
    "/dashboard/shopCode/showShop?type=1",
    "/dashboard/shopCode/searchCity",
    "/dashboard/shopCode/addressKeyWordList"
  ]);
  await page.getByText("MoChat Go 门店活码").waitFor({ timeout: 30000 });
  await page.getByText("前端联调门店活码").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("contact-sop-console", "/dashboard/contactSop/page", [
    "/dashboard/contactSop/index",
    "/dashboard/contactSop/info?id=965001"
  ]);
  await page.getByText("MoChat Go 个人 SOP").waitFor({ timeout: 30000 });
  await page.getByText("前端联调个人 SOP").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("room-sop-console", "/dashboard/roomSop/page", [
    "/dashboard/roomSop/index",
    "/dashboard/roomSop/info?id=965002"
  ]);
  await page.getByText("MoChat Go 群 SOP").waitFor({ timeout: 30000 });
  await page.getByText("前端联调群 SOP").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("room-fission-console", "/dashboard/roomFission/page", [
    "/dashboard/roomFission/index",
    "/dashboard/roomFission/info?id=985001",
    "/dashboard/roomFission/show?id=985001",
    "/dashboard/roomFission/showRoom?fissionId=985001",
    "/dashboard/roomFission/showContact?fissionId=985001"
  ]);
  await page.getByText("MoChat Go 群裂变").waitFor({ timeout: 30000 });
  await page.getByText("前端联调群裂变").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("room-quality-console", "/dashboard/roomQuality/page", [
    "/dashboard/roomQuality/index",
    "/dashboard/roomQuality/showContact"
  ]);
  await page.getByText("MoChat Go 群质检").waitFor({ timeout: 30000 });
  await page.getByText("前端联调群质检").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("room-calendar-console", "/dashboard/roomCalendar/page", [
    "/dashboard/roomCalendar/index",
    "/dashboard/roomCalendar/show"
  ]);
  await page.getByText("MoChat Go 群日历").waitFor({ timeout: 30000 });
  await page.getByText("前端联调群日历").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("room-remind-console", "/dashboard/roomRemind/page", [
    "/dashboard/roomRemind/index",
    "/dashboard/task/roomRemind"
  ]);
  await page.getByText("MoChat Go 客户群提醒").waitFor({ timeout: 30000 });
  await page.getByText("前端联调群提醒").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("room-infinite-pull-console", "/dashboard/roomInfinitePull/page", [
    "/dashboard/roomInfinitePull/index",
    "/dashboard/roomInfinitePull/info"
  ]);
  await page.getByText("MoChat Go 无限拉群").waitFor({ timeout: 30000 });
  await page.getByText("前端联调无限拉群").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("room-clock-in-console", "/dashboard/roomClockIn/page", [
    "/dashboard/roomClockIn/index",
    "/dashboard/roomClockIn/show",
    "/dashboard/roomClockIn/showContact",
    "/dashboard/roomClockIn/dayDetail"
  ]);
  await page.getByText("MoChat Go 群打卡").waitFor({ timeout: 30000 });
  await page.getByText("前端联调群打卡").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("saas-alert-console", "/dashboard/saasAlert/page", [
    "/dashboard/saasAlert/setting",
    "/dashboard/saasAlert/index"
  ]);
  await page.getByText("MoChat Go SaaS 告警管理").waitFor({ timeout: 30000 });
  await page.getByText("前端联调 SaaS 告警 9/3").first().waitFor({ timeout: 15000 });
  await visitBusinessPage("contact-transfer-resign", "/contactTransfer/resignIndex", [
    "/dashboard/contactTransfer/unassignedList"
  ]);
  await visitBusinessPage("contact-transfer-work", "/contactTransfer/workIndex", [
    "/dashboard/contactTransfer/info"
  ]);
  await visitBusinessPage("contact-transfer-work-logs", "/contactTransfer/workAllotRecord", [
    "/dashboard/contactTransfer/log"
  ]);
  await visitBusinessPage("contact-transfer-logs", "/contactTransfer/resignAllotRecord", [
    "/dashboard/contactTransfer/log"
  ]);

  const postBindLoginShowObserved = dashboardResponses
    .slice(postBindDashboardResponseStart)
    .some(item => item.url.includes("/dashboard/user/loginShow") && item.status === 200);
  const result = {
    url: page.url(),
    token: await page.evaluate(() => {
      const raw = localStorage.getItem("ACCESS_TOKEN");
      if (!raw) return "";
      try {
        return JSON.parse(raw);
      } catch (_) {
        return raw;
      }
    }),
    title: await page.title(),
    authStatus: typeof authResp.status === "function" ? authResp.status() : null,
    authError: authResp.error || "",
    permissionStatus: typeof permissionResp.status === "function" ? permissionResp.status() : null,
    permissionError: permissionResp.error || "",
    tenantIndexStatus: typeof tenantIndexResp.status === "function" ? tenantIndexResp.status() : null,
    tenantIndexURL: typeof tenantIndexResp.url === "function" ? tenantIndexResp.url() : "",
    tenantIndexError: tenantIndexResp.error || "",
    bindStatus: typeof bindResp.status === "function" ? bindResp.status() : null,
    bindError: bindResp.error || "",
    postBindLoginShowStatus: typeof postBindLoginShowResp.status === "function" ? postBindLoginShowResp.status() : null,
    postBindLoginShowError: postBindLoginShowResp.error || "",
    postBindLoginShowObserved,
    selectedCorp,
    selectedCorpVisible,
    pageVisits,
    localRequestErrors,
    forbiddenExternalRequests: requests.filter(row => row.includes("api.mo.chat") || row.includes("oss.mo.chat")),
    dashboardResponses
  };

  fs.writeFileSync(resultPath, JSON.stringify(result) + "\n", "utf8");
  fs.writeFileSync(consoleErrorsPath, consoleErrors.join("\n") + (consoleErrors.length ? "\n" : ""), "utf8");
  fs.writeFileSync(requestsPath, requests.join("\n") + (requests.length ? "\n" : ""), "utf8");
  fs.mkdirSync(path.dirname(screenshotPath), { recursive: true });
  await page.screenshot({ path: screenshotPath, fullPage: true });
  fs.writeFileSync(screenshotPathOut, screenshotPath + "\n", "utf8");
  console.log(JSON.stringify(result));
  await browser.close();
})().catch(async err => {
  console.error(err && err.stack ? err.stack : String(err));
  process.exit(1);
});
JS
NODE_PATH="$PLAYWRIGHT_NODE_PATH${NODE_PATH:+:$NODE_PATH}" node "$PLAYWRIGHT_SCRIPT" "$FRONT_BASE_URL" "$PLAYWRIGHT_RESULT" "$CONSOLE_ERRORS" "$REQUESTS_LOG" "$SCREENSHOT_PATH" "$SCREENSHOT_PATH_OUT"
rg -n "/dashboard/(user/auth|role/permissionByUser|user/loginShow|corp/select|corp/bind)|=> \\[[45]" "$REQUESTS_LOG" || true

python3 - "$PLAYWRIGHT_RESULT" <<'PY'
import json
import re
import sys

text = open(sys.argv[1], encoding="utf-8").read()
result = None
for line in reversed([line.strip() for line in text.splitlines() if line.strip()]):
    try:
        candidate = json.loads(line)
        if isinstance(candidate, str):
            candidate = json.loads(candidate)
        if isinstance(candidate, dict):
            result = candidate
            break
    except json.JSONDecodeError:
        continue
if result is None:
    match = re.search(r"\{.*\}", text, re.S)
    if match:
        result = json.loads(match.group(0))
if result is None:
    raise SystemExit("missing playwright JSON result")
if not result.get("token", "").startswith("Bearer "):
    raise SystemExit("ACCESS_TOKEN was not stored: " + json.dumps(result, ensure_ascii=False))
if "/login" in result.get("url", ""):
    raise SystemExit("still on login page: " + json.dumps(result, ensure_ascii=False))
if result.get("bindStatus") != 200:
    raise SystemExit("corp/bind did not return 200: " + json.dumps(result, ensure_ascii=False))
if result.get("postBindLoginShowStatus") != 200 and not result.get("postBindLoginShowObserved"):
    raise SystemExit("post-bind loginShow did not return 200: " + json.dumps(result, ensure_ascii=False))
if result.get("tenantIndexStatus") != 200 or "/dashboard/external/tenantIndex" not in result.get("tenantIndexURL", ""):
    raise SystemExit("tenantIndex did not use the local Go handler: " + json.dumps(result, ensure_ascii=False))
if result.get("forbiddenExternalRequests"):
    raise SystemExit("dashboard still used legacy MoChat external services: " + json.dumps(result["forbiddenExternalRequests"], ensure_ascii=False))
if result.get("selectedCorp") != "前端切换企业" or not result.get("selectedCorpVisible"):
    raise SystemExit("selected corp did not switch: " + json.dumps(result, ensure_ascii=False))
visits = result.get("pageVisits") or []
expected_visits = {
    "system-home",
    "corps",
    "users",
    "password-update",
    "roles",
    "role-permission",
    "menus",
    "departments",
    "employees",
    "contacts",
    "contact-field-pivot",
    "loss-contacts",
    "contact-tags",
    "rooms",
    "work-room-detail",
    "work-room-statistics",
    "channel-codes",
    "channel-code-statistics",
    "channel-code-store",
    "mediums",
    "greetings",
    "greeting-store",
    "room-welcomes",
    "room-welcome-create",
    "auto-pulls",
    "work-room-auto-pull-store",
    "room-tag-pulls",
    "room-tag-pull-create",
    "room-tag-pull-detail",
    "room-tag-pull-contact-detail",
    "auto-tag-keyword",
    "auto-tag-keyword-create",
    "auto-tag-keyword-show",
    "auto-tag-join-room",
    "auto-tag-join-room-create",
    "auto-tag-join-room-show",
    "auto-tag-day-part",
    "auto-tag-day-part-create",
    "auto-tag-day-part-show",
    "contact-batches",
    "contact-batch-store",
    "contact-batch-detail",
    "room-batches",
    "room-batch-store",
    "room-batch-detail",
    "work-fissions",
    "work-fission-create",
    "work-fission-edit",
    "work-fission-invite",
    "work-fission-data",
    "official-accounts",
    "official-account-create",
    "contact-fields",
    "chat-tool-customer",
    "chat-tool-enhance",
    "statistics-contact",
    "statistics-employee",
    "sensitive-words-console",
    "lottery-console",
    "radar-console",
    "shop-code-console",
    "contact-sop-console",
    "room-sop-console",
    "room-fission-console",
    "room-quality-console",
    "room-calendar-console",
    "room-remind-console",
    "room-infinite-pull-console",
    "room-clock-in-console",
    "saas-alert-console",
    "contact-transfer-resign",
    "contact-transfer-work",
    "contact-transfer-work-logs",
    "contact-transfer-logs",
}
seen_visits = {visit.get("name") for visit in visits}
if seen_visits != expected_visits:
    raise SystemExit("business page visits missing: " + json.dumps(result, ensure_ascii=False))
for visit in visits:
    if "/404" in visit.get("currentURL", ""):
        raise SystemExit("business page rendered 404: " + json.dumps(result, ensure_ascii=False))
    for endpoint in visit.get("endpoints") or []:
        if endpoint.get("status") != 200:
            raise SystemExit("business page endpoint did not return 200: " + json.dumps(result, ensure_ascii=False))
bad_dashboard = [
    item for item in result.get("dashboardResponses", [])
    if int(item.get("status") or 0) >= 400
]
if bad_dashboard:
    raise SystemExit("dashboard frontend request returned error: " + json.dumps(bad_dashboard, ensure_ascii=False))
if result.get("localRequestErrors"):
    raise SystemExit("frontend local request returned error: " + json.dumps(result["localRequestErrors"], ensure_ascii=False))
PY

cache_value="$(compose exec -T redis redis-cli GET mc:user.1)"
if [ "$cache_value" != "2-0" ]; then
  echo "unexpected corp cache value: $cache_value" >&2
  exit 1
fi

echo "dashboard frontend login e2e passed"
