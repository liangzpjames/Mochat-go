#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/local/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-php-check}"
export MOCHAT_PHP_PORT="${MOCHAT_PHP_PORT:-19501}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18080}"
WECOM_ADDR="${MOCHAT_FAKE_WECOM_ADDR:-127.0.0.1:19052}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-real-php.XXXXXX")"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/fake-wecom.log"
GO_BIN="$WORK_DIR/mochat-go"
GO_PID=""
WECOM_PID=""

compose() {
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

compose_up_php_stack() {
  local php_image="${PROJECT_NAME}-php"
  if docker image inspect "$php_image" >/dev/null 2>&1; then
    compose up --no-build -d mysql redis php
  else
    compose up --build -d mysql redis php
  fi
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
  if [ "${KEEP_WORK_DIR:-0}" = "1" ]; then
    echo "keeping work dir: $WORK_DIR" >&2
  else
    rm -rf "$WORK_DIR"
  fi
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

wait_service_healthy() {
  local service="$1"
  local deadline=$((SECONDS + 240))
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
  compose logs --tail=160 "$service" >&2 || true
  exit 1
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
  compose ps >&2 || true
  compose logs --tail=160 php >&2 || true
  exit 1
}

assert_port_free() {
  local addr="$1"
  local port="${addr##*:}"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port $port is already in use" >&2
    exit 1
  fi
}

assert_port_free "$GO_ADDR"
assert_port_free "$WECOM_ADDR"

python3 - "$WECOM_ADDR" "$WORK_DIR/wecom_requests.ndjson" >"$WECOM_LOG" 2>&1 <<'PY' &
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

host, port = sys.argv[1].rsplit(":", 1)
log_path = sys.argv[2]

class Handler(BaseHTTPRequestHandler):
    def _json(self, payload):
        raw = json.dumps(payload, ensure_ascii=False).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def _log(self, body=b""):
        with open(log_path, "a", encoding="utf-8") as f:
            f.write(json.dumps({"method": self.command, "path": self.path, "body": body.decode("utf-8", "ignore")}, ensure_ascii=False) + "\n")

    def do_GET(self):
        self._log()
        if self.path.startswith("/sns/oauth2/component/access_token"):
            self._json({
                "access_token": "official-web-token",
                "expires_in": 7200,
                "refresh_token": "official-refresh-token",
                "openid": "openid-work-fission",
                "scope": "snsapi_userinfo",
                "unionid": "union-fission-parent"
            })
        elif self.path.startswith("/sns/userinfo"):
            self._json({
                "openid": "openid-work-fission",
                "nickname": "裂变微信用户",
                "sex": 1,
                "province": "",
                "city": "",
                "country": "",
                "headimgurl": "https://avatar.example/wechat.png",
                "privilege": [],
                "unionid": "union-fission-parent"
            })
        elif self.path.startswith("/cgi-bin/gettoken"):
            self._json({"errcode": 0, "errmsg": "ok", "access_token": "fake-token", "expires_in": 7200})
        elif self.path.startswith("/cgi-bin/auth/getuserinfo"):
            self._json({"errcode": 0, "errmsg": "ok", "userid": "go-migrate-user"})
        elif self.path.startswith("/cgi-bin/get_jsapi_ticket"):
            self._json({"errcode": 0, "errmsg": "ok", "ticket": "go-corp-jsapi-ticket", "expires_in": 7200})
        elif self.path.startswith("/cgi-bin/ticket/get"):
            self._json({"errcode": 0, "errmsg": "ok", "ticket": "go-agent-jsapi-ticket", "expires_in": 7200})
        elif self.path.startswith("/cgi-bin/agent/get"):
            self._json({
                "errcode": 0,
                "errmsg": "ok",
                "name": "Go迁移企业应用新增",
                "square_logo_url": "https://wecom.example/agent-created-logo.png",
                "description": "Go迁移企业应用描述",
                "close": 0,
                "redirect_domain": "go-agent.example.com",
                "report_location_flag": 1,
                "isreportenter": 1,
                "home_url": "https://go-agent.example.com/home"
            })
        elif self.path.startswith("/cgi-bin/department/list"):
            self._json({
                "errcode": 0,
                "errmsg": "ok",
                "department": [
                    {"id": 1, "name": "Go迁移总部同步", "parentid": 0, "order": 100},
                    {"id": 2, "name": "Go迁移销售部同步", "parentid": 1, "order": 90},
                    {"id": 3, "name": "Go迁移同步新部门", "parentid": 1, "order": 80}
                ]
            })
        elif self.path.startswith("/cgi-bin/user/list"):
            query = parse_qs(urlparse(self.path).query)
            department_id = query.get("department_id", [""])[0]
            users = []
            if department_id in {"1", "2"}:
                users.append({
                    "userid": "go-migrate-user",
                    "name": "Go迁移员工同步更新",
                    "mobile": "13800000000",
                    "position": "同步管理员",
                    "gender": "1",
                    "email": "go-migrate-sync@example.com",
                    "avatar": "https://wecom.example/avatar-updated.png",
                    "thumb_avatar": "https://wecom.example/thumb-updated.png",
                    "telephone": "010-88888888",
                    "alias": "sync-admin",
                    "extattr": {"attrs": []},
                    "status": 4,
                    "qr_code": "https://wecom.example/qr-updated.png",
                    "external_profile": {"external_corp_name": "Go迁移"},
                    "external_position": "同步负责人",
                    "address": "同步地址",
                    "open_userid": "open-go-migrate-user",
                    "main_department": 2,
                    "department": [2],
                    "is_leader_in_dept": [1],
                    "order": [20]
                })
            if department_id in {"1", "3"}:
                users.append({
                    "userid": "go-sync-user",
                    "name": "Go迁移同步新员工",
                    "mobile": "13700000000",
                    "position": "同步顾问",
                    "gender": 2,
                    "email": "go-sync@example.com",
                    "avatar": "https://wecom.example/new-avatar.png",
                    "thumb_avatar": "https://wecom.example/new-thumb.png",
                    "telephone": "",
                    "alias": "sync-new",
                    "extattr": {"attrs": []},
                    "status": 1,
                    "qr_code": "https://wecom.example/new-qr.png",
                    "external_profile": {},
                    "external_position": "同步顾问",
                    "address": "",
                    "open_userid": "open-go-sync-user",
                    "main_department": 3,
                    "department": [3],
                    "is_leader_in_dept": [0],
                    "order": [30]
                })
            self._json({"errcode": 0, "errmsg": "ok", "userlist": users})
        elif self.path.startswith("/cgi-bin/externalcontact/get_follow_user_list"):
            self._json({"errcode": 0, "errmsg": "ok", "follow_user": ["go-sync-user", "go-migrate-user"]})
        elif self.path.startswith("/cgi-bin/user/get"):
            self._json({"errcode": 0, "errmsg": "ok", "userid": "1", "name": "Go迁移通讯录校验"})
        elif self.path.startswith("/cgi-bin/externalcontact/list"):
            query = parse_qs(urlparse(self.path).query)
            userid = query.get("userid", [""])[0]
            if userid == "go-migrate-user":
                self._json({"errcode": 0, "errmsg": "ok", "external_userid": ["external-user-900001", "go-sync-contact"]})
            elif userid == "go-sync-user":
                self._json({"errcode": 84061, "errmsg": "no customer"})
            else:
                self._json({"errcode": 0, "errmsg": "ok", "external_userid": []})
        elif self.path.startswith("/cgi-bin/externalcontact/get"):
            query = parse_qs(urlparse(self.path).query)
            external_userid = query.get("external_userid", [""])[0]
            if external_userid == "external-user-900001":
                self._json({
                    "errcode": 0,
                    "errmsg": "ok",
                    "external_contact": {
                        "external_userid": "external-user-900001",
                        "name": "Go迁移客户同步更新",
                        "avatar": "https://wecom.example/contact-updated.png",
                        "type": 1,
                        "gender": 2,
                        "unionid": "union-go-900001",
                        "position": "采购负责人",
                        "corp_name": "Go客户",
                        "corp_full_name": "Go客户有限公司",
                        "external_profile": {"external_attr": []}
                    },
                    "follow_user": [{
                        "userid": "go-migrate-user",
                        "remark": "Go同步备注",
                        "description": "Go同步描述",
                        "remark_corp_name": "Go同步企业备注",
                        "remark_mobiles": ["13600000000"],
                        "add_way": 3,
                        "oper_userid": "go-migrate-user",
                        "state": "go-sync-state",
                        "createtime": 1710000000,
                        "tags": [{"tag_id": "go-contact-tag-sync", "group_name": "Go迁移同步标签组", "tag_name": "Go迁移同步标签", "type": 1}]
                    }]
                })
            elif external_userid == "go-sync-contact":
                self._json({
                    "errcode": 0,
                    "errmsg": "ok",
                    "external_contact": {
                        "external_userid": "go-sync-contact",
                        "name": "Go迁移同步新客户",
                        "avatar": "https://wecom.example/contact-created.png",
                        "type": 1,
                        "gender": 1,
                        "unionid": "union-go-sync-contact",
                        "position": "老板",
                        "corp_name": "新客户",
                        "corp_full_name": "新客户有限公司",
                        "external_profile": {}
                    },
                    "follow_user": [{
                        "userid": "go-migrate-user",
                        "remark": "Go同步新客户备注",
                        "description": "Go同步新客户描述",
                        "remark_mobiles": ["13500000000"],
                        "add_way": 2,
                        "oper_userid": "go-migrate-user",
                        "state": "go-sync-created",
                        "createtime": 1710000500,
                        "tags": [{"tag_id": "go-contact-tag-sync", "group_name": "Go迁移同步标签组", "tag_name": "Go迁移同步标签", "type": 1}]
                    }]
                })
            else:
                self._json({"errcode": 0, "errmsg": "ok", "external_contact": {"external_userid": "1"}})
        else:
            self._json({"errcode": 0, "errmsg": "ok"})

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0") or "0")
        body = self.rfile.read(length)
        self._log(body)
        if self.path.startswith("/cgi-bin/component/api_component_token"):
            self._json({"errcode": 0, "errmsg": "ok", "component_access_token": "component-token", "expires_in": 7200})
        elif self.path.startswith("/cgi-bin/component/api_create_preauthcode"):
            self._json({"errcode": 0, "errmsg": "ok", "pre_auth_code": "pre-auth-code-go", "expires_in": 600})
        elif self.path.startswith("/cgi-bin/component/api_query_auth"):
            self._json({
                "errcode": 0,
                "errmsg": "ok",
                "authorization_info": {
                    "authorizer_appid": "official-authorizer-93",
                    "authorizer_access_token": "authorizer-access-token-93",
                    "expires_in": 7200,
                    "authorizer_refresh_token": "authorizer-refresh-token-93",
                    "func_info": [{"funcscope_category": {"id": 1}}, {"funcscope_category": {"id": 2}}]
                }
            })
        elif self.path.startswith("/cgi-bin/component/api_authorizer_token"):
            self._json({
                "errcode": 0,
                "errmsg": "ok",
                "authorizer_access_token": "authorizer-access-token-from-refresh",
                "expires_in": 7200,
                "authorizer_refresh_token": "authorizer-refresh-token-93"
            })
        elif self.path.startswith("/cgi-bin/component/api_get_authorizer_info"):
            self._json({
                "errcode": 0,
                "errmsg": "ok",
                "authorizer_info": {
                    "nick_name": "Go迁移授权回跳公众号",
                    "head_img": "https://wechat.example/head-93.png",
                    "service_type_info": {"id": 2},
                    "verify_type_info": {"id": 0},
                    "user_name": "gh_go93",
                    "principal_name": "Go迁移授权主体",
                    "alias": "go-official-93",
                    "business_info": {"open_pay": 1, "open_scan": 1},
                    "qrcode_url": "https://wechat.example/qrcode-93.png"
                }
            })
        elif self.path.startswith("/cgi-bin/message/custom/send"):
            self._json({"errcode": 0, "errmsg": "ok"})
        elif self.path.startswith("/cgi-bin/media/uploadimg"):
            self._json({"errcode": 0, "errmsg": "ok", "url": "https://wecom.example/room-tag-pull-upload.png"})
        elif self.path.startswith("/cgi-bin/media/upload"):
            self._json({"errcode": 0, "errmsg": "ok", "media_id": "go-sidebar-medium-media-id"})
        elif self.path.startswith("/cgi-bin/externalcontact/add_msg_template"):
            self._json({"errcode": 0, "errmsg": "ok", "msgid": "go-room-tag-pull-msgid"})
        elif self.path.startswith("/cgi-bin/externalcontact/get_corp_tag_list"):
            self._json({
                "errcode": 0,
                "errmsg": "ok",
                "tag_group": [
                    {
                        "group_id": "go-tag-group",
                        "group_name": "Go迁移标签组",
                        "order": 1,
                        "tag": [
                            {"id": "go-contact-tag", "name": "Go迁移标签", "order": 1}
                        ]
                    },
                    {
                        "group_id": "go-tag-group-sync",
                        "group_name": "Go迁移同步标签组",
                        "order": 8,
                        "tag": [
                            {"id": "go-contact-tag-sync", "name": "Go迁移同步标签", "order": 9}
                        ]
                    }
                ]
            })
        elif self.path.startswith("/cgi-bin/externalcontact/remark"):
            self._json({"errcode": 0, "errmsg": "ok"})
        elif self.path.startswith("/cgi-bin/externalcontact/mark_tag"):
            self._json({"errcode": 0, "errmsg": "ok"})
        elif self.path.startswith("/cgi-bin/message/send"):
            self._json({"errcode": 0, "errmsg": "ok"})
        elif self.path.startswith("/cgi-bin/externalcontact/add_contact_way"):
            self._json({"errcode": 0, "errmsg": "ok", "config_id": "go-channel-code-config-created", "qr_code": "https://wecom.example/channel-code-created.png"})
        elif self.path.startswith("/cgi-bin/externalcontact/update_contact_way"):
            self._json({"errcode": 0, "errmsg": "ok"})
        elif self.path.startswith("/cgi-bin/externalcontact/contact_way/create"):
            self._json({"errcode": 0, "errmsg": "ok", "config_id": "go-auto-pull-created", "qr_code": "https://wecom.example/auto-pull-created.png"})
        elif self.path.startswith("/cgi-bin/externalcontact/contact_way/update"):
            self._json({"errcode": 0, "errmsg": "ok"})
        elif self.path.startswith("/cgi-bin/externalcontact/get_unassigned_list"):
            self._json({"errcode": 0, "errmsg": "ok", "info": [{"handover_userid": "go-migrate-user", "external_userid": "external-user-900001", "dimission_time": 1710000000}]})
        elif self.path.startswith("/cgi-bin/externalcontact/transfer_customer"):
            self._json({"errcode": 0, "errmsg": "ok"})
        elif self.path.startswith("/cgi-bin/externalcontact/groupchat/list"):
            self._json({
                "errcode": 0,
                "errmsg": "ok",
                "group_chat_list": [
                    {"chat_id": "go-room-900001", "status": 0},
                    {"chat_id": "go-room-sync-created", "status": 0}
                ],
                "next_cursor": ""
            })
        elif self.path.startswith("/cgi-bin/externalcontact/groupchat/get"):
            payload = json.loads(body.decode("utf-8") or "{}")
            chat_id = payload.get("chat_id", "")
            if chat_id == "go-room-900001":
                self._json({
                    "errcode": 0,
                    "errmsg": "ok",
                    "group_chat": {
                        "chat_id": "go-room-900001",
                        "name": "Go迁移客户群同步更新",
                        "owner": "go-migrate-user",
                        "notice": "Go同步群公告",
                        "create_time": 1710001000,
                        "member_list": [
                            {"userid": "go-migrate-user", "type": 1, "join_time": 1710001001, "join_scene": 1},
                            {"userid": "external-user-900001", "type": 2, "join_time": 1710001002, "join_scene": 2, "unionid": "union-go-900001"},
                            {"userid": "go-sync-contact", "type": 2, "join_time": 1710001003, "join_scene": 3, "unionid": "union-go-sync-contact"}
                        ]
                    }
                })
            else:
                self._json({
                    "errcode": 0,
                    "errmsg": "ok",
                    "group_chat": {
                        "chat_id": "go-room-sync-created",
                        "name": "Go迁移同步新客户群",
                        "owner": "go-sync-user",
                        "notice": "Go同步新群公告",
                        "create_time": 1710002000,
                        "member_list": [
                            {"userid": "go-sync-user", "type": 1, "join_time": 1710002001, "join_scene": 1},
                            {"userid": "go-sync-contact", "type": 2, "join_time": 1710002002, "join_scene": 3, "unionid": "union-go-sync-contact"}
                        ]
                    }
                })
        elif self.path.startswith("/cgi-bin/externalcontact/groupchat/transfer"):
            self._json({"errcode": 0, "errmsg": "ok", "failed_chat_list": []})
        else:
            self._json({"errcode": 0, "errmsg": "ok"})

    def log_message(self, format, *args):
        return

ThreadingHTTPServer((host, int(port)), Handler).serve_forever()
PY
WECOM_PID="$!"

wait_url "http://$WECOM_ADDR/cgi-bin/gettoken" 200

compose_up_php_stack
wait_service_healthy mysql
wait_service_healthy redis
wait_url "http://127.0.0.1:$MOCHAT_PHP_PORT/" 200

compose exec -T redis redis-cli FLUSHDB >/dev/null

password_hash="$(compose exec -T php php -r 'echo password_hash(md5("secret123" . getenv("SIMPLE_JWT_SECRET")), PASSWORD_DEFAULT);')"
read -r smoke_today smoke_yesterday smoke_last_month_day <<EOF
$(python3 - <<'PY'
from datetime import date, timedelta

today = date.today()
yesterday = today - timedelta(days=1)
last_month_end = today.replace(day=1) - timedelta(days=1)
last_month_day = last_month_end.replace(day=1) + timedelta(days=1)
print(today.isoformat(), yesterday.isoformat(), last_month_day.isoformat())
PY
)
EOF

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_corp_day_data WHERE id IN (900001, 900002, 900003);
DELETE FROM mc_work_update_time WHERE id IN (900001, 900002, 900003, 900004);
DELETE FROM mc_contact_process WHERE corp_id = 1;
DELETE FROM mc_contact_employee_track WHERE contact_id = 900001 AND content IN ('修改用户资料：备注', '修改用户资料：描述', '修改用户资料：客户编号', '系统对该客户打标签【Go迁移更新标签】');
DELETE FROM mc_work_contact_tag_pivot WHERE contact_id IN (SELECT id FROM mc_work_contact WHERE wx_external_userid = 'go-sync-contact');
DELETE FROM mc_work_contact_tag_pivot WHERE contact_id = 900001 AND employee_id = 1 AND contact_tag_id = 900002;
DELETE FROM mc_work_contact_tag_pivot WHERE contact_id = 900001 AND employee_id = 1 AND contact_tag_id = 900003;
DELETE FROM mc_work_contact_tag_pivot WHERE contact_tag_id IN (900004, 900005);
DELETE FROM mc_work_contact_employee WHERE contact_id IN (SELECT id FROM mc_work_contact WHERE wx_external_userid = 'go-sync-contact');
DELETE FROM mc_work_contact WHERE wx_external_userid = 'go-sync-contact' OR unionid = 'union-fission-new';
DELETE FROM mc_work_contact_tag WHERE id IN (900002, 900003, 900004, 900005) OR wx_contact_tag_id IN ('go-contact-tag-sync', 'go-contact-tag-old') OR name IN ('Go迁移更新标签', 'Go迁移侧边栏标签', 'Go迁移同步标签', 'Go迁移同步旧标签');
DELETE FROM mc_work_contact_tag_group WHERE id IN (900004, 900005) OR wx_group_id IN ('go-tag-group-sync', 'go-tag-group-old') OR group_name IN ('Go迁移同步标签组', 'Go迁移同步旧标签组');
DELETE FROM mc_work_contact_room WHERE id IN (900010, 900011) OR room_id IN (SELECT id FROM mc_work_room WHERE wx_chat_id IN ('go-room-sync-created', 'go-room-sync-old-delete'));
DELETE FROM mc_work_room WHERE id IN (900010) OR wx_chat_id IN ('go-room-sync-created', 'go-room-sync-old-delete');
DELETE FROM mc_contact_message_batch_send_employee WHERE batch_id IN (SELECT id FROM mc_contact_message_batch_send WHERE id IN (900001, 900002) OR content LIKE '%Go迁移客户群发新增%');
DELETE FROM mc_contact_message_batch_send_result WHERE batch_id IN (SELECT id FROM mc_contact_message_batch_send WHERE id IN (900001, 900002) OR content LIKE '%Go迁移客户群发新增%');
DELETE FROM mc_contact_message_batch_send WHERE id IN (900001, 900002) OR content LIKE '%Go迁移客户群发新增%';
DELETE FROM mc_room_message_batch_send_employee WHERE batch_id IN (SELECT id FROM mc_room_message_batch_send WHERE id IN (900001, 900002) OR batch_title = 'Go迁移客户群群发新增');
DELETE FROM mc_room_message_batch_send_result WHERE batch_id IN (SELECT id FROM mc_room_message_batch_send WHERE id IN (900001, 900002) OR batch_title = 'Go迁移客户群群发新增');
DELETE FROM mc_room_message_batch_send WHERE id IN (900001, 900002) OR batch_title = 'Go迁移客户群群发新增';
DELETE FROM mc_official_account_set WHERE id = 900001 OR (corp_id = 1 AND type IN (2, 3) AND official_account_id IN (91, 92));
DELETE FROM mc_official_account WHERE id IN (91, 92) OR authorizer_appid IN ('official-authorizer-93', 'official-authorizer-event') OR nickname IN ('Go迁移公众号', 'Go迁移备用公众号', 'Go迁移授权回跳公众号');
DELETE FROM mc_work_fission_contact WHERE fission_id IN (900001, 900002) OR id IN (900001, 900002, 900003) OR union_id = 'union-fission-new';
DELETE FROM mc_work_fission_invite WHERE fission_id IN (900001, 900002);
DELETE FROM mc_work_fission_poster WHERE fission_id IN (900001, 900002);
DELETE FROM mc_work_fission_push WHERE fission_id IN (900001, 900002);
DELETE FROM mc_work_fission_welcome WHERE fission_id IN (900001, 900002);
DELETE FROM mc_work_fission WHERE id IN (900001, 900002);
DELETE FROM mc_work_employee_department WHERE employee_id IN (SELECT id FROM mc_work_employee WHERE wx_user_id = 'go-sync-user' OR mobile = '13700000000') OR department_id IN (SELECT id FROM mc_work_department WHERE corp_id = 1 AND wx_department_id = 3);
DELETE FROM mc_work_employee WHERE wx_user_id = 'go-sync-user' OR mobile = '13700000000';
DELETE FROM mc_user WHERE phone = '13700000000';
DELETE FROM mc_work_department WHERE corp_id = 1 AND wx_department_id = 3;
DELETE FROM mc_work_room_group WHERE id = 900001 OR name = 'Go迁移客户群分组';
DELETE FROM mc_channel_code_group WHERE name IN ('Go迁移渠道分组新增', 'Go迁移渠道分组更新');
DELETE FROM mc_channel_code WHERE name IN ('Go迁移渠道码新增', 'Go迁移渠道码更新');
DELETE FROM mc_work_agent WHERE wx_agent_id = '1000003' OR name = 'Go迁移企业应用新增';
DELETE FROM mc_corp WHERE id IN (1, 2) OR wx_corpid IN ('wx-test-corp', 'wx-test-forbidden-corp');
ALTER TABLE mc_corp AUTO_INCREMENT = 1;
INSERT INTO mc_tenant (id, name, status, logo, login_background, url, copyright, created_at, updated_at)
VALUES (1, 'Go迁移测试租户', 1, '', '', '', '', NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), status=VALUES(status), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_user (id, phone, password, name, gender, department, position, login_time, status, tenant_id, created_at, updated_at, isSuperAdmin)
VALUES (1, '13800000000', '$password_hash', 'Go迁移测试用户', 1, '迁移组', '管理员', NOW(), 1, 1, NOW(), NOW(), 1)
ON DUPLICATE KEY UPDATE phone=VALUES(phone), password=VALUES(password), name=VALUES(name), status=VALUES(status), tenant_id=VALUES(tenant_id), isSuperAdmin=VALUES(isSuperAdmin), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_user (id, phone, password, name, gender, department, position, login_time, status, tenant_id, created_at, updated_at, isSuperAdmin)
VALUES (2, '13900000000', '$password_hash', 'Go迁移普通用户', 1, '迁移组', '成员', NOW(), 1, 1, NOW(), NOW(), 0)
ON DUPLICATE KEY UPDATE phone=VALUES(phone), password=VALUES(password), name=VALUES(name), status=VALUES(status), tenant_id=VALUES(tenant_id), isSuperAdmin=VALUES(isSuperAdmin), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_rbac_role (id, tenant_id, name, remarks, status, operate_id, operate_name, data_permission, created_at, updated_at)
VALUES (1, 1, 'Go迁移管理员', '', 1, 1, 'Go迁移测试用户', NULL, NOW(), NOW())
ON DUPLICATE KEY UPDATE tenant_id=VALUES(tenant_id), name=VALUES(name), status=VALUES(status), operate_id=VALUES(operate_id), operate_name=VALUES(operate_name), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_rbac_user_role (id, user_id, role_id, created_at, updated_at, deleted_at)
VALUES (900001, 1, 1, NOW(), NOW(), NULL), (900002, 2, 1, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE user_id=VALUES(user_id), role_id=VALUES(role_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_rbac_role_menu (id, role_id, menu_id, created_at, updated_at)
VALUES (900001, 1, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE role_id=VALUES(role_id), menu_id=VALUES(menu_id), updated_at=NOW();
INSERT INTO mc_work_employee (id, wx_user_id, corp_id, name, mobile, position, gender, email, avatar, thumb_avatar, telephone, alias, extattr, status, qr_code, external_profile, external_position, address, open_user_id, wx_main_department_id, main_department_id, log_user_id, contact_auth, audit_status, created_at, updated_at)
VALUES (1, 'go-migrate-user', 1, 'Go迁移员工', '13800000000', '管理员', 1, 'go-migrate@example.com', '', '', '', '', NULL, 1, '', NULL, '管理员', '', '', 0, 0, 1, 2, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), name=VALUES(name), mobile=VALUES(mobile), status=VALUES(status), log_user_id=VALUES(log_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_employee (id, wx_user_id, corp_id, name, mobile, position, gender, email, avatar, thumb_avatar, telephone, alias, extattr, status, qr_code, external_profile, external_position, address, open_user_id, wx_main_department_id, main_department_id, log_user_id, contact_auth, audit_status, created_at, updated_at)
VALUES (2, 'go-normal-user', 1, 'Go迁移普通员工', '13900000000', '成员', 1, 'go-normal@example.com', '', '', '', '', NULL, 1, '', NULL, '成员', '', '', 0, 0, 2, 2, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), name=VALUES(name), mobile=VALUES(mobile), status=VALUES(status), log_user_id=VALUES(log_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_department (id, wx_department_id, corp_id, name, parent_id, wx_parentid, \`order\`, level, path, created_at, updated_at)
VALUES
  (900001, 1, 1, 'Go迁移总部', 0, 0, 100, 1, '#900001#', NOW(), NOW()),
  (900002, 2, 1, 'Go迁移销售部', 900001, 1, 90, 2, '#900001#-#900002#', NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_department_id=VALUES(wx_department_id), corp_id=VALUES(corp_id), name=VALUES(name), parent_id=VALUES(parent_id), wx_parentid=VALUES(wx_parentid), \`order\`=VALUES(\`order\`), level=VALUES(level), path=VALUES(path), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_employee_department (id, employee_id, department_id, is_leader_in_dept, \`order\`, created_at, updated_at)
VALUES
  (900001, 1, 900001, 0, 1, NOW(), NOW()),
  (900002, 2, 900002, 0, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE employee_id=VALUES(employee_id), department_id=VALUES(department_id), is_leader_in_dept=VALUES(is_leader_in_dept), \`order\`=VALUES(\`order\`), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_employee_statistic (id, corp_id, employee_id, new_apply_cnt, new_contact_cnt, chat_cnt, message_cnt, reply_percentage, avg_reply_time, negative_feedback_cnt, syn_time, created_at, updated_at)
VALUES (900001, 1, 1, 2, 3, 8, 5, 75, 10, 1, NOW(), NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), employee_id=VALUES(employee_id), new_apply_cnt=VALUES(new_apply_cnt), new_contact_cnt=VALUES(new_contact_cnt), chat_cnt=VALUES(chat_cnt), message_cnt=VALUES(message_cnt), reply_percentage=VALUES(reply_percentage), avg_reply_time=VALUES(avg_reply_time), negative_feedback_cnt=VALUES(negative_feedback_cnt), syn_time=VALUES(syn_time), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_contact_tag_group (id, wx_group_id, corp_id, group_name, \`order\`, created_at, updated_at)
VALUES (900001, 'go-tag-group', 1, 'Go迁移标签组', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_group_id=VALUES(wx_group_id), corp_id=VALUES(corp_id), group_name=VALUES(group_name), \`order\`=VALUES(\`order\`), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_contact_tag (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at)
VALUES (900001, 'go-contact-tag', 1, 'Go迁移标签', 1, 900001, NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_contact_tag_id=VALUES(wx_contact_tag_id), corp_id=VALUES(corp_id), name=VALUES(name), \`order\`=VALUES(\`order\`), contact_tag_group_id=VALUES(contact_tag_group_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_contact_tag_group (id, wx_group_id, corp_id, group_name, \`order\`, created_at, updated_at)
VALUES (900004, 'go-tag-group-old', 1, 'Go迁移同步旧标签组', 4, NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_group_id=VALUES(wx_group_id), corp_id=VALUES(corp_id), group_name=VALUES(group_name), \`order\`=VALUES(\`order\`), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_contact_tag (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at)
VALUES (900004, 'go-contact-tag-old', 1, 'Go迁移同步旧标签', 4, 900004, NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_contact_tag_id=VALUES(wx_contact_tag_id), corp_id=VALUES(corp_id), name=VALUES(name), \`order\`=VALUES(\`order\`), contact_tag_group_id=VALUES(contact_tag_group_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_channel_code_group (id, corp_id, name, created_at, updated_at)
VALUES (900001, 1, 'Go迁移渠道分组', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), name=VALUES(name), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_channel_code (id, corp_id, group_id, name, qrcode_url, wx_config_id, auto_add_friend, tags, type, drainage_employee, welcome_message, created_at, updated_at)
VALUES (900003, 1, 0, 'Go迁移渠道码', 'qrcode/go-channel.png', '', 1, '[900001]', 1, '[]', '[]', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), group_id=VALUES(group_id), name=VALUES(name), qrcode_url=VALUES(qrcode_url), auto_add_friend=VALUES(auto_add_friend), tags=VALUES(tags), type=VALUES(type), drainage_employee=VALUES(drainage_employee), welcome_message=VALUES(welcome_message), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_official_account (id, app_type, appid, authorized_status, authorizer_appid, head_img, avatar, nickname, service_type_info, verify_type_info, original_id, principal_name, alias, qrcode_url, user_name, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES
  (91, 'official_account', 'component-appid', 1, 'official-authorizer-91', '', 'official/avatar-91.png', 'Go迁移公众号', 2, 0, 'gh_go91', 'Go迁移主体', 'go-official-91', '', 'gh_go91', 0, 1, 1, NOW(), NOW()),
  (92, 'official_account', 'component-appid', 1, 'official-authorizer-92', '', 'official/avatar-92.png', 'Go迁移备用公众号', 2, 0, 'gh_go92', 'Go迁移主体', 'go-official-92', '', 'gh_go92', 0, 1, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE app_type=VALUES(app_type), appid=VALUES(appid), authorized_status=VALUES(authorized_status), authorizer_appid=VALUES(authorizer_appid), avatar=VALUES(avatar), nickname=VALUES(nickname), service_type_info=VALUES(service_type_info), verify_type_info=VALUES(verify_type_info), original_id=VALUES(original_id), principal_name=VALUES(principal_name), alias=VALUES(alias), user_name=VALUES(user_name), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_official_account_set (id, official_account_id, type, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (900001, 91, 2, 0, 1, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE official_account_id=VALUES(official_account_id), type=VALUES(type), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_contact (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, position, corp_name, corp_full_name, external_profile, business_no, created_at, updated_at)
VALUES
  (900001, 1, 'external-user-900001', 'Go迁移客户', 'Go迁移客户昵称', 'avatar/contact.png', 1, 1, 1, 'union-fission-parent', '', '', '', NULL, 'GO-900001', NOW(), NOW()),
  (900002, 1, 'external-user-900002', 'Go迁移客户二', 'Go迁移客户二昵称', '', 1, 1, 0, 'union-fission-child', '', '', '', NULL, 'GO-900002', NOW(), NOW()),
  (900004, 1, 'external-user-fission-new', 'Go迁移新裂变客户', 'Go迁移新裂变昵称', 'avatar/fission-new.png', 1, 1, 1, 'union-fission-new', '', '', '', NULL, 'GO-900004', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), wx_external_userid=VALUES(wx_external_userid), name=VALUES(name), nick_name=VALUES(nick_name), avatar=VALUES(avatar), follow_up_status=VALUES(follow_up_status), type=VALUES(type), gender=VALUES(gender), unionid=VALUES(unionid), business_no=VALUES(business_no), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_contact_employee (id, employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles, add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at)
VALUES
  (900001, 1, 900001, 'Go迁移备注', 'Go迁移描述', '', NULL, 1, 'go-migrate-user', '', 1, 1, NOW(), NOW(), NOW()),
  (900003, 1, 900002, 'Go迁移渠道码备注', 'Go迁移渠道码描述', '', NULL, 1, 'go-migrate-user', 'channelCode-900003', 1, 1, '2026-07-03 10:00:00', NOW(), NOW())
ON DUPLICATE KEY UPDATE employee_id=VALUES(employee_id), contact_id=VALUES(contact_id), remark=VALUES(remark), description=VALUES(description), remark_corp_name=VALUES(remark_corp_name), remark_mobiles=VALUES(remark_mobiles), add_way=VALUES(add_way), oper_userid=VALUES(oper_userid), state=VALUES(state), corp_id=VALUES(corp_id), status=VALUES(status), create_time=VALUES(create_time), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_contact_employee (id, employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles, add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at, deleted_at)
VALUES (900002, 1, 900002, 'Go迁移流失备注', 'Go迁移流失描述', '', NULL, 2, 'go-migrate-user', '', 1, 2, '2026-07-02 08:00:00', NOW(), NOW(), '2026-07-03 10:00:00')
ON DUPLICATE KEY UPDATE employee_id=VALUES(employee_id), contact_id=VALUES(contact_id), remark=VALUES(remark), description=VALUES(description), remark_corp_name=VALUES(remark_corp_name), remark_mobiles=VALUES(remark_mobiles), add_way=VALUES(add_way), oper_userid=VALUES(oper_userid), state=VALUES(state), corp_id=VALUES(corp_id), status=VALUES(status), create_time=VALUES(create_time), updated_at=NOW(), deleted_at=VALUES(deleted_at);
INSERT INTO mc_work_fission (id, corp_id, active_name, service_employees, auto_pass, auto_add_tag, contact_tags, end_time, qr_code_invalid, tasks, new_friend, delete_invalid, receive_prize, receive_prize_employees, receive_links, receive_qrcode, create_user_id, created_at, updated_at)
VALUES
  (900001, 1, 'Go迁移裂变活动', '[{"id":1,"name":"Go迁移员工","wxUserId":"go-migrate-user"}]', 1, 1, '[{"id":900001,"name":"Go迁移标签"}]', '2037-01-01 00:00:00', 7, '[{"level":1,"num":2,"prize":"Go奖励"}]', 1, 0, 1, '[{"id":1,"name":"Go迁移员工"}]', '[{"level":1,"url":"https://example.com/prize"}]', '[]', 1, '2026-07-03 12:00:00', NOW()),
  (900002, 1, 'Go迁移裂变删除', '[{"id":1,"name":"Go迁移员工","wxUserId":"go-migrate-user"}]', 1, 0, '[{"id":900001,"name":"Go迁移标签"}]', '2037-01-01 00:00:00', 7, '[{"level":1,"num":1,"prize":"删除奖励"}]', 1, 0, 0, '[]', '[]', '[]', 1, '2026-07-03 12:10:00', NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), active_name=VALUES(active_name), service_employees=VALUES(service_employees), auto_pass=VALUES(auto_pass), auto_add_tag=VALUES(auto_add_tag), contact_tags=VALUES(contact_tags), end_time=VALUES(end_time), qr_code_invalid=VALUES(qr_code_invalid), tasks=VALUES(tasks), new_friend=VALUES(new_friend), delete_invalid=VALUES(delete_invalid), receive_prize=VALUES(receive_prize), receive_prize_employees=VALUES(receive_prize_employees), receive_links=VALUES(receive_links), receive_qrcode=VALUES(receive_qrcode), create_user_id=VALUES(create_user_id), created_at=VALUES(created_at), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_fission_poster (id, fission_id, poster_type, cover_pic, wx_cover_pic, foward_text, avatar_show, nickname_show, nickname_color, card_corp_image_name, card_corp_name, card_corp_logo, qrcode_w, qrcode_h, qrcode_x, qrcode_y, qrcode_id, qrcode_url, created_at, updated_at)
VALUES
  (900001, 900001, 1, 'work-fission/poster.png', 'wx-poster', '请帮我助力', 1, 1, '#ff0000', 'Go形象', 'Go迁移企业', 'work-fission/logo.png', '120', '120', '10', '20', 'qr-fission-900001', 'https://qr.example/fission.png', NOW(), NOW()),
  (900002, 900002, 1, 'work-fission/delete-poster.png', 'wx-delete-poster', '删除助力', 1, 1, '#00ff00', 'Go形象', 'Go迁移企业', 'work-fission/logo.png', '120', '120', '10', '20', 'qr-fission-900002', 'https://qr.example/fission-delete.png', NOW(), NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), poster_type=VALUES(poster_type), cover_pic=VALUES(cover_pic), wx_cover_pic=VALUES(wx_cover_pic), foward_text=VALUES(foward_text), avatar_show=VALUES(avatar_show), nickname_show=VALUES(nickname_show), nickname_color=VALUES(nickname_color), card_corp_image_name=VALUES(card_corp_image_name), card_corp_name=VALUES(card_corp_name), card_corp_logo=VALUES(card_corp_logo), qrcode_w=VALUES(qrcode_w), qrcode_h=VALUES(qrcode_h), qrcode_x=VALUES(qrcode_x), qrcode_y=VALUES(qrcode_y), qrcode_id=VALUES(qrcode_id), qrcode_url=VALUES(qrcode_url), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_fission_welcome (id, fission_id, msg_text, link_title, link_desc, link_cover_url, link_wx_url, created_at, updated_at)
VALUES
  (900001, 900001, '欢迎参加Go裂变', '欢迎标题', '欢迎描述', 'work-fission/welcome.png', 'wx-welcome', NOW(), NOW()),
  (900002, 900002, '欢迎删除裂变', '删除欢迎标题', '删除欢迎描述', 'work-fission/delete-welcome.png', 'wx-delete-welcome', NOW(), NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), msg_text=VALUES(msg_text), link_title=VALUES(link_title), link_desc=VALUES(link_desc), link_cover_url=VALUES(link_cover_url), link_wx_url=VALUES(link_wx_url), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_fission_push (id, fission_id, push_employee, push_contact, msg_text, msg_complex, msg_complex_type, created_at, updated_at)
VALUES
  (900001, 900001, 1, 1, 'Go裂变推送', '{"image":"work-fission/push.png","title":"推送图"}', 'image', NOW(), NOW()),
  (900002, 900002, 1, 0, '删除裂变推送', '{"image":"work-fission/delete-push.png","title":"删除推送图"}', 'image', NOW(), NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), push_employee=VALUES(push_employee), push_contact=VALUES(push_contact), msg_text=VALUES(msg_text), msg_complex=VALUES(msg_complex), msg_complex_type=VALUES(msg_complex_type), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_fission_invite (id, fission_id, type, text, link_title, link_desc, link_pic, wx_link_pic, created_at, updated_at)
VALUES
  (900001, 900001, 1, '邀请好友助力', '邀请标题', '邀请描述', 'work-fission/invite.png', 'wx-invite', NOW(), NOW()),
  (900002, 900002, 1, '删除邀请好友', '删除邀请标题', '删除邀请描述', 'work-fission/delete-invite.png', 'wx-delete-invite', NOW(), NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), type=VALUES(type), text=VALUES(text), link_title=VALUES(link_title), link_desc=VALUES(link_desc), link_pic=VALUES(link_pic), wx_link_pic=VALUES(wx_link_pic), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_fission_contact (id, fission_id, union_id, nickname, avatar, contact_superior_user_parent, level, employee, invite_count, loss, status, receive_level, is_new, external_user_id, qrcode_id, qrcode_url, created_at, updated_at)
VALUES
  (900001, 900001, 'union-fission-parent', '裂变父客户', 'fission-parent-avatar.png', 0, 1, 'go-migrate-user', 2, 0, 1, 0, 1, 'external-user-900001', 'qr-contact-parent', 'https://qr.example/parent.png', '2026-07-03 12:00:00', NOW()),
  (900002, 900001, 'union-fission-child', '裂变子客户', 'fission-child-avatar.png', 900001, 2, 'go-migrate-user', 0, 1, 0, 0, 1, 'external-user-900002', 'qr-contact-child', 'https://qr.example/child.png', '2026-07-03 12:05:00', NOW()),
  (900003, 900002, 'union-fission-parent', '删除裂变父客户', 'delete-fission-parent.png', 0, 1, 'go-migrate-user', 0, 0, 0, 0, 1, 'external-user-900001', 'qr-contact-delete', 'https://qr.example/delete.png', '2026-07-03 12:10:00', NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), union_id=VALUES(union_id), nickname=VALUES(nickname), avatar=VALUES(avatar), contact_superior_user_parent=VALUES(contact_superior_user_parent), level=VALUES(level), employee=VALUES(employee), invite_count=VALUES(invite_count), loss=VALUES(loss), status=VALUES(status), receive_level=VALUES(receive_level), is_new=VALUES(is_new), external_user_id=VALUES(external_user_id), qrcode_id=VALUES(qrcode_id), qrcode_url=VALUES(qrcode_url), created_at=VALUES(created_at), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_room (id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, room_group_id, created_at, updated_at)
VALUES (900001, 1, 'go-room-900001', 'Go迁移客户群', 1, '', 0, NOW(), 500, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), wx_chat_id=VALUES(wx_chat_id), name=VALUES(name), owner_id=VALUES(owner_id), notice=VALUES(notice), status=VALUES(status), create_time=VALUES(create_time), room_max=VALUES(room_max), room_group_id=VALUES(room_group_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_room_group (id, corp_id, name, created_at, updated_at)
VALUES (900001, 1, 'Go迁移客户群分组', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), name=VALUES(name), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_contact_room (id, wx_user_id, contact_id, employee_id, unionid, room_id, join_scene, type, status, join_time, out_time, created_at, updated_at)
VALUES (900001, 'external-user-900001', 900001, 1, '', 900001, 3, 2, 1, '$smoke_today 08:00:00', '', NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_user_id=VALUES(wx_user_id), contact_id=VALUES(contact_id), employee_id=VALUES(employee_id), unionid=VALUES(unionid), room_id=VALUES(room_id), join_scene=VALUES(join_scene), type=VALUES(type), status=VALUES(status), join_time=VALUES(join_time), out_time=VALUES(out_time), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_room_auto_pull (id, corp_id, qrcode_name, qrcode_url, wx_config_id, is_verified, leading_words, tags, employees, rooms, created_at, updated_at)
VALUES (900001, 1, 'Go迁移自动拉群', 'qrcode/auto-pull.png', 'config-old', 2, '欢迎入群', '[900001]', '[1]', '[{"roomId":900001,"maxNum":50,"roomQrcodeUrl":"qrcode/room-auto-pull.png"}]', '2026-07-03 10:00:00', NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), qrcode_name=VALUES(qrcode_name), qrcode_url=VALUES(qrcode_url), wx_config_id=VALUES(wx_config_id), is_verified=VALUES(is_verified), leading_words=VALUES(leading_words), tags=VALUES(tags), employees=VALUES(employees), rooms=VALUES(rooms), created_at=VALUES(created_at), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_tag_pull (id, name, employees, choose_contact, guide, rooms, filter_contact, contact_num, wx_tid, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES
  (900001, 'Go迁移标签建群', '1,2', '{"is_all":1,"gender":1,"tag_ids":[900001],"start_time":"2026-07-01","end_time":"2026-07-04"}', '请扫码入群', '[{"id":900001,"name":"Go迁移客户群","num":50,"image":"qrcode/room-tag-pull.png","wx_image":"qrcode/room-tag-pull-wx.png"}]', 1, 2, '[{"wxUserId":"go-migrate-user","tid":"go-room-tag-tid-1","status":0},{"wxUserId":"go-normal-user","tid":"go-room-tag-tid-2","status":1}]', 1, 1, 1, '2026-07-03 10:00:00', NOW()),
  (900002, 'Go迁移标签建群删除', '1', '{"is_all":0}', '删除测试', '[{"id":900001,"name":"Go迁移客户群","num":50,"image":"qrcode/room-tag-pull-delete.png","wx_image":"qrcode/room-tag-pull-delete-wx.png"}]', 1, 0, '[]', 1, 1, 1, '2026-07-03 10:00:00', NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), employees=VALUES(employees), choose_contact=VALUES(choose_contact), guide=VALUES(guide), rooms=VALUES(rooms), filter_contact=VALUES(filter_contact), contact_num=VALUES(contact_num), wx_tid=VALUES(wx_tid), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), created_at=VALUES(created_at), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_tag_pull_contact (id, room_tag_pull_id, contact_id, wx_external_userid, contact_name, employee_id, wx_user_id, send_status, is_join_room, room_id, created_at, updated_at)
VALUES
  (900001, 900001, 900001, 'external-user-900001', 'Go迁移客户', 1, 'go-migrate-user', 0, 1, 900001, NOW(), NOW()),
  (900002, 900001, 900002, 'external-user-900002', 'Go迁移客户二', 2, 'go-normal-user', 1, 0, 900001, NOW(), NOW())
ON DUPLICATE KEY UPDATE room_tag_pull_id=VALUES(room_tag_pull_id), contact_id=VALUES(contact_id), wx_external_userid=VALUES(wx_external_userid), contact_name=VALUES(contact_name), employee_id=VALUES(employee_id), wx_user_id=VALUES(wx_user_id), send_status=VALUES(send_status), is_join_room=VALUES(is_join_room), room_id=VALUES(room_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_contact_message_batch_send (
  id, corp_id, user_id, user_name, employee_ids, filter_params, filter_params_detail, content,
  send_way, definite_time, send_time, send_employee_total, send_contact_total, send_total,
  not_send_total, received_total, not_received_total, receive_limit_total, not_friend_total,
  send_status, created_at, updated_at
)
VALUES
  (900001, 1, 1, 'Go迁移测试用户', '[1]', '{"gender":1,"rooms":[900001],"tags":[900001],"addTimeStart":"2026-07-01","addTimeEnd":"2026-07-04"}', '{"gender":1,"addTimeStart":"2026-07-01","addTimeEnd":"2026-07-04","rooms":[{"id":900001,"name":"Go迁移客户群"}],"tags":[{"id":900001,"name":"Go迁移标签"}],"excludeContacts":[]}', '[{"msgType":"text","content":"Go迁移客户群发文本"},{"msgType":"image","pic_url":"medium/go-media.png"}]', 1, NULL, '2026-07-03 10:10:00', 1, 1, 1, 0, 1, 0, 0, 0, 1, '2026-07-03 10:00:00', NOW()),
  (900002, 1, 1, 'Go迁移测试用户', '[1]', '{}', '{"rooms":[],"tags":[],"excludeContacts":[]}', '[{"msgType":"text","content":"Go迁移客户群发删除"}]', 2, '2037-01-01 10:00:00', NULL, 1, 1, 0, 1, 0, 1, 0, 0, 0, '2026-07-03 10:20:00', NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), user_id=VALUES(user_id), user_name=VALUES(user_name), employee_ids=VALUES(employee_ids), filter_params=VALUES(filter_params), filter_params_detail=VALUES(filter_params_detail), content=VALUES(content), send_way=VALUES(send_way), definite_time=VALUES(definite_time), send_time=VALUES(send_time), send_employee_total=VALUES(send_employee_total), send_contact_total=VALUES(send_contact_total), send_total=VALUES(send_total), not_send_total=VALUES(not_send_total), received_total=VALUES(received_total), not_received_total=VALUES(not_received_total), receive_limit_total=VALUES(receive_limit_total), not_friend_total=VALUES(not_friend_total), send_status=VALUES(send_status), created_at=VALUES(created_at), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_contact_message_batch_send_employee (id, batch_id, employee_id, wx_user_id, send_contact_total, err_code, err_msg, msg_id, send_time, last_sync_time, status, receive_status, created_at, updated_at)
VALUES
  (900001, 900001, 1, 'go-migrate-user', 1, '0', 'ok', 'go-contact-message-msgid', '2026-07-03 10:10:00', '2026-07-03 10:20:00', 1, 1, NOW(), NOW()),
  (900002, 900002, 1, 'go-migrate-user', 1, '0', '', '', NULL, NULL, 0, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE batch_id=VALUES(batch_id), employee_id=VALUES(employee_id), wx_user_id=VALUES(wx_user_id), send_contact_total=VALUES(send_contact_total), err_code=VALUES(err_code), err_msg=VALUES(err_msg), msg_id=VALUES(msg_id), send_time=VALUES(send_time), last_sync_time=VALUES(last_sync_time), status=VALUES(status), receive_status=VALUES(receive_status), updated_at=NOW();
INSERT INTO mc_contact_message_batch_send_result (id, batch_id, employee_id, contact_id, external_user_id, user_id, status, send_time, created_at, updated_at)
VALUES
  (900001, 900001, 1, 900001, 'external-user-900001', 'go-migrate-user', 1, 1783159200, NOW(), NOW()),
  (900002, 900002, 1, 900001, 'external-user-900001', 'go-migrate-user', 0, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE batch_id=VALUES(batch_id), employee_id=VALUES(employee_id), contact_id=VALUES(contact_id), external_user_id=VALUES(external_user_id), user_id=VALUES(user_id), status=VALUES(status), send_time=VALUES(send_time), updated_at=NOW();
INSERT INTO mc_room_message_batch_send (
  id, corp_id, user_id, user_name, employee_ids, batch_title, content, send_way, definite_time, send_time,
  send_room_total, send_employee_total, send_total, not_send_total, received_total, not_received_total,
  send_status, created_at, updated_at
)
VALUES
  (900001, 1, 1, 'Go迁移测试用户', '[1]', 'Go迁移客户群群发', '[{"msgType":"text","content":"Go迁移客户群群发文本"},{"msgType":"image","pic_url":"medium/go-media.png"}]', 1, NULL, '2026-07-03 11:10:00', 1, 1, 1, 0, 1, 0, 1, '2026-07-03 11:00:00', NOW()),
  (900002, 1, 1, 'Go迁移测试用户', '[1]', 'Go迁移客户群群发删除', '[{"msgType":"text","content":"Go迁移客户群群发删除"}]', 2, '2037-01-01 11:00:00', NULL, 1, 1, 0, 1, 0, 1, 0, '2026-07-03 11:20:00', NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), user_id=VALUES(user_id), user_name=VALUES(user_name), employee_ids=VALUES(employee_ids), batch_title=VALUES(batch_title), content=VALUES(content), send_way=VALUES(send_way), definite_time=VALUES(definite_time), send_time=VALUES(send_time), send_room_total=VALUES(send_room_total), send_employee_total=VALUES(send_employee_total), send_total=VALUES(send_total), not_send_total=VALUES(not_send_total), received_total=VALUES(received_total), not_received_total=VALUES(not_received_total), send_status=VALUES(send_status), created_at=VALUES(created_at), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_room_message_batch_send_employee (id, batch_id, employee_id, wx_user_id, send_room_total, err_code, err_msg, msg_id, send_time, last_sync_time, status, receive_status, created_at, updated_at)
VALUES
  (900001, 900001, 1, 'go-migrate-user', 1, '0', 'ok', 'go-room-message-msgid', '2026-07-03 11:10:00', '2026-07-03 11:20:00', 1, 1, NOW(), NOW()),
  (900002, 900002, 1, 'go-migrate-user', 1, '0', '', '', NULL, NULL, 0, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE batch_id=VALUES(batch_id), employee_id=VALUES(employee_id), wx_user_id=VALUES(wx_user_id), send_room_total=VALUES(send_room_total), err_code=VALUES(err_code), err_msg=VALUES(err_msg), msg_id=VALUES(msg_id), send_time=VALUES(send_time), last_sync_time=VALUES(last_sync_time), status=VALUES(status), receive_status=VALUES(receive_status), updated_at=NOW();
INSERT INTO mc_room_message_batch_send_result (id, batch_id, employee_id, room_id, room_name, room_employee_num, room_create_time, chat_id, user_id, status, send_time, created_at, updated_at)
VALUES
  (900001, 900001, 1, 900001, 'Go迁移客户群', 1, '2026-07-03 08:00:00', 'go-room-900001', 'go-migrate-user', 1, 1783159200, NOW(), NOW()),
  (900002, 900002, 1, 900001, 'Go迁移客户群', 1, '2026-07-03 08:00:00', 'go-room-900001', 'go-migrate-user', 0, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE batch_id=VALUES(batch_id), employee_id=VALUES(employee_id), room_id=VALUES(room_id), room_name=VALUES(room_name), room_employee_num=VALUES(room_employee_num), room_create_time=VALUES(room_create_time), chat_id=VALUES(chat_id), user_id=VALUES(user_id), status=VALUES(status), send_time=VALUES(send_time), updated_at=NOW();
INSERT INTO mc_work_contact_tag_pivot (id, contact_id, employee_id, contact_tag_id, type, created_at, updated_at)
VALUES
  (900001, 900001, 1, 900001, 1, NOW(), NOW()),
  (900002, 900002, 2, 900001, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE contact_id=VALUES(contact_id), employee_id=VALUES(employee_id), contact_tag_id=VALUES(contact_tag_id), type=VALUES(type), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_contact_tag_pivot (id, contact_id, employee_id, contact_tag_id, type, created_at, updated_at, deleted_at)
VALUES (900004, 900002, 1, 900001, 1, NOW(), NOW(), '2026-07-03 10:00:00')
ON DUPLICATE KEY UPDATE contact_id=VALUES(contact_id), employee_id=VALUES(employee_id), contact_tag_id=VALUES(contact_tag_id), type=VALUES(type), updated_at=NOW(), deleted_at=VALUES(deleted_at);
INSERT INTO mc_contact_employee_track (id, employee_id, contact_id, event, content, corp_id, created_at, updated_at)
VALUES
  (900001, 1, 900001, 1, 'Go迁移互动轨迹', 1, '2026-07-03 09:30:00', '2026-07-03 09:30:00'),
  (900002, 1, 900001, 2, 'Go迁移互动轨迹二', 1, '2026-07-03 09:20:00', '2026-07-03 09:20:00')
ON DUPLICATE KEY UPDATE employee_id=VALUES(employee_id), contact_id=VALUES(contact_id), event=VALUES(event), content=VALUES(content), corp_id=VALUES(corp_id), created_at=VALUES(created_at), updated_at=VALUES(updated_at), deleted_at=NULL;
INSERT INTO mc_work_agent (id, corp_id, wx_agent_id, wx_secret, name, square_logo_url, description, close, redirect_domain, report_location_flag, is_reportenter, home_url, created_at, updated_at)
VALUES (1, 1, '1000002', 'agent-secret', 'Go迁移侧边栏', 'https://example.com/logo.png', '', 0, '', 0, 0, '', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), wx_agent_id=VALUES(wx_agent_id), wx_secret=VALUES(wx_secret), name=VALUES(name), square_logo_url=VALUES(square_logo_url), close=VALUES(close), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_medium (id, type, is_sync, content, media_id, last_upload_time, corp_id, medium_group_id, user_id, user_name, created_at, updated_at)
VALUES (900001, 2, 1, '{"imagePath":"medium/go-media.png","imageName":"go-media.png"}', 'old-sidebar-medium-media-id', 0, 1, 0, 1, 'Go迁移测试用户', NOW(), NOW())
ON DUPLICATE KEY UPDATE type=VALUES(type), is_sync=VALUES(is_sync), content=VALUES(content), media_id=VALUES(media_id), last_upload_time=VALUES(last_upload_time), corp_id=VALUES(corp_id), medium_group_id=VALUES(medium_group_id), user_id=VALUES(user_id), user_name=VALUES(user_name), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_corp_day_data (id, corp_id, add_contact_num, add_room_num, add_into_room_num, loss_contact_num, quit_room_num, date, created_at, updated_at)
VALUES
  (900001, 1, 5, 2, 3, 1, 1, '$smoke_today', NOW(), NOW()),
  (900002, 1, 4, 1, 2, 0, 0, '$smoke_yesterday', NOW(), NOW()),
  (900003, 1, 7, 3, 4, 2, 1, '$smoke_last_month_day', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), add_contact_num=VALUES(add_contact_num), add_room_num=VALUES(add_room_num), add_into_room_num=VALUES(add_into_room_num), loss_contact_num=VALUES(loss_contact_num), quit_room_num=VALUES(quit_room_num), date=VALUES(date), updated_at=NOW();
INSERT INTO mc_work_update_time (id, corp_id, type, last_update_time, created_at, updated_at)
VALUES (900001, 1, 6, '2026-07-02 12:00:00', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), type=VALUES(type), last_update_time=VALUES(last_update_time), updated_at=NOW();
INSERT INTO mc_work_update_time (id, corp_id, type, last_update_time, created_at, updated_at)
VALUES (900002, 1, 1, '2026-07-02 09:00:00', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), type=VALUES(type), last_update_time=VALUES(last_update_time), updated_at=NOW();
INSERT INTO mc_work_update_time (id, corp_id, type, last_update_time, created_at, updated_at)
VALUES (900003, 1, 2, '2026-07-02 11:00:00', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), type=VALUES(type), last_update_time=VALUES(last_update_time), updated_at=NOW();
INSERT INTO mc_work_update_time (id, corp_id, type, last_update_time, created_at, updated_at)
VALUES (900004, 1, 3, '2026-07-02 10:00:00', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), type=VALUES(type), last_update_time=VALUES(last_update_time), updated_at=NOW();
INSERT INTO mc_contact_field (id, name, label, type, options, \`order\`, status, is_sys, created_at, updated_at)
VALUES (900001, 'go_migrate_field', 'Go迁移字段', 3, '["选项A", "选项B"]', 99, 1, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), label=VALUES(label), type=VALUES(type), options=VALUES(options), \`order\`=VALUES(\`order\`), status=VALUES(status), is_sys=VALUES(is_sys), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_contact_field (id, name, label, type, options, \`order\`, status, is_sys, created_at, updated_at)
VALUES (900002, 'go_migrate_picture_field', 'Go迁移图片字段', 11, '[]', 98, 1, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), label=VALUES(label), type=VALUES(type), options=VALUES(options), \`order\`=VALUES(\`order\`), status=VALUES(status), is_sys=VALUES(is_sys), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_contact_field (id, name, label, type, options, \`order\`, status, is_sys, created_at, updated_at)
VALUES (900003, 'go_migrate_checkbox_field', 'Go迁移多选字段', 2, '["多选A", "多选B"]', 97, 1, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), label=VALUES(label), type=VALUES(type), options=VALUES(options), \`order\`=VALUES(\`order\`), status=VALUES(status), is_sys=VALUES(is_sys), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_contact_field_pivot (id, contact_id, contact_field_id, value, created_at, updated_at)
VALUES
  (900001, 900001, 900001, '选项A', NOW(), NOW()),
  (900002, 900001, 900002, 'portrait/a.png', NOW(), NOW()),
  (900003, 900001, 900003, '多选A,多选B', NOW(), NOW())
ON DUPLICATE KEY UPDATE contact_id=VALUES(contact_id), contact_field_id=VALUES(contact_field_id), value=VALUES(value), updated_at=NOW(), deleted_at=NULL;
SQL

mkdir -p "$WORK_DIR/upload/medium"
printf '%s' 'go sidebar medium image' >"$WORK_DIR/upload/medium/go-media.png"

go build -o "$GO_BIN" ./cmd/mochat-go

env \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_SOURCE_ROOT=../mochat \
  MOCHAT_COMPAT_MANIFEST=../docs/migration/compat_manifest.json \
  MOCHAT_PHP_UPSTREAM="http://127.0.0.1:$MOCHAT_PHP_PORT" \
  MOCHAT_API_BASE_URL=http://127.0.0.1:9501 \
  MOCHAT_DASHBOARD_BASE_URL=http://127.0.0.1:9501 \
  MOCHAT_SIDEBAR_BASE_URL=http://sidebar.local \
  MOCHAT_FILE_STORAGE_ROOT="$WORK_DIR/upload" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_WECHAT_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_WECHAT_OPEN_PLATFORM_APP_ID="component-appid" \
  MOCHAT_WECHAT_OPEN_PLATFORM_SECRET="component-secret" \
  MOCHAT_WECHAT_OPEN_PLATFORM_TOKEN="component-token-for-callback" \
  MOCHAT_WECHAT_OPEN_PLATFORM_AES_KEY="abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG" \
  MOCHAT_WECHAT_COMPONENT_VERIFY_TICKET="component-ticket" \
  MOCHAT_OPERATION_BASE_URL="http://127.0.0.1:$MOCHAT_PHP_PORT" \
  MOCHAT_MYSQL_DSN='mochat:mochat_pass@tcp(127.0.0.1:13306)/mochat?parseTime=true&loc=Local' \
  MOCHAT_REDIS_ADDR=127.0.0.1:26379 \
  MOCHAT_SIMPLE_JWT_SECRET='3S6ybWbSy&23fFeq8' \
  MOCHAT_SIMPLE_JWT_PREFIX=mc_jwt_ \
  MOCHAT_SIDEBAR_JWT_SECRET='Br3LXhp&Ysha1zRDh' \
  MOCHAT_SIDEBAR_JWT_PREFIX=default \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
  MOCHAT_GO_MIGRATE_LOGOUT=1 \
  MOCHAT_GO_MIGRATE_PERMISSION_BY_USER=1 \
  MOCHAT_GO_MIGRATE_CORP_SELECT=1 \
  MOCHAT_GO_MIGRATE_CORP_BIND=1 \
  MOCHAT_GO_MIGRATE_CORP_INDEX=1 \
  MOCHAT_GO_MIGRATE_CORP_SHOW=1 \
  MOCHAT_GO_MIGRATE_CORP_STORE=1 \
  MOCHAT_GO_MIGRATE_CORP_UPDATE=1 \
  MOCHAT_GO_MIGRATE_WEWORK_CALLBACK=1 \
  MOCHAT_GO_MIGRATE_CHAT_TOOL_CONFIG=1 \
  MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY=1 \
  MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY_UPLOAD=1 \
  MOCHAT_GO_MIGRATE_AGENT_STORE=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_AUTH=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_OAUTH=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_JSSDK_CONFIG=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_WX_JS_SDK_CONFIG=1 \
  MOCHAT_GO_MIGRATE_ROLE_SELECT=1 \
  MOCHAT_GO_MIGRATE_ROLE_INDEX=1 \
  MOCHAT_GO_MIGRATE_ROLE_SHOW=1 \
  MOCHAT_GO_MIGRATE_ROLE_PERMISSION_SHOW=1 \
  MOCHAT_GO_MIGRATE_ROLE_SHOW_EMPLOYEE=1 \
  MOCHAT_GO_MIGRATE_MENU_ICON_INDEX=1 \
  MOCHAT_GO_MIGRATE_MENU_SELECT=1 \
  MOCHAT_GO_MIGRATE_MENU_INDEX=1 \
  MOCHAT_GO_MIGRATE_MENU_SHOW=1 \
  MOCHAT_GO_MIGRATE_CORP_DATA_INDEX=1 \
  MOCHAT_GO_MIGRATE_CORP_DATA_LINE_CHAT=1 \
  MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SEARCH_CONDITION=1 \
  MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SYNC=1 \
  MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_DEPARTMENT_MEMBER_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_SELECT_BY_PHONE=1 \
  MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_PAGE_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_SHOW_EMPLOYEE=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_DETAIL=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TAG_GROUP_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_DETAIL=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_LIST=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_ALL=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_SYNC=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_SYNC=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_LOSS=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_SOURCE=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_SHOW=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_TRACK=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_UPDATE=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_BATCH_LABELING=1 \
  MOCHAT_GO_MIGRATE_WORK_CONTACT_ROOM_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_ROOM_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_SYNC=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_BATCH_UPDATE=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_WORK_ROOM_MANAGE=1 \
  MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_INFO=1 \
  MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_UNASSIGNED_LIST=1 \
  MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_ROOM=1 \
  MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_LOG=1 \
  MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_SAVE_UNASSIGNED_LIST=1 \
  MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_INDEX=1 \
  MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_ROOM_STORE=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_SHOW=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_STORE=1 \
  MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_UPDATE=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_INDEX=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW_CONTACT=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_ROOM_LIST=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_CHOOSE_CONTACT=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_STORE=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_FILTER_CONTACT=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_REMIND_SEND=1 \
  MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_DESTROY=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_INDEX=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW_ROOM=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_EMPLOYEE_SEND_INDEX=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_CONTACT_RECEIVE_INDEX=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_STORE=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_REMIND=1 \
  MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_DESTROY=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_INDEX=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_SHOW=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_ROOM_OWNER_SEND_INDEX=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_ROOM_RECEIVE_INDEX=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_STORE=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_REMIND=1 \
  MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_DESTROY=1 \
  MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_INDEX=1 \
  MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_SET=1 \
  MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_GET_PRE_AUTH_URL=1 \
  MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_AUTH_REDIRECT=1 \
  MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_AUTH_EVENT_CALLBACK=1 \
  MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_MESSAGE_EVENT_CALLBACK=1 \
  MOCHAT_GO_MIGRATE_LOAD=1 \
  MOCHAT_GO_MIGRATE_WORK_FISSION_INDEX=1 \
  MOCHAT_GO_MIGRATE_WORK_FISSION_SHOW=1 \
  MOCHAT_GO_MIGRATE_WORK_FISSION_INFO=1 \
  MOCHAT_GO_MIGRATE_WORK_FISSION_STATISTICS=1 \
  MOCHAT_GO_MIGRATE_WORK_FISSION_CHOOSE_CONTACT=1 \
  MOCHAT_GO_MIGRATE_WORK_FISSION_STORE=1 \
  MOCHAT_GO_MIGRATE_WORK_FISSION_UPDATE=1 \
  MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE=1 \
  MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE_DATA=1 \
  MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE_DETAIL=1 \
  MOCHAT_GO_MIGRATE_WORK_FISSION_DESTROY=1 \
  MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_INVITE_FRIENDS=1 \
  MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_POSTER=1 \
  MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_TASK_DATA=1 \
  MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_RECEIVE=1 \
  MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_AUTH=1 \
  MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_OPEN_USER_INFO=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TAG_ALL=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_DETAIL=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_SHOW=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TRACK=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_UPDATE=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_INDEX=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_UPDATE=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_MEDIUM_MEDIA_ID_UPDATE=1 \
  MOCHAT_GO_MIGRATE_CONTACT_FIELD_INDEX=1 \
  MOCHAT_GO_MIGRATE_CONTACT_FIELD_SHOW=1 \
  MOCHAT_GO_MIGRATE_CONTACT_FIELD_PORTRAIT=1 \
  MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_INDEX=1 \
  MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_UPDATE=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_FIELD_PIVOT_INDEX=1 \
  MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_FIELD_PIVOT_UPDATE=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_INDEX=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_SHOW=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_CONTACT=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_STATISTICS=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_STATISTICS_INDEX=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_STORE=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_UPDATE=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_INDEX=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_DETAIL=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_STORE=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_UPDATE=1 \
  MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_MOVE=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200

curl -sS -f "http://$GO_ADDR/readyz" >"$WORK_DIR/readyz.json"
curl -sS -f -X POST -H 'Content-Type: application/json' -d '{"phone":"13800000000","password":"secret123"}' "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/go_auth.json"
token="$(python3 - "$WORK_DIR/go_auth.json" <<'PY'
import json
import sys
body = json.load(open(sys.argv[1], encoding="utf-8"))
if body.get("code") != 200:
    raise SystemExit(body)
print(body["data"]["token"])
PY
)"

curl -sS -f -X POST -H 'Content-Type: application/json' -d '{"phone":"13900000000","password":"secret123"}' "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/go_normal_auth.json"
normal_token="$(python3 - "$WORK_DIR/go_normal_auth.json" <<'PY'
import json
import sys
body = json.load(open(sys.argv[1], encoding="utf-8"))
if body.get("code") != 200:
    raise SystemExit(body)
print(body["data"]["token"])
PY
)"

curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d '{"corpName":"Go迁移测试企业","wxCorpId":"wx-test-corp","employeeSecret":"employee-secret","contactSecret":"contact-secret"}' "http://$GO_ADDR/dashboard/corp/store" >"$WORK_DIR/go_corp_store.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT id, name, wx_corpid, employee_secret, event_callback, contact_secret, token, encoding_aes_key, tenant_id FROM mc_corp WHERE id = 1 AND deleted_at IS NULL" >"$WORK_DIR/corp_store_created.tsv"
corp_store_user_cache="$(compose exec -T redis redis-cli GET mc:user.1 || true)"
python3 - "$WORK_DIR" "$corp_store_user_cache" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
cache_value = sys.argv[2].strip()
store = json.loads((work / "go_corp_store.json").read_text(encoding="utf-8"))
created = (work / "corp_store_created.tsv").read_text(encoding="utf-8").strip().split("\t")
assert store["code"] == 200 and store["data"] == [], store
assert created[:6] == [
    "1",
    "Go迁移测试企业",
    "wx-test-corp",
    "employee-secret",
    "http://127.0.0.1:9501/weWork/callback",
    "contact-secret",
], created
assert created[6] and len(created[7]) == 43 and created[8] == "1", created
assert cache_value == "1-0", cache_value
PY
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -e "UPDATE mc_corp SET token = 'callback-token', encoding_aes_key = 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG' WHERE id = 1;"
cat >"$WORK_DIR/wework_callback_payload.go" <<'GO'
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	token   = "callback-token"
	aesKey  = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	corpID  = "wx-test-corp"
	ts      = "1710000000"
	nonce   = "go-callback-nonce"
)

func main() {
	dir := os.Args[1]
	getEncrypted := encrypt([]byte("verified"))
	write(filepath.Join(dir, "wework_callback_get_query.txt"), query("1", getEncrypted))

	eventXML := []byte(`<xml><ToUserName><![CDATA[wx-test-corp]]></ToUserName><MsgType><![CDATA[event]]></MsgType><Event><![CDATA[change_contact]]></Event><ChangeType><![CDATA[create_user]]></ChangeType><UserID><![CDATA[go-callback-user]]></UserID></xml>`)
	postEncrypted := encrypt(eventXML)
	write(filepath.Join(dir, "wework_callback_post_query.txt"), query("", postEncrypted))
	write(filepath.Join(dir, "wework_callback_post_body.xml"), fmt.Sprintf(`<xml><ToUserName><![CDATA[wx-test-corp]]></ToUserName><Encrypt><![CDATA[%s]]></Encrypt></xml>`, postEncrypted))
}

func query(cid string, encrypted string) string {
	values := url.Values{}
	if cid != "" {
		values.Set("cid", cid)
	}
	values.Set("timestamp", ts)
	values.Set("nonce", nonce)
	values.Set("msg_signature", signature(encrypted))
	values.Set("echostr", encrypted)
	if cid == "" {
		values.Del("echostr")
	}
	return values.Encode()
}

func signature(encrypted string) string {
	items := []string{token, ts, nonce, encrypted}
	sort.Strings(items)
	sum := sha1.Sum([]byte(strings.Join(items, "")))
	return hex.EncodeToString(sum[:])
}

func encrypt(message []byte) string {
	key, err := base64.StdEncoding.DecodeString(aesKey + "=")
	if err != nil {
		panic(err)
	}
	var plain bytes.Buffer
	plain.Write([]byte("abcdefghijklmnop"))
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(message)))
	plain.Write(size[:])
	plain.Write(message)
	plain.WriteString(corpID)
	padded := pkcs7Pad(plain.Bytes(), aes.BlockSize)
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	return append(append([]byte{}, data...), bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func write(path string, value string) {
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		panic(err)
	}
}
GO
env -u GOROOT go run "$WORK_DIR/wework_callback_payload.go" "$WORK_DIR"
wework_get_query="$(cat "$WORK_DIR/wework_callback_get_query.txt")"
curl -sS -f "http://$GO_ADDR/dashboard/corp/weWorkCallback?$wework_get_query" >"$WORK_DIR/go_wework_callback_get.txt"
wework_post_query="$(cat "$WORK_DIR/wework_callback_post_query.txt")"
curl -sS -f -X POST -H 'Content-Type: application/xml' --data-binary @"$WORK_DIR/wework_callback_post_body.xml" "http://$GO_ADDR/weWork/callback?$wework_post_query" >"$WORK_DIR/go_wework_callback_post.txt"
compose exec -T redis redis-cli --raw LRANGE mochat-go:wework-callback 0 -1 >"$WORK_DIR/wework_callback_queue.jsonl"
python3 - "$WORK_DIR" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
assert (work / "go_wework_callback_get.txt").read_text(encoding="utf-8") == "verified"
assert (work / "go_wework_callback_post.txt").read_text(encoding="utf-8") == "success"
events = [json.loads(line) for line in (work / "wework_callback_queue.jsonl").read_text(encoding="utf-8").splitlines() if line.strip()]
assert len(events) == 1, events
event = events[0]
assert event["corpId"] == 1 and event["wxCorpId"] == "wx-test-corp", event
assert event["eventPath"] == "event.change_contact.create_user", event
assert event["message"]["UserID"] == "go-callback-user", event
PY
compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
INSERT INTO mc_corp (id, name, wx_corpid, social_code, employee_secret, event_callback, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at)
VALUES (2, 'Go迁移无权限企业', 'wx-test-forbidden-corp', '', '', '', '', '', '', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), tenant_id=VALUES(tenant_id), updated_at=NOW(), deleted_at=NULL;
SQL
compose exec -T redis redis-cli DEL mc:user.1 >/dev/null

sidebar_token="$(compose exec -T php php <<'PHP'
<?php
function b64url(string $raw): string {
    return rtrim(strtr(base64_encode($raw), '+/', '-_'), '=');
}
$secret = getenv('SIDEBAR_JWT_SECRET') ?: 'Br3LXhp&Ysha1zRDh';
$now = time();
$uid = 1;
$payload = [
    'sub' => '1',
    'iss' => 'http://:',
    'exp' => $now + 604800,
    'iat' => $now,
    'nbf' => $now,
    'uid' => $uid,
    's' => bin2hex(random_bytes(16)),
    'jti' => md5($now . '-' . $uid . '-' . bin2hex(random_bytes(16))),
];
$header = b64url('{"typ":"jwt"}');
$body = b64url(json_encode($payload, JSON_UNESCAPED_SLASHES));
$signature = password_hash(md5($header . '.' . $body . $secret), PASSWORD_DEFAULT);
echo $header . '.' . $body . '.' . b64url($signature);
PHP
)"

curl -sS -f "http://$GO_ADDR/sidebar/agent/oauth?agentId=1&act=medium&isJsRedirect=1" >"$WORK_DIR/go_sidebar_agent_oauth_url.json"
curl -sS -f "http://$GO_ADDR/sidebar/agent/oauth?agentId=1&code=go-oauth-code" >"$WORK_DIR/go_sidebar_agent_oauth_token.json"
curl -sS -o /dev/null -w '%{http_code} %{redirect_url}' "http://$GO_ADDR/sidebar/agent/auth?agentId=1&target=%2Fcontact" >"$WORK_DIR/go_sidebar_agent_auth_redirect.txt"
curl -sS -o /dev/null -w '%{http_code} %{redirect_url}' "http://$GO_ADDR/sidebar/agent/auth?agentId=1&target=%2Fcontact&code=go-sidebar-code" >"$WORK_DIR/go_sidebar_agent_auth_callback.txt"
curl -sS -f -H "Authorization: Bearer $sidebar_token" "http://$GO_ADDR/sidebar/agent/jssdkConfig?agentId=1&uriPath=http%3A%2F%2Fsidebar.local%2Fcontact" >"$WORK_DIR/go_sidebar_agent_jssdk_config.json"
curl -sS -f "http://$GO_ADDR/sidebar/wxJsSdk/config?corpId=1&agentId=1&uriPath=/contact" >"$WORK_DIR/go_sidebar_wx_jssdk_config.json"
curl -sS -f "http://$GO_ADDR/sidebar/wxJsSdk/config?corpId=1&uriPath=/contact" >"$WORK_DIR/go_sidebar_wx_jssdk_corp_config.json"
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d '{"wxAgentId":"1000003","wxSecret":"agent-secret-created","type":1}' "http://$GO_ADDR/dashboard/agent/store" >"$WORK_DIR/go_agent_store.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT corp_id, wx_agent_id, wx_secret, name, square_logo_url, description, close, redirect_domain, report_location_flag, is_reportenter, home_url FROM mc_work_agent WHERE wx_agent_id = '1000003' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1" >"$WORK_DIR/agent_store_created.tsv"

auto_pull_store_payload='{"corpId":1,"qrcodeName":"自动拉群写入测试","isVerified":2,"leadingWords":"写入欢迎语","employees":"1","tags":"900001","rooms":"[{\"roomId\":900001,\"maxNum\":45,\"roomQrcodeUrl\":\"qrcode/room-created.png\"}]"}'
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$auto_pull_store_payload" "http://$GO_ADDR/dashboard/workRoomAutoPull/store" >"$WORK_DIR/go_work_room_auto_pull_store.json"
auto_pull_created_id="$(compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT id FROM mc_work_room_auto_pull WHERE qrcode_name = '自动拉群写入测试' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1;")"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT qrcode_url, wx_config_id FROM mc_work_room_auto_pull WHERE id = ${auto_pull_created_id};" >"$WORK_DIR/work_room_auto_pull_created_qrcode.tsv"

auto_pull_update_payload='{"workRoomAutoPullId":900001,"isVerified":1,"employees":"1","tags":"900001","rooms":"[{\"roomId\":900001,\"maxNum\":40,\"roomQrcodeUrl\":\"qrcode/room-auto-pull-updated.png\"}]"}'
curl -sS -f -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$auto_pull_update_payload" "http://$GO_ADDR/dashboard/workRoomAutoPull/update" >"$WORK_DIR/go_work_room_auto_pull_update.json"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT is_verified, employees, tags, rooms FROM mc_work_room_auto_pull WHERE id = 900001;" >"$WORK_DIR/work_room_auto_pull_after_update.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT event, operation_id, business_id FROM mc_business_log WHERE business_id IN (${auto_pull_created_id}, 900001) AND event IN (200, 201) ORDER BY event ASC;" >"$WORK_DIR/work_room_auto_pull_business_logs.tsv"

curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/corp/select?corpName=测试" >"$WORK_DIR/go_corp_select.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/corp/index?corpName=Go&page=1&perPage=10" >"$WORK_DIR/go_corp_index.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/corp/show?corpId=1" >"$WORK_DIR/go_corp_show.json"
curl -sS -f -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d '{"corpId":1,"corpName":"Go迁移更新企业","wxCorpId":"wx-test-corp","employeeSecret":"employee-secret-updated","contactSecret":"contact-secret-updated"}' "http://$GO_ADDR/dashboard/corp/update" >"$WORK_DIR/go_corp_update.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/corp/show?corpId=1" >"$WORK_DIR/go_corp_show_after_update.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/user/loginShow" >"$WORK_DIR/go_login_show.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/corpData/index" >"$WORK_DIR/go_corp_data_index.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/corpData/lineChat" >"$WORK_DIR/go_corp_data_line_chat.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workEmployee/index?corpId=1&status=1&contactAuth=2&page=1&perPage=10" >"$WORK_DIR/go_work_employee_index.json"
php_work_employee_index_code="$(curl -sS -o "$WORK_DIR/php_work_employee_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workEmployee/index?corpId=1&status=1&contactAuth=2&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workEmployee/searchCondition" >"$WORK_DIR/go_work_employee_search_condition.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workDepartment/index" >"$WORK_DIR/go_work_department_index.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workEmployeeDepartment/memberIndex?departmentIds=900001,900002" >"$WORK_DIR/go_work_department_member_index.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workDepartment/selectByPhone?phone=13800000000&type=2" >"$WORK_DIR/go_work_department_select_by_phone.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workDepartment/pageIndex?page=1&perPage=10" >"$WORK_DIR/go_work_department_page_index.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workDepartment/showEmployee?departmentId=900001&page=1&perPage=10" >"$WORK_DIR/go_work_department_show_employee.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContactTagGroup/index" >"$WORK_DIR/go_work_contact_tag_group_index.json"
php_work_contact_tag_group_index_code="$(curl -sS -o "$WORK_DIR/php_work_contact_tag_group_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workContactTagGroup/index")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContactTagGroup/detail?groupId=900001" >"$WORK_DIR/go_work_contact_tag_group_detail.json"
php_work_contact_tag_group_detail_code="$(curl -sS -o "$WORK_DIR/php_work_contact_tag_group_detail.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workContactTagGroup/detail?groupId=900001")"
curl -sS -f -H "Authorization: Bearer $sidebar_token" "http://$GO_ADDR/sidebar/workContactTagGroup/index" >"$WORK_DIR/go_sidebar_work_contact_tag_group_index.json"
php_sidebar_work_contact_tag_group_index_code="$(curl -sS -o "$WORK_DIR/php_sidebar_work_contact_tag_group_index.json" -w '%{http_code}' -H "Authorization: Bearer $sidebar_token" "http://127.0.0.1:$MOCHAT_PHP_PORT/sidebar/workContactTagGroup/index")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContactTag/index?groupId=900001&page=1&perPage=10" >"$WORK_DIR/go_work_contact_tag_index.json"
php_work_contact_tag_index_code="$(curl -sS -o "$WORK_DIR/php_work_contact_tag_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workContactTag/index?groupId=900001&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContactTag/detail?tagId=900001" >"$WORK_DIR/go_work_contact_tag_detail.json"
php_work_contact_tag_detail_code="$(curl -sS -o "$WORK_DIR/php_work_contact_tag_detail.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workContactTag/detail?tagId=900001")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContactTag/contactTagList?name=%E8%BF%81%E7%A7%BB" >"$WORK_DIR/go_work_contact_tag_list.json"
php_work_contact_tag_list_code="$(curl -sS -o "$WORK_DIR/php_work_contact_tag_list.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workContactTag/contactTagList?name=%E8%BF%81%E7%A7%BB")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContactTag/allTag?groupId=900001" >"$WORK_DIR/go_work_contact_tag_all.json"
php_work_contact_tag_all_code="$(curl -sS -o "$WORK_DIR/php_work_contact_tag_all.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workContactTag/allTag?groupId=900001")"
curl -sS -f -X PUT -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContactTag/synContactTag" >"$WORK_DIR/go_work_contact_tag_sync.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact_tag_group WHERE wx_group_id = 'go-tag-group-sync' AND group_name = 'Go迁移同步标签组' AND \`order\` = 8 AND corp_id = 1 AND deleted_at IS NULL;" >"$WORK_DIR/work_contact_tag_sync_group_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact_tag tag JOIN mc_work_contact_tag_group grp ON grp.id = tag.contact_tag_group_id WHERE tag.wx_contact_tag_id = 'go-contact-tag-sync' AND tag.name = 'Go迁移同步标签' AND tag.\`order\` = 9 AND tag.corp_id = 1 AND grp.wx_group_id = 'go-tag-group-sync' AND tag.deleted_at IS NULL AND grp.deleted_at IS NULL;" >"$WORK_DIR/work_contact_tag_sync_tag_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT deleted_at IS NOT NULL FROM mc_work_contact_tag_group WHERE id = 900004;" >"$WORK_DIR/work_contact_tag_sync_old_group_deleted.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT deleted_at IS NOT NULL FROM mc_work_contact_tag WHERE id = 900004;" >"$WORK_DIR/work_contact_tag_sync_old_tag_deleted.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_update_time WHERE corp_id = 1 AND type = 3 AND last_update_time IS NOT NULL;" >"$WORK_DIR/work_contact_tag_sync_time_count.txt"
python3 - "$WORK_DIR" "$WORK_DIR/wecom_requests.ndjson" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
request_log = pathlib.Path(sys.argv[2])

def text(name):
    return (work / name).read_text(encoding="utf-8").strip()

sync = json.loads(text("go_work_contact_tag_sync.json"))
assert sync["code"] == 200 and sync["data"] == [], sync
assert text("work_contact_tag_sync_group_count.txt") == "1"
assert text("work_contact_tag_sync_tag_count.txt") == "1"
assert text("work_contact_tag_sync_old_group_deleted.txt") == "1"
assert text("work_contact_tag_sync_old_tag_deleted.txt") == "1"
assert text("work_contact_tag_sync_time_count.txt") == "1"
requests = [json.loads(line) for line in request_log.read_text(encoding="utf-8").splitlines() if line.strip()]
assert any("/cgi-bin/externalcontact/get_corp_tag_list" in item["path"] for item in requests), requests
PY
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/channelCode/index?name=Go%E8%BF%81%E7%A7%BB&page=1&perPage=20" >"$WORK_DIR/go_channel_code_index.json"
curl -sS -f -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/channelCode/index?name=Go%E8%BF%81%E7%A7%BB&page=1&perPage=20" >"$WORK_DIR/php_channel_code_index.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/channelCode/show?channelCodeId=900003" >"$WORK_DIR/go_channel_code_show.json"
curl -sS -f -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/channelCode/show?channelCodeId=900003" >"$WORK_DIR/php_channel_code_show.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/channelCode/contact?channelCodeId=900003&page=1&perPage=15" >"$WORK_DIR/go_channel_code_contact.json"
curl -sS -f -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/channelCode/contact?channelCodeId=900003&page=1&perPage=15" >"$WORK_DIR/php_channel_code_contact.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/channelCode/statistics?channelCodeId=900003&type=1&startTime=2026-07-01&endTime=2026-07-03" >"$WORK_DIR/go_channel_code_statistics.json"
curl -sS -f -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/channelCode/statistics?channelCodeId=900003&type=1&startTime=2026-07-01&endTime=2026-07-03" >"$WORK_DIR/php_channel_code_statistics.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/channelCode/statisticsIndex?channelCodeId=900003&type=1&startTime=2026-07-01&endTime=2026-07-03&page=1&perPage=10" >"$WORK_DIR/go_channel_code_statistics_index.json"
curl -sS -f -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/channelCode/statisticsIndex?channelCodeId=900003&type=1&startTime=2026-07-01&endTime=2026-07-03&page=1&perPage=10" >"$WORK_DIR/php_channel_code_statistics_index.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/channelCodeGroup/index" >"$WORK_DIR/go_channel_code_group_index.json"
php_channel_code_group_index_code="$(curl -sS -o "$WORK_DIR/php_channel_code_group_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/channelCodeGroup/index")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/channelCodeGroup/detail?groupId=900001" >"$WORK_DIR/go_channel_code_group_detail.json"
php_channel_code_group_detail_code="$(curl -sS -o "$WORK_DIR/php_channel_code_group_detail.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/channelCodeGroup/detail?groupId=900001")"
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d '{"name":["Go迁移渠道分组新增"]}' "http://$GO_ADDR/dashboard/channelCodeGroup/store" >"$WORK_DIR/go_channel_code_group_store.json"
curl -sS -f -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d '{"groupId":900001,"name":"Go迁移渠道分组更新"}' "http://$GO_ADDR/dashboard/channelCodeGroup/update" >"$WORK_DIR/go_channel_code_group_update.json"
curl -sS -f -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d '{"channelCodeId":900003,"groupId":900001}' "http://$GO_ADDR/dashboard/channelCodeGroup/move" >"$WORK_DIR/go_channel_code_group_move.json"
channel_code_store_payload='{"baseInfo":{"groupId":900001,"name":"Go迁移渠道码新增","autoAddFriend":1,"tags":[900001]},"drainageEmployee":{"type":1,"employees":[],"specialPeriod":{"status":1,"detail":[{"startDate":"2026-01-01","endDate":"2026-12-31","timeSlot":[{"startTime":"00:00","endTime":"00:00","employeeId":[1]}]}]},"addMax":{"status":2,"employees":[],"spareEmployeeIds":[]}},"welcomeMessage":{"scanCodePush":2,"messageDetail":[]}}'
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$channel_code_store_payload" "http://$GO_ADDR/dashboard/channelCode/store" >"$WORK_DIR/go_channel_code_store.json"
channel_code_created_id="$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT id FROM mc_channel_code WHERE name = 'Go迁移渠道码新增' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
channel_code_update_payload="{\"channelCodeId\":${channel_code_created_id},\"baseInfo\":{\"groupId\":900001,\"name\":\"Go迁移渠道码更新\",\"autoAddFriend\":2,\"tags\":[900001]},\"drainageEmployee\":{\"type\":1,\"employees\":[],\"specialPeriod\":{\"status\":1,\"detail\":[{\"startDate\":\"2026-01-01\",\"endDate\":\"2026-12-31\",\"timeSlot\":[{\"startTime\":\"00:00\",\"endTime\":\"00:00\",\"employeeId\":[1]}]}]},\"addMax\":{\"status\":2,\"employees\":[],\"spareEmployeeIds\":[]}},\"welcomeMessage\":{\"scanCodePush\":2,\"messageDetail\":[]}}"
curl -sS -f -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$channel_code_update_payload" "http://$GO_ADDR/dashboard/channelCode/update" >"$WORK_DIR/go_channel_code_update.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT id, name, qrcode_url, wx_config_id, auto_add_friend, group_id, type, tags FROM mc_channel_code WHERE id = ${channel_code_created_id}" >"$WORK_DIR/channel_code_created.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT event, operation_id, business_id FROM mc_business_log WHERE business_id = ${channel_code_created_id} AND event IN (100, 101) ORDER BY event ASC;" >"$WORK_DIR/channel_code_business_logs.tsv"
channel_group_created_count="$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_channel_code_group WHERE name = 'Go迁移渠道分组新增' AND deleted_at IS NULL")"
channel_group_updated_name="$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT name FROM mc_channel_code_group WHERE id = 900001")"
channel_code_moved_group="$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT group_id FROM mc_channel_code WHERE id = 900003")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContact/index?keyWords=%E8%BF%81%E7%A7%BB&page=1&perPage=20" >"$WORK_DIR/go_work_contact_index.json"
php_work_contact_index_code="$(curl -sS -o "$WORK_DIR/php_work_contact_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workContact/index?keyWords=%E8%BF%81%E7%A7%BB&page=1&perPage=20")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContact/lossContact?page=1&perPage=20" >"$WORK_DIR/go_work_contact_loss.json"
php_work_contact_loss_code="$(curl -sS -o "$WORK_DIR/php_work_contact_loss.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workContact/lossContact?page=1&perPage=20")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContact/source" >"$WORK_DIR/go_work_contact_source.json"
php_work_contact_source_code="$(curl -sS -o "$WORK_DIR/php_work_contact_source.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workContact/source")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContact/show?contactId=900001&employeeId=1" >"$WORK_DIR/go_work_contact_show.json"
php_work_contact_show_code="$(curl -sS -o "$WORK_DIR/php_work_contact_show.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workContact/show?contactId=900001&employeeId=1")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContact/track?contactId=900001" >"$WORK_DIR/go_work_contact_track.json"
php_work_contact_track_code="$(curl -sS -o "$WORK_DIR/php_work_contact_track.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workContact/track?contactId=900001")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContactRoom/index?workRoomId=900001&page=1&perPage=10" >"$WORK_DIR/go_work_contact_room_index.json"
php_work_contact_room_index_code="$(curl -sS -o "$WORK_DIR/php_work_contact_room_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workContactRoom/index?workRoomId=900001&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workContactRoom/index?workRoomId=900001&status=0&page=1&perPage=10" >"$WORK_DIR/go_work_contact_room_index_status0.json"
php_work_contact_room_index_status0_code="$(curl -sS -o "$WORK_DIR/php_work_contact_room_index_status0.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workContactRoom/index?workRoomId=900001&status=0&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workRoom/index?workRoomName=%E8%BF%81%E7%A7%BB&page=1&perPage=10" >"$WORK_DIR/go_work_room_index.json"
php_work_room_index_code="$(curl -sS -o "$WORK_DIR/php_work_room_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workRoom/index?workRoomName=%E8%BF%81%E7%A7%BB&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workRoom/roomIndex?name=%E8%BF%81%E7%A7%BB" >"$WORK_DIR/go_work_room_room_index.json"
php_work_room_room_index_code="$(curl -sS -o "$WORK_DIR/php_work_room_room_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workRoom/roomIndex?name=%E8%BF%81%E7%A7%BB")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workRoom/statistics?workRoomId=900001&type=1&startTime=$smoke_today&endTime=$smoke_today" >"$WORK_DIR/go_work_room_statistics.json"
php_work_room_statistics_code="$(curl -sS -o "$WORK_DIR/php_work_room_statistics.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workRoom/statistics?workRoomId=900001&type=1&startTime=$smoke_today&endTime=$smoke_today")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workRoom/statisticsIndex?workRoomId=900001&type=1&startTime=$smoke_today&endTime=$smoke_today&page=1&perPage=10" >"$WORK_DIR/go_work_room_statistics_index.json"
php_work_room_statistics_index_code="$(curl -sS -o "$WORK_DIR/php_work_room_statistics_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workRoom/statisticsIndex?workRoomId=900001&type=1&startTime=$smoke_today&endTime=$smoke_today&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactTransfer/saveUnassignedList" >"$WORK_DIR/go_contact_transfer_sync.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactTransfer/info?contactName=Go%E8%BF%81%E7%A7%BB&employeeId=%5B1%5D" >"$WORK_DIR/go_contact_transfer_info.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactTransfer/unassignedList?contactName=Go%E8%BF%81%E7%A7%BB&employeeId=%5B1%5D" >"$WORK_DIR/go_contact_transfer_unassigned.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactTransfer/room?roomName=Go%E8%BF%81%E7%A7%BB" >"$WORK_DIR/go_contact_transfer_room.json"
contact_transfer_customer_payload='{"type":1,"takeoverUserId":"go-normal-user","list":"[{\"contactWxId\":\"external-user-900001\",\"employeeWxId\":\"go-migrate-user\"}]"}'
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$contact_transfer_customer_payload" "http://$GO_ADDR/dashboard/contactTransfer/index" >"$WORK_DIR/go_contact_transfer_index.json"
contact_transfer_room_payload='{"takeoverUserId":"go-normal-user","list":"[\"go-room-900001\"]"}'
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$contact_transfer_room_payload" "http://$GO_ADDR/dashboard/contactTransfer/room" >"$WORK_DIR/go_contact_transfer_room_store.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactTransfer/log?mode=1&name=Go%E8%BF%81%E7%A7%BB" >"$WORK_DIR/go_contact_transfer_log_mode1.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactTransfer/log?mode=2&name=Go%E8%BF%81%E7%A7%BB" >"$WORK_DIR/go_contact_transfer_log_mode2.json"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT COUNT(*) FROM mc_work_unassigned WHERE corp_id = 1 AND handover_userid = 'go-migrate-user' AND external_userid = 'external-user-900001' AND deleted_at IS NULL;" >"$WORK_DIR/contact_transfer_unassigned_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT status, type, name, contact_id, handover_employee_id, takeover_employee_id FROM mc_work_transfer_log WHERE corp_id = 1 AND contact_id IN ('external-user-900001', 'go-room-900001') ORDER BY id DESC LIMIT 2;" >"$WORK_DIR/contact_transfer_logs.tsv"
python3 - "$WORK_DIR" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])

def load(name):
    return json.loads((work / name).read_text(encoding="utf-8"))

sync = load("go_contact_transfer_sync.json")
info = load("go_contact_transfer_info.json")
unassigned = load("go_contact_transfer_unassigned.json")
rooms = load("go_contact_transfer_room.json")
index = load("go_contact_transfer_index.json")
room_store = load("go_contact_transfer_room_store.json")
log1 = load("go_contact_transfer_log_mode1.json")
log2 = load("go_contact_transfer_log_mode2.json")

assert sync["code"] == 200 and sync["data"] == [], sync
assert info["code"] == 200 and any(item["contactWxId"] == "external-user-900001" for item in info["data"]), info
assert unassigned["code"] == 200 and any(item["contactWxId"] == "external-user-900001" for item in unassigned["data"]["list"]), unassigned
assert rooms["code"] == 200 and any(item["chatId"] == "go-room-900001" for item in rooms["data"]), rooms
assert index["code"] == 200 and index["data"][0]["errcode"] == 0, index
assert room_store["code"] == 200 and room_store["data"] == [], room_store
assert log1["code"] == 200 and any(item["name"] == "Go迁移客户" for item in log1["data"]), log1
assert log2["code"] == 200 and any(item["name"] == "Go迁移客户群" for item in log2["data"]), log2
assert (work / "contact_transfer_unassigned_count.txt").read_text(encoding="utf-8").strip() == "1"
logs = (work / "contact_transfer_logs.tsv").read_text(encoding="utf-8").strip()
assert "external-user-900001" in logs and "go-room-900001" in logs, logs
PY
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workRoomAutoPull/index?qrcodeName=Go&page=1&perPage=10" >"$WORK_DIR/go_work_room_auto_pull_index.json"
php_work_room_auto_pull_index_code="$(curl -sS -o "$WORK_DIR/php_work_room_auto_pull_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workRoomAutoPull/index?qrcodeName=Go&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workRoomAutoPull/show?workRoomAutoPullId=900001" >"$WORK_DIR/go_work_room_auto_pull_show.json"
php_work_room_auto_pull_show_code="$(curl -sS -o "$WORK_DIR/php_work_room_auto_pull_show.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workRoomAutoPull/show?workRoomAutoPullId=900001")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/roomTagPull/index?name=Go&page=1&perPage=10" >"$WORK_DIR/go_room_tag_pull_index.json"
php_room_tag_pull_index_code="$(curl -sS -o "$WORK_DIR/php_room_tag_pull_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/roomTagPull/index?name=Go&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/roomTagPull/show?id=900001" >"$WORK_DIR/go_room_tag_pull_show.json"
php_room_tag_pull_show_code="$(curl -sS -o "$WORK_DIR/php_room_tag_pull_show.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/roomTagPull/show?id=900001")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/roomTagPull/showContact?id=900001&type=1&page=1&perPage=10" >"$WORK_DIR/go_room_tag_pull_show_contact.json"
php_room_tag_pull_show_contact_code="$(curl -sS -o "$WORK_DIR/php_room_tag_pull_show_contact.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/roomTagPull/showContact?id=900001&type=1&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/roomTagPull/showContact?id=900001&type=2&page=1&perPage=10" >"$WORK_DIR/go_room_tag_pull_show_employee.json"
php_room_tag_pull_show_employee_code="$(curl -sS -o "$WORK_DIR/php_room_tag_pull_show_employee.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/roomTagPull/showContact?id=900001&type=2&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/roomTagPull/roomList?employees[]=1&name=Go" >"$WORK_DIR/go_room_tag_pull_room_list.json"
php_room_tag_pull_room_list_code="$(curl -sS -o "$WORK_DIR/php_room_tag_pull_room_list.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/roomTagPull/roomList?employees[]=1&name=Go")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/roomTagPull/chooseContact?employees[]=1&is_all=1&gender=1&tag_ids[]=900001" >"$WORK_DIR/go_room_tag_pull_choose_contact.json"
php_room_tag_pull_choose_contact_code="$(curl -sS -o "$WORK_DIR/php_room_tag_pull_choose_contact.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/roomTagPull/chooseContact?employees[]=1&is_all=1&gender=1&tag_ids[]=900001")"
room_tag_pull_filter_payload='{"employees":[1],"choose_contact":{"is_all":1,"gender":1,"tag_ids":[900001]},"rooms":[{"id":900001,"name":"Go迁移客户群","num":50}]}'
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$room_tag_pull_filter_payload" "http://$GO_ADDR/dashboard/roomTagPull/filterContact" >"$WORK_DIR/go_room_tag_pull_filter_contact.json"
php_room_tag_pull_filter_contact_code="$(curl -sS -o "$WORK_DIR/php_room_tag_pull_filter_contact.json" -w '%{http_code}' -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$room_tag_pull_filter_payload" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/roomTagPull/filterContact")"
room_tag_pull_store_payload='{"name":"Go迁移标签建群新增","employees":[1],"choose_contact":{"is_all":1,"gender":1,"tag_ids":[900001]},"guide":"请扫码入群","rooms":[{"id":900001,"name":"Go迁移客户群","num":50,"image":"data:image/jpeg;base64,aGVsbG8gd29ybGQ="}],"filter_contact":0}'
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$room_tag_pull_store_payload" "http://$GO_ADDR/dashboard/roomTagPull/store" >"$WORK_DIR/go_room_tag_pull_store.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/roomTagPull/remindSend?id=900001&wxUserId=go-migrate-user" >"$WORK_DIR/go_room_tag_pull_remind_send.json"
room_tag_pull_created_id="$(python3 - "$WORK_DIR/go_room_tag_pull_store.json" <<'PY'
import json
import sys
body = json.load(open(sys.argv[1], encoding="utf-8"))
print(body["data"][0])
PY
)"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT name, contact_num, JSON_LENGTH(wx_tid) FROM mc_room_tag_pull WHERE id = ${room_tag_pull_created_id};" >"$WORK_DIR/room_tag_pull_created.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT COUNT(*) FROM mc_room_tag_pull_contact WHERE room_tag_pull_id = ${room_tag_pull_created_id} AND deleted_at IS NULL;" >"$WORK_DIR/room_tag_pull_created_contacts.tsv"
curl -sS -f -X DELETE -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d '{"id":900002}' "http://$GO_ADDR/dashboard/roomTagPull/destroy" >"$WORK_DIR/go_room_tag_pull_destroy.json"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT deleted_at IS NOT NULL FROM mc_room_tag_pull WHERE id = 900002;" >"$WORK_DIR/room_tag_pull_deleted.tsv"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactMessageBatchSend/index?page=1&perPage=10" >"$WORK_DIR/go_contact_message_batch_send_index.json"
php_contact_message_batch_send_index_code="$(curl -sS -o "$WORK_DIR/php_contact_message_batch_send_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/contactMessageBatchSend/index?page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactMessageBatchSend/show?batchId=900001" >"$WORK_DIR/go_contact_message_batch_send_show.json"
php_contact_message_batch_send_show_code="$(curl -sS -o "$WORK_DIR/php_contact_message_batch_send_show.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/contactMessageBatchSend/show?batchId=900001")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactMessageBatchSend/showRoom?workRoomId=900001" >"$WORK_DIR/go_contact_message_batch_send_show_room.json"
php_contact_message_batch_send_show_room_code="$(curl -sS -o "$WORK_DIR/php_contact_message_batch_send_show_room.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/contactMessageBatchSend/showRoom?workRoomId=900001")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactMessageBatchSend/employeeSendIndex?batchId=900001&sendStatus=1&page=1&perPage=15" >"$WORK_DIR/go_contact_message_batch_send_employee_index.json"
php_contact_message_batch_send_employee_index_code="$(curl -sS -o "$WORK_DIR/php_contact_message_batch_send_employee_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/contactMessageBatchSend/employeeSendIndex?batchId=900001&sendStatus=1&page=1&perPage=15")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactMessageBatchSend/contactReceiveIndex?batchId=900001&sendStatus=1&page=1&perPage=15" >"$WORK_DIR/go_contact_message_batch_send_contact_index.json"
php_contact_message_batch_send_contact_index_code="$(curl -sS -o "$WORK_DIR/php_contact_message_batch_send_contact_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/contactMessageBatchSend/contactReceiveIndex?batchId=900001&sendStatus=1&page=1&perPage=15")"
contact_message_batch_send_store_payload='{"employeeIds":[1],"filterParams":{"gender":1,"rooms":[900001],"tags":[900001],"addTimeStart":"2026-07-01","addTimeEnd":"2026-07-04"},"content":[{"msgType":"text","content":"Go迁移客户群发新增"},{"msgType":"image","pic_url":"medium/go-media.png"}],"sendWay":1}'
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$contact_message_batch_send_store_payload" "http://$GO_ADDR/dashboard/contactMessageBatchSend/store" >"$WORK_DIR/go_contact_message_batch_send_store.json"
contact_message_batch_send_created_id="$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT id FROM mc_contact_message_batch_send WHERE content LIKE '%Go迁移客户群发新增%' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT send_status, send_employee_total, send_contact_total, not_received_total FROM mc_contact_message_batch_send WHERE id = ${contact_message_batch_send_created_id};" >"$WORK_DIR/contact_message_batch_send_created.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT status, receive_status, msg_id, send_time IS NOT NULL FROM mc_contact_message_batch_send_employee WHERE batch_id = ${contact_message_batch_send_created_id};" >"$WORK_DIR/contact_message_batch_send_created_employee.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_contact_message_batch_send_result WHERE batch_id = ${contact_message_batch_send_created_id} AND contact_id = 900001 AND external_user_id = 'external-user-900001';" >"$WORK_DIR/contact_message_batch_send_created_result_count.txt"
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d '{"batchId":900001,"batchEmployId":900001}' "http://$GO_ADDR/dashboard/contactMessageBatchSend/remind" >"$WORK_DIR/go_contact_message_batch_send_remind.json"
curl -sS -f -X DELETE -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d '{"batchId":900002}' "http://$GO_ADDR/dashboard/contactMessageBatchSend/destroy" >"$WORK_DIR/go_contact_message_batch_send_destroy.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT deleted_at IS NOT NULL FROM mc_contact_message_batch_send WHERE id = 900002;" >"$WORK_DIR/contact_message_batch_send_deleted.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_contact_message_batch_send_employee WHERE batch_id = 900002;" >"$WORK_DIR/contact_message_batch_send_deleted_employee_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_contact_message_batch_send_result WHERE batch_id = 900002;" >"$WORK_DIR/contact_message_batch_send_deleted_result_count.txt"
python3 - "$WORK_DIR" "$WORK_DIR/wecom_requests.ndjson" "$php_contact_message_batch_send_index_code" "$php_contact_message_batch_send_show_code" "$php_contact_message_batch_send_show_room_code" "$php_contact_message_batch_send_employee_index_code" "$php_contact_message_batch_send_contact_index_code" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
request_log = pathlib.Path(sys.argv[2])
codes = sys.argv[3:]
if any(code != "200" for code in codes):
    raise SystemExit(f"unexpected contactMessageBatchSend PHP status codes: {codes}")

def load(name):
    return json.loads((work / name).read_text(encoding="utf-8"))

go_index = load("go_contact_message_batch_send_index.json")
php_index = load("php_contact_message_batch_send_index.json")
go_show = load("go_contact_message_batch_send_show.json")
php_show = load("php_contact_message_batch_send_show.json")
go_room = load("go_contact_message_batch_send_show_room.json")
php_room = load("php_contact_message_batch_send_show_room.json")
go_employee = load("go_contact_message_batch_send_employee_index.json")
php_employee = load("php_contact_message_batch_send_employee_index.json")
go_contact = load("go_contact_message_batch_send_contact_index.json")
php_contact = load("php_contact_message_batch_send_contact_index.json")
store = load("go_contact_message_batch_send_store.json")
remind = load("go_contact_message_batch_send_remind.json")
destroy = load("go_contact_message_batch_send_destroy.json")

assert go_index["code"] == 200 and php_index["code"] == 200, (go_index, php_index)
go_item = next(item for item in go_index["data"]["list"] if item["id"] == 900001)
assert go_item["sendWay"] == 1 and go_item["sendStatus"] == 1, go_item
assert go_item["sendTotal"] == 1 and go_item["receivedTotal"] == 1, go_item
assert go_item["content"][0]["content"] == "Go迁移客户群发文本", go_item
assert go_item["content"][1]["pic_url"] == "http://127.0.0.1:9501/static/medium/go-media.png", go_item

assert go_show["code"] == 200 and php_show["code"] == 200, (go_show, php_show)
show = go_show["data"]
assert show["creator"] == "Go迁移测试用户", show
assert show["filterParams"]["gender"] == 1 and show["filterParams"]["rooms"] == [900001], show
assert show["filterParamsDetail"]["rooms"][0]["name"] == "Go迁移客户群", show
assert show["sendEmployeeTotal"] == 1 and show["sendContactTotal"] == 1, show

assert go_room["code"] == 200 and php_room["code"] == 200, (go_room, php_room)
assert go_room["data"]["name"] == "Go迁移客户群", go_room
assert go_room["data"]["ownerName"] in {"Go迁移员工", "Go迁移更新企业-Go迁移员工"}, go_room
assert go_room["data"]["totalContact"] >= 1, go_room

assert go_employee["code"] == 200 and php_employee["code"] == 200, (go_employee, php_employee)
employee = go_employee["data"]["list"][0]
assert employee["employeeId"] == 1 and employee["employeeName"] == "Go迁移员工", employee
assert employee["status"] == 1 and employee["sendContactTotal"] == 1, employee

assert go_contact["code"] == 200 and php_contact["code"] == 200, (go_contact, php_contact)
contact = go_contact["data"]["list"][0]
assert contact["contactId"] == 900001 and contact["contactName"] == "Go迁移客户", contact
assert contact["employeeId"] == 1 and contact["status"] == 1, contact

assert store["code"] == 200 and store["data"] == [], store
created = (work / "contact_message_batch_send_created.tsv").read_text(encoding="utf-8").strip().split("\t")
assert created == ["1", "1", "1", "1"], created
created_employee = (work / "contact_message_batch_send_created_employee.tsv").read_text(encoding="utf-8").strip().split("\t")
assert created_employee[0] == "1" and created_employee[1] == "1" and created_employee[2] == "go-room-tag-pull-msgid" and created_employee[3] == "1", created_employee
assert (work / "contact_message_batch_send_created_result_count.txt").read_text(encoding="utf-8").strip() == "1"
assert remind["code"] == 200 and remind["data"] == [], remind
assert destroy["code"] == 200 and destroy["data"] == [], destroy
assert (work / "contact_message_batch_send_deleted.tsv").read_text(encoding="utf-8").strip() == "1"
assert (work / "contact_message_batch_send_deleted_employee_count.txt").read_text(encoding="utf-8").strip() == "0"
assert (work / "contact_message_batch_send_deleted_result_count.txt").read_text(encoding="utf-8").strip() == "0"

requests = [json.loads(line) for line in request_log.read_text(encoding="utf-8").splitlines() if line.strip()]
assert any("/cgi-bin/externalcontact/add_msg_template" in item["path"] and "Go迁移客户群发新增" in item["body"] and "external-user-900001" in item["body"] for item in requests), requests
assert any("/cgi-bin/message/send" in item["path"] and "客户群发任务" in item["body"] and "go-migrate-user" in item["body"] for item in requests), requests
PY
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/roomMessageBatchSend/index?batchTitle=Go&page=1&perPage=10" >"$WORK_DIR/go_room_message_batch_send_index.json"
php_room_message_batch_send_index_code="$(curl -sS -o "$WORK_DIR/php_room_message_batch_send_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/roomMessageBatchSend/index?batchTitle=Go&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/roomMessageBatchSend/show?batchId=900001" >"$WORK_DIR/go_room_message_batch_send_show.json"
php_room_message_batch_send_show_code="$(curl -sS -o "$WORK_DIR/php_room_message_batch_send_show.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/roomMessageBatchSend/show?batchId=900001")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/roomMessageBatchSend/roomOwnerSendIndex?batchId=900001&sendStatus=1&page=1&perPage=15" >"$WORK_DIR/go_room_message_batch_send_owner_index.json"
php_room_message_batch_send_owner_index_code="$(curl -sS -o "$WORK_DIR/php_room_message_batch_send_owner_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/roomMessageBatchSend/roomOwnerSendIndex?batchId=900001&sendStatus=1&page=1&perPage=15")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/roomMessageBatchSend/roomReceiveIndex?batchId=900001&sendStatus=1&keyWords=Go&page=1&perPage=15" >"$WORK_DIR/go_room_message_batch_send_room_index.json"
php_room_message_batch_send_room_index_code="$(curl -sS -o "$WORK_DIR/php_room_message_batch_send_room_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/roomMessageBatchSend/roomReceiveIndex?batchId=900001&sendStatus=1&keyWords=Go&page=1&perPage=15")"
room_message_batch_send_store_payload='{"batchTitle":"Go迁移客户群群发新增","employeeIds":[1],"content":[{"msgType":"text","content":"Go迁移客户群群发新增"},{"msgType":"image","pic_url":"medium/go-media.png"}],"sendWay":1}'
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$room_message_batch_send_store_payload" "http://$GO_ADDR/dashboard/roomMessageBatchSend/store" >"$WORK_DIR/go_room_message_batch_send_store.json"
room_message_batch_send_created_id="$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT id FROM mc_room_message_batch_send WHERE batch_title = 'Go迁移客户群群发新增' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT send_status, send_employee_total, send_room_total, not_received_total FROM mc_room_message_batch_send WHERE id = ${room_message_batch_send_created_id};" >"$WORK_DIR/room_message_batch_send_created.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT status, receive_status, msg_id, send_time IS NOT NULL FROM mc_room_message_batch_send_employee WHERE batch_id = ${room_message_batch_send_created_id};" >"$WORK_DIR/room_message_batch_send_created_employee.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_room_message_batch_send_result WHERE batch_id = ${room_message_batch_send_created_id} AND room_id = 900001 AND chat_id = 'go-room-900001';" >"$WORK_DIR/room_message_batch_send_created_result_count.txt"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/roomMessageBatchSend/remind?batchId=900001&batchEmployId=1" >"$WORK_DIR/go_room_message_batch_send_remind.json"
curl -sS -f -X DELETE -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d '{"batchId":900002}' "http://$GO_ADDR/dashboard/roomMessageBatchSend/destroy" >"$WORK_DIR/go_room_message_batch_send_destroy.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT deleted_at IS NOT NULL FROM mc_room_message_batch_send WHERE id = 900002;" >"$WORK_DIR/room_message_batch_send_deleted.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_room_message_batch_send_employee WHERE batch_id = 900002;" >"$WORK_DIR/room_message_batch_send_deleted_employee_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_room_message_batch_send_result WHERE batch_id = 900002;" >"$WORK_DIR/room_message_batch_send_deleted_result_count.txt"
python3 - "$WORK_DIR" "$WORK_DIR/wecom_requests.ndjson" "$php_room_message_batch_send_index_code" "$php_room_message_batch_send_show_code" "$php_room_message_batch_send_owner_index_code" "$php_room_message_batch_send_room_index_code" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
request_log = pathlib.Path(sys.argv[2])
codes = sys.argv[3:]
if any(code != "200" for code in codes):
    raise SystemExit(f"unexpected roomMessageBatchSend PHP status codes: {codes}")

def load(name):
    return json.loads((work / name).read_text(encoding="utf-8"))

go_index = load("go_room_message_batch_send_index.json")
php_index = load("php_room_message_batch_send_index.json")
go_show = load("go_room_message_batch_send_show.json")
php_show = load("php_room_message_batch_send_show.json")
go_owner = load("go_room_message_batch_send_owner_index.json")
php_owner = load("php_room_message_batch_send_owner_index.json")
go_room = load("go_room_message_batch_send_room_index.json")
php_room = load("php_room_message_batch_send_room_index.json")
store = load("go_room_message_batch_send_store.json")
remind = load("go_room_message_batch_send_remind.json")
destroy = load("go_room_message_batch_send_destroy.json")

assert go_index["code"] == 200 and php_index["code"] == 200, (go_index, php_index)
item = next(item for item in go_index["data"]["list"] if item["id"] == 900001)
assert item["batchTitle"] == "Go迁移客户群群发", item
assert item["sendWay"] == 1 and item["sendStatus"] == 1, item
assert item["content"][0]["content"] == "Go迁移客户群群发文本", item
assert item["content"][1]["pic_url"] == "http://127.0.0.1:9501/static/medium/go-media.png", item

assert go_show["code"] == 200 and php_show["code"] == 200, (go_show, php_show)
show = go_show["data"]
assert show["batchTitle"] == "Go迁移客户群群发" and show["creator"] == "Go迁移测试用户", show
assert show["seedRooms"][0]["name"] == "Go迁移客户群", show
assert show["sendEmployeeTotal"] == 1 and show["sendRoomTotal"] == 1, show

assert go_owner["code"] == 200 and php_owner["code"] == 200, (go_owner, php_owner)
owner = go_owner["data"]["list"][0]
assert owner["employeeId"] == 1 and owner["employeeName"] == "Go迁移员工", owner
assert owner["status"] == 1 and owner["sendRoomTotal"] == 1 and owner["sendSuccessTotal"] == 1, owner

assert go_room["code"] == 200 and php_room["code"] == 200, (go_room, php_room)
room = go_room["data"]["list"][0]
assert room["roomId"] == 900001 and room["roomName"] == "Go迁移客户群", room
assert room["employeeId"] == 1 and room["status"] == 1 and room["roomEmployeeNum"] == 1, room

assert store["code"] == 200 and store["data"] == [], store
created = (work / "room_message_batch_send_created.tsv").read_text(encoding="utf-8").strip().split("\t")
assert created == ["1", "1", "1", "1"], created
created_employee = (work / "room_message_batch_send_created_employee.tsv").read_text(encoding="utf-8").strip().split("\t")
assert created_employee[0] == "1" and created_employee[1] == "1" and created_employee[2] == "go-room-tag-pull-msgid" and created_employee[3] == "1", created_employee
assert (work / "room_message_batch_send_created_result_count.txt").read_text(encoding="utf-8").strip() == "1"
assert remind["code"] == 200 and remind["data"] == [], remind
assert destroy["code"] == 200 and destroy["data"] == [], destroy
assert (work / "room_message_batch_send_deleted.tsv").read_text(encoding="utf-8").strip() == "1"
assert (work / "room_message_batch_send_deleted_employee_count.txt").read_text(encoding="utf-8").strip() == "0"
assert (work / "room_message_batch_send_deleted_result_count.txt").read_text(encoding="utf-8").strip() == "0"

requests = [json.loads(line) for line in request_log.read_text(encoding="utf-8").splitlines() if line.strip()]
assert any("/cgi-bin/externalcontact/add_msg_template" in item["path"] and "Go迁移客户群群发新增" in item["body"] and "go-room-900001" in item["body"] and '"chat_type":"group"' in item["body"].replace(" ", "") for item in requests), requests
assert any("/cgi-bin/message/send" in item["path"] and "客户群群发任务" in item["body"] and "go-migrate-user" in item["body"] for item in requests), requests
PY
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/officialAccount/index" >"$WORK_DIR/go_official_account_index.json"
php_official_account_index_code="$(curl -sS -o "$WORK_DIR/php_official_account_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/officialAccount/index")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/officialAccount/index?type=2" >"$WORK_DIR/go_official_account_index_type2.json"
php_official_account_index_type2_code="$(curl -sS -o "$WORK_DIR/php_official_account_index_type2.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/officialAccount/index?type=2")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/officialAccount/set?type=3&official_account_id=92" >"$WORK_DIR/go_official_account_set.json"
php_official_account_set_code="$(curl -sS -o "$WORK_DIR/php_official_account_set.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/officialAccount/set?type=3&official_account_id=92")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/officialAccount/index?type=3" >"$WORK_DIR/go_official_account_index_type3.json"
php_official_account_index_type3_code="$(curl -sS -o "$WORK_DIR/php_official_account_index_type3.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/officialAccount/index?type=3")"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT official_account_id FROM mc_official_account_set WHERE corp_id = 1 AND type = 3 AND deleted_at IS NULL ORDER BY id DESC LIMIT 1;" >"$WORK_DIR/official_account_set_type3.tsv"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/officialAccount/getPreAuthUrl" >"$WORK_DIR/go_official_account_get_pre_auth_url.json"
official_account_auth_redirect_code="$(curl -sS -D "$WORK_DIR/go_official_account_auth_redirect.headers" -o "$WORK_DIR/go_official_account_auth_redirect.body" -w '%{http_code}' "http://$GO_ADDR/dashboard/officialAccount/authRedirect/?auth_code=auth-code-go&corp_id=1")"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT appid, authorized_status, authorizer_appid, authorization_code, encoding_aes_key, token, secret, nickname, avatar, service_type_info, verify_type_info, user_name, principal_name, alias, qrcode_url, COALESCE(func_info, ''), JSON_EXTRACT(business_info, '$.open_pay') FROM mc_official_account WHERE authorizer_appid = 'official-authorizer-93' AND corp_id = 1 AND deleted_at IS NULL ORDER BY id DESC LIMIT 1;" >"$WORK_DIR/official_account_auth_redirect_created.tsv"
auth_event_authorized_xml='<xml><AppId><![CDATA[component-appid]]></AppId><CreateTime>1710000600</CreateTime><InfoType><![CDATA[authorized]]></InfoType><AuthorizerAppid><![CDATA[official-authorizer-event]]></AuthorizerAppid><AuthorizationCode><![CDATA[event-auth-code]]></AuthorizationCode><PreAuthCode><![CDATA[event-pre-auth-code]]></PreAuthCode></xml>'
auth_event_unauthorized_xml='<xml><AppId><![CDATA[component-appid]]></AppId><CreateTime>1710000700</CreateTime><InfoType><![CDATA[unauthorized]]></InfoType><AuthorizerAppid><![CDATA[official-authorizer-event]]></AuthorizerAppid></xml>'
curl -sS -f -X POST -H 'Content-Type: text/xml' --data-binary "$auth_event_authorized_xml" "http://$GO_ADDR/dashboard/officialAccount/authEventCallback" >"$WORK_DIR/go_official_account_auth_event_authorized.txt"
curl -sS -f -X POST -H 'Content-Type: text/xml' --data-binary "$auth_event_unauthorized_xml" "http://$GO_ADDR/dashboard/officialAccount/authEventCallback" >"$WORK_DIR/go_official_account_auth_event_unauthorized.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT appid, authorized_status, authorizer_appid, authorization_code, pre_auth_code, encoding_aes_key, token, secret, create_time FROM mc_official_account WHERE authorizer_appid = 'official-authorizer-event' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1;" >"$WORK_DIR/official_account_auth_event.tsv"
message_event_text_xml='<xml><ToUserName><![CDATA[official-authorizer-93]]></ToUserName><FromUserName><![CDATA[from-openid-normal]]></FromUserName><CreateTime>1710000800</CreateTime><MsgType><![CDATA[text]]></MsgType><Content><![CDATA[你好]]></Content><MsgId>1</MsgId></xml>'
message_event_case_xml='<xml><ToUserName><![CDATA[gh_3c884a361561]]></ToUserName><FromUserName><![CDATA[from-openid-case]]></FromUserName><CreateTime>1710000801</CreateTime><MsgType><![CDATA[text]]></MsgType><Content><![CDATA[TESTCOMPONENT_MSG_TYPE_TEXT]]></Content><MsgId>2</MsgId></xml>'
message_event_query_auth_xml='<xml><ToUserName><![CDATA[gh_3c884a361561]]></ToUserName><FromUserName><![CDATA[from-openid-query]]></FromUserName><CreateTime>1710000802</CreateTime><MsgType><![CDATA[text]]></MsgType><Content><![CDATA[QUERY_AUTH_CODE:query-code-go]]></Content><MsgId>3</MsgId></xml>'
curl -sS -f -X POST -H 'Content-Type: text/xml' --data-binary "$message_event_text_xml" "http://$GO_ADDR/dashboard/official-authorizer-93/officialAccount/messageEventCallback" >"$WORK_DIR/go_official_account_message_text.txt"
curl -sS -f -X POST -H 'Content-Type: text/xml' --data-binary "$message_event_case_xml" "http://$GO_ADDR/dashboard/official-authorizer-93/officialAccount/messageEventCallback" >"$WORK_DIR/go_official_account_message_case.txt"
message_event_query_code="$(curl -sS -X POST -H 'Content-Type: text/xml' --data-binary "$message_event_query_auth_xml" -o "$WORK_DIR/go_official_account_message_query.txt" -w '%{http_code}' "http://$GO_ADDR/dashboard/official-authorizer-93/officialAccount/messageEventCallback")"
load_oauth_cookie="$WORK_DIR/load_oauth.cookie"
load_oauth_redirect_code="$(curl -sS -D "$WORK_DIR/go_load_auth_redirect.headers" -o "$WORK_DIR/go_load_auth_redirect.body" -w '%{http_code}' -c "$load_oauth_cookie" "http://$GO_ADDR/load?target=%2Flegacy-load&corp_id=1")"
load_oauth_callback_code="$(curl -sS -D "$WORK_DIR/go_load_auth_callback.headers" -o "$WORK_DIR/go_load_auth_callback.body" -w '%{http_code}' -b "$load_oauth_cookie" -c "$load_oauth_cookie" "http://$GO_ADDR/load?target=%2Flegacy-load&code=oauth-code&appid=official-authorizer-91")"
python3 - "$WORK_DIR" "$WORK_DIR/wecom_requests.ndjson" "$official_account_auth_redirect_code" "$message_event_query_code" "$load_oauth_redirect_code" "$load_oauth_callback_code" "$php_official_account_index_code" "$php_official_account_index_type2_code" "$php_official_account_set_code" "$php_official_account_index_type3_code" <<'PY'
import json
import os
import pathlib
import sys
from urllib.parse import parse_qs, urlparse

work = pathlib.Path(sys.argv[1])
request_log = pathlib.Path(sys.argv[2])
auth_redirect_code = sys.argv[3]
message_event_query_code = sys.argv[4]
load_oauth_redirect_code = sys.argv[5]
load_oauth_callback_code = sys.argv[6]
codes = sys.argv[7:]
if auth_redirect_code != "302":
    raise SystemExit(f"unexpected officialAccount authRedirect code: {auth_redirect_code}")
if message_event_query_code != "200":
    raise SystemExit(f"unexpected messageEventCallback QUERY_AUTH_CODE status: {message_event_query_code}")
if load_oauth_redirect_code != "302" or load_oauth_callback_code != "302":
    raise SystemExit(f"unexpected /load oauth status: {load_oauth_redirect_code}, {load_oauth_callback_code}")
if codes != ["200", "200", "200", "200"]:
    raise SystemExit(f"unexpected officialAccount PHP status codes: {codes}")

def load(name):
    return json.loads((work / name).read_text(encoding="utf-8"))

def location(name):
    for line in (work / name).read_text(encoding="utf-8").splitlines():
        if line.lower().startswith("location:"):
            return line.split(":", 1)[1].strip()
    return ""

go_index = load("go_official_account_index.json")
php_index = load("php_official_account_index.json")
go_type2 = load("go_official_account_index_type2.json")
php_type2 = load("php_official_account_index_type2.json")
go_set = load("go_official_account_set.json")
php_set = load("php_official_account_set.json")
go_type3 = load("go_official_account_index_type3.json")
php_type3 = load("php_official_account_index_type3.json")
go_pre_auth = load("go_official_account_get_pre_auth_url.json")

assert go_index["code"] == 200 and php_index["code"] == 200, (go_index, php_index)
go_names = {item["nickname"] for item in go_index["data"]}
php_names = {item["nickname"] for item in php_index["data"]}
assert {"Go迁移公众号", "Go迁移备用公众号"} <= go_names, go_index
assert {"Go迁移公众号", "Go迁移备用公众号"} <= php_names, php_index
assert go_type2["code"] == 200 and php_type2["code"] == 200, (go_type2, php_type2)
assert go_type2["data"]["id"] == php_type2["data"]["id"] == 91, (go_type2, php_type2)
assert go_set["code"] == 200 and go_set["data"] == [], go_set
assert php_set["code"] == 200 and php_set["data"] == [], php_set
assert go_type3["code"] == 200 and php_type3["code"] == 200, (go_type3, php_type3)
assert go_type3["data"]["id"] == php_type3["data"]["id"] == 92, (go_type3, php_type3)
assert (work / "official_account_set_type3.tsv").read_text(encoding="utf-8").strip() == "92"
assert go_pre_auth["code"] == 200, go_pre_auth
pre_auth_url = go_pre_auth["data"]["url"]
pre_auth_query = parse_qs(urlparse(pre_auth_url).query)
assert pre_auth_url.startswith("https://mp.weixin.qq.com/cgi-bin/componentloginpage?"), pre_auth_url
assert pre_auth_query["component_appid"] == ["component-appid"], pre_auth_query
assert pre_auth_query["pre_auth_code"] == ["pre-auth-code-go"], pre_auth_query
assert pre_auth_query["auth_type"] == ["3"], pre_auth_query
assert pre_auth_query["redirect_uri"] == ["http://127.0.0.1:9501/authRedirect?corp_id=1"], pre_auth_query
assert location("go_official_account_auth_redirect.headers") == "http://127.0.0.1:9501/officialAccount/index"
created = (work / "official_account_auth_redirect_created.tsv").read_text(encoding="utf-8").strip().split("\t")
assert created[:8] == ["component-appid", "1", "official-authorizer-93", "auth-code-go", "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG", "component-token-for-callback", "component-secret", "Go迁移授权回跳公众号"], created
assert created[8] == "https://wechat.example/head-93.png" and created[9:15] == ["2", "0", "gh_go93", "Go迁移授权主体", "go-official-93", "https://wechat.example/qrcode-93.png"], created
assert "funcscope_category" in created[15] and '"id":1' in created[15].replace(" ", ""), created
assert created[16] == "1", created
assert (work / "go_official_account_auth_event_authorized.txt").read_text(encoding="utf-8") == "success"
assert (work / "go_official_account_auth_event_unauthorized.txt").read_text(encoding="utf-8") == "success"
event = (work / "official_account_auth_event.tsv").read_text(encoding="utf-8").strip().split("\t")
assert event == ["component-appid", "3", "official-authorizer-event", "event-auth-code", "event-pre-auth-code", "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG", "component-token-for-callback", "component-secret", "1710000700"], event
assert (work / "go_official_account_message_text.txt").read_text(encoding="utf-8") == "Hello！"
assert (work / "go_official_account_message_case.txt").read_text(encoding="utf-8") == "TESTCOMPONENT_MSG_TYPE_TEXT_callback"
assert (work / "go_official_account_message_query.txt").read_text(encoding="utf-8") == ""
load_auth_url = location("go_load_auth_redirect.headers")
load_auth_query = parse_qs(urlparse(load_auth_url).query)
operation_base = f"http://127.0.0.1:{os.environ.get('MOCHAT_PHP_PORT', '19501')}"
assert load_auth_url.startswith("https://open.weixin.qq.com/connect/oauth2/authorize?"), load_auth_url
assert load_auth_query["appid"] == ["official-authorizer-91"], load_auth_query
assert load_auth_query["component_appid"] == ["component-appid"], load_auth_query
assert load_auth_query["redirect_uri"] == [operation_base + "/load?target=" + operation_base.replace(":", "%3A").replace("/", "%2F") + "%2Flegacy-load&official_account_id=91"], load_auth_query
assert location("go_load_auth_callback.headers") == operation_base + "/legacy-load"
requests = [json.loads(line) for line in request_log.read_text(encoding="utf-8").splitlines() if line.strip()]
assert any("/cgi-bin/component/api_create_preauthcode" in item["path"] and "component-appid" in item["body"] for item in requests), requests
assert any("/cgi-bin/component/api_query_auth" in item["path"] and "auth-code-go" in item["body"] for item in requests), requests
assert any("/cgi-bin/component/api_get_authorizer_info" in item["path"] and "official-authorizer-93" in item["body"] for item in requests), requests
assert any("/cgi-bin/component/api_query_auth" in item["path"] and "query-code-go" in item["body"] for item in requests), requests
assert any("/cgi-bin/component/api_authorizer_token" in item["path"] and "authorizer-refresh-token-93" in item["body"] for item in requests), requests
assert any("/cgi-bin/message/custom/send" in item["path"] and "query-code-go_from_api" in item["body"] and "from-openid-query" in item["body"] for item in requests), requests
PY
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workFission/index?active_name=Go&page=1&perPage=10" >"$WORK_DIR/go_work_fission_index.json"
php_work_fission_index_code="$(curl -sS -o "$WORK_DIR/php_work_fission_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workFission/index?active_name=Go&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workFission/show?id=900001" >"$WORK_DIR/go_work_fission_show.json"
php_work_fission_show_code="$(curl -sS -o "$WORK_DIR/php_work_fission_show.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workFission/show?id=900001")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workFission/info?id=900001" >"$WORK_DIR/go_work_fission_info.json"
php_work_fission_info_code="$(curl -sS -o "$WORK_DIR/php_work_fission_info.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workFission/info?id=900001")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workFission/statistics?fission_ids=%5B900001%5D" >"$WORK_DIR/go_work_fission_statistics.json"
php_work_fission_statistics_code="$(curl -sS -o "$WORK_DIR/php_work_fission_statistics.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workFission/statistics?fission_ids=%5B900001%5D")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workFission/chooseContact?employee_ids=%5B1%5D&is_all=1&start_time=2026-07-01&end_time=2037-01-01&gender=1" >"$WORK_DIR/go_work_fission_choose_contact.json"
php_work_fission_choose_contact_code="$(curl -sS -o "$WORK_DIR/php_work_fission_choose_contact.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workFission/chooseContact?employee_ids=%5B1%5D&is_all=1&start_time=2026-07-01&end_time=2037-01-01&gender=1")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workFission/inviteData?fission_ids=%5B900001%5D&status=0&loss=1&page=1&perPage=10" >"$WORK_DIR/go_work_fission_invite_data.json"
php_work_fission_invite_data_code="$(curl -sS -o "$WORK_DIR/php_work_fission_invite_data.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workFission/inviteData?fission_ids=%5B900001%5D&status=0&loss=1&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workFission/inviteDetail?id=900001" >"$WORK_DIR/go_work_fission_invite_detail.json"
php_work_fission_invite_detail_code="$(curl -sS -o "$WORK_DIR/php_work_fission_invite_detail.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/workFission/inviteDetail?id=900001")"
work_fission_invite_payload='{"fission_id":900001,"text":"Go迁移裂变邀请新增","link_title":"裂变邀请标题新增","link_desc":"裂变邀请描述新增","link_pic":"work-fission/invite-new.png","filter":{"employee_ids":[1],"is_all":1,"start_time":"2026-07-01","end_time":"2037-01-01","gender":1}}'
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$work_fission_invite_payload" "http://$GO_ADDR/dashboard/workFission/invite" >"$WORK_DIR/go_work_fission_invite.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT text, link_title, link_desc, link_pic FROM mc_work_fission_invite WHERE fission_id = 900001 AND deleted_at IS NULL ORDER BY id DESC LIMIT 1;" >"$WORK_DIR/work_fission_invite_updated.tsv"
work_fission_store_payload='{"fission":{"active_name":"Go迁移裂变新增","service_employees":[{"id":1,"name":"Go迁移员工","wxUserId":"go-migrate-user"}],"auto_pass":true,"auto_add_tag":true,"contact_tags":[{"id":900001,"name":"Go迁移标签"}],"end_time":"2037-03-01 00:00:00","qr_code_invalid":7,"tasks":[{"count":2,"prize":"新增奖励"}],"new_friend":true,"delete_invalid":false,"receive_prize":0,"receive_prize_employees":[{"id":1,"wxUserId":"go-migrate-user"}],"receive_links":[{"url":"https://gift.example/store"}]},"welcome":{"msg_text":"Go迁移裂变欢迎新增","link_title":"欢迎新增标题","link_desc":"欢迎新增描述","link_cover_url":"data:image/jpeg;base64,aGVsbG8gd29ybGQ="},"poster":{"poster_type":1,"cover_pic":"data:image/jpeg;base64,aGVsbG8gd29ybGQ=","foward_text":"Go迁移裂变转发新增","avatar_show":true,"nickname_show":true,"nickname_color":"#123456","card_corp_image_name":"Go形象新增","card_corp_name":"Go企业新增","card_corp_logo":"data:image/jpeg;base64,aGVsbG8gd29ybGQ=","qrcode_w":"120","qrcode_h":"120","qrcode_x":"10","qrcode_y":"20"},"push":{"push_employee":true,"push_contact":true,"msg_text":"Go迁移裂变推送新增","msg_complex":{"msg_complex_type":"image","image":"data:image/jpeg;base64,aGVsbG8gd29ybGQ="}},"invite":{"text":"Go迁移裂变客户邀请新增","link_title":"客户邀请标题新增","link_desc":"客户邀请描述新增","link_pic":"data:image/jpeg;base64,aGVsbG8gd29ybGQ="}}'
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$work_fission_store_payload" "http://$GO_ADDR/dashboard/workFission/store" >"$WORK_DIR/go_work_fission_store.json"
work_fission_created_id="$(python3 - "$WORK_DIR/go_work_fission_store.json" <<'PY'
import json
import pathlib
import sys
payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print(int(payload["data"][0]))
PY
)"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT f.active_name, w.msg_text, p.foward_text, pu.msg_text, i.text, JSON_UNQUOTE(JSON_EXTRACT(f.receive_qrcode, '$.qrId')) FROM mc_work_fission f JOIN mc_work_fission_welcome w ON w.fission_id=f.id AND w.deleted_at IS NULL JOIN mc_work_fission_poster p ON p.fission_id=f.id AND p.deleted_at IS NULL JOIN mc_work_fission_push pu ON pu.fission_id=f.id AND pu.deleted_at IS NULL JOIN mc_work_fission_invite i ON i.fission_id=f.id AND i.deleted_at IS NULL WHERE f.id = $work_fission_created_id;" >"$WORK_DIR/work_fission_store_created.tsv"
work_fission_update_payload="$(cat <<JSON
{"fission":{"id":$work_fission_created_id,"active_name":"Go迁移裂变更新","service_employees":[{"id":1,"name":"Go迁移员工","wxUserId":"go-migrate-user"}],"auto_pass":false,"auto_add_tag":false,"contact_tags":[],"end_time":"2037-04-01 00:00:00","qr_code_invalid":3,"tasks":[{"count":5,"prize":"更新奖励"}],"receive_prize":1,"receive_prize_employees":[],"receive_links":[{"url":"https://gift.example/update"}]},"welcome":{"msg_text":"Go迁移裂变欢迎更新","link_title":"欢迎更新标题","link_desc":"欢迎更新描述","link_cover_url":"data:image/jpeg;base64,aGVsbG8gd29ybGQ="},"poster":{"poster_type":0,"cover_pic":"","foward_text":"Go迁移裂变转发更新","avatar_show":false,"nickname_show":true,"nickname_color":"#654321","card_corp_image_name":"","card_corp_name":"","card_corp_logo":"","qrcode_w":"90","qrcode_h":"90","qrcode_x":"1","qrcode_y":"2"},"push":{"push_employee":false,"push_contact":true,"msg_text":"Go迁移裂变推送更新","msg_complex_type":"","msg_complex":{}}}
JSON
)"
curl -sS -f -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$work_fission_update_payload" "http://$GO_ADDR/dashboard/workFission/update" >"$WORK_DIR/go_work_fission_update.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT f.active_name, w.msg_text, p.foward_text, pu.msg_text, f.receive_prize FROM mc_work_fission f JOIN mc_work_fission_welcome w ON w.fission_id=f.id AND w.deleted_at IS NULL JOIN mc_work_fission_poster p ON p.fission_id=f.id AND p.deleted_at IS NULL JOIN mc_work_fission_push pu ON pu.fission_id=f.id AND pu.deleted_at IS NULL WHERE f.id = $work_fission_created_id;" >"$WORK_DIR/work_fission_update_updated.tsv"
curl -sS -f -X DELETE -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d '{"id":900002}' "http://$GO_ADDR/dashboard/workFission/destroy" >"$WORK_DIR/go_work_fission_destroy.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT deleted_at IS NOT NULL FROM mc_work_fission WHERE id = 900002;" >"$WORK_DIR/work_fission_deleted.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT SUM(deleted_at IS NOT NULL) FROM mc_work_fission_poster WHERE fission_id = 900002;" >"$WORK_DIR/work_fission_poster_deleted.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT SUM(deleted_at IS NOT NULL) FROM mc_work_fission_welcome WHERE fission_id = 900002;" >"$WORK_DIR/work_fission_welcome_deleted.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT SUM(deleted_at IS NOT NULL) FROM mc_work_fission_push WHERE fission_id = 900002;" >"$WORK_DIR/work_fission_push_deleted.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT SUM(deleted_at IS NOT NULL) FROM mc_work_fission_invite WHERE fission_id = 900002;" >"$WORK_DIR/work_fission_invite_deleted.tsv"
work_fission_oauth_cookie="$WORK_DIR/work_fission_oauth.cookie"
operation_auth_redirect_code="$(curl -sS -D "$WORK_DIR/go_operation_work_fission_auth_redirect.headers" -o "$WORK_DIR/go_operation_work_fission_auth_redirect.body" -w '%{http_code}' -c "$work_fission_oauth_cookie" -b "$work_fission_oauth_cookie" "http://$GO_ADDR/operation/auth/workFission?id=900001&target=%2FworkFission%3Fid%3D900001")"
operation_auth_callback_code="$(curl -sS -D "$WORK_DIR/go_operation_work_fission_auth_callback.headers" -o "$WORK_DIR/go_operation_work_fission_auth_callback.body" -w '%{http_code}' -c "$work_fission_oauth_cookie" -b "$work_fission_oauth_cookie" "http://$GO_ADDR/operation/auth/workFission?id=900001&target=%2FworkFission%3Fid%3D900001&code=oauth-code&appid=official-authorizer-91")"
curl -sS -f -b "$work_fission_oauth_cookie" "http://$GO_ADDR/operation/openUserInfo/workFission?id=900001" >"$WORK_DIR/go_operation_work_fission_open_user_info.json"
curl -sS -f "http://$GO_ADDR/operation/openUserInfo/workFission?id=900001" >"$WORK_DIR/go_operation_work_fission_open_user_info_empty.json"
curl -sS -f "http://$GO_ADDR/operation/workFission/taskData?union_id=union-fission-parent&fission_id=900001" >"$WORK_DIR/go_operation_work_fission_task_data.json"
curl -sS -f "http://$GO_ADDR/operation/workFission/inviteFriends?union_id=union-fission-parent&fission_id=900001" >"$WORK_DIR/go_operation_work_fission_invite_friends.json"
curl -sS -f -G \
  --data-urlencode "fission_id=900001" \
  --data-urlencode "union_id=union-fission-parent" \
  --data-urlencode "nickname=裂变父客户" \
  --data-urlencode "avatar=https://avatar.example/parent.png" \
  "http://$GO_ADDR/operation/workFission/poster" >"$WORK_DIR/go_operation_work_fission_poster_existing.json"
curl -sS -f -X PUT -H 'Content-Type: application/json' -d '{"union_id":"union-fission-parent","fission_id":900001,"level":2}' "http://$GO_ADDR/operation/workFission/receive" >"$WORK_DIR/go_operation_work_fission_receive.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT receive_level FROM mc_work_fission_contact WHERE id = 900001;" >"$WORK_DIR/work_fission_receive_level.tsv"
curl -sS -f -G \
  --data-urlencode "fission_id=900001" \
  --data-urlencode "union_id=union-fission-new" \
  --data-urlencode "nickname=裂变新客户" \
  --data-urlencode "avatar=https://avatar.example/new.png" \
  "http://$GO_ADDR/operation/workFission/poster" >"$WORK_DIR/go_operation_work_fission_poster_new.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT level, external_user_id, qrcode_id, qrcode_url FROM mc_work_fission_contact WHERE fission_id = 900001 AND union_id = 'union-fission-new' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1;" >"$WORK_DIR/work_fission_operation_new_contact.tsv"
python3 - "$WORK_DIR" "$WORK_DIR/wecom_requests.ndjson" "$MOCHAT_PHP_PORT" "$operation_auth_redirect_code" "$operation_auth_callback_code" "$php_work_fission_index_code" "$php_work_fission_show_code" "$php_work_fission_info_code" "$php_work_fission_statistics_code" "$php_work_fission_choose_contact_code" "$php_work_fission_invite_data_code" "$php_work_fission_invite_detail_code" <<'PY'
import json
import pathlib
import sys
from urllib.parse import parse_qs, unquote, urlparse

work = pathlib.Path(sys.argv[1])
request_log = pathlib.Path(sys.argv[2])
php_port = sys.argv[3]
operation_auth_redirect_code = sys.argv[4]
operation_auth_callback_code = sys.argv[5]
codes = sys.argv[6:]
if operation_auth_redirect_code != "302" or operation_auth_callback_code != "302":
    raise SystemExit(f"unexpected workFission OAuth status codes: redirect={operation_auth_redirect_code}, callback={operation_auth_callback_code}")
if any(code != "200" for code in codes):
    raise SystemExit(f"unexpected workFission PHP status codes: {codes}")

def load(name):
    return json.loads((work / name).read_text(encoding="utf-8"))

def location(name):
    for line in (work / name).read_text(encoding="utf-8").splitlines():
        if line.lower().startswith("location:"):
            return line.split(":", 1)[1].strip()
    return ""

index = load("go_work_fission_index.json")
show = load("go_work_fission_show.json")
info = load("go_work_fission_info.json")
statistics = load("go_work_fission_statistics.json")
choose = load("go_work_fission_choose_contact.json")
invite_data = load("go_work_fission_invite_data.json")
invite_detail = load("go_work_fission_invite_detail.json")
invite = load("go_work_fission_invite.json")
store = load("go_work_fission_store.json")
update = load("go_work_fission_update.json")
destroy = load("go_work_fission_destroy.json")
operation_task_data = load("go_operation_work_fission_task_data.json")
operation_invite_friends = load("go_operation_work_fission_invite_friends.json")
operation_poster_existing = load("go_operation_work_fission_poster_existing.json")
operation_receive = load("go_operation_work_fission_receive.json")
operation_poster_new = load("go_operation_work_fission_poster_new.json")
operation_open_user_info = load("go_operation_work_fission_open_user_info.json")
operation_open_user_info_empty = load("go_operation_work_fission_open_user_info_empty.json")

assert index["code"] == 200, index
item = next(item for item in index["data"]["list"] if item["id"] == 900001)
assert item["active_name"] == "Go迁移裂变活动", item
assert item["status"] == "进行中" and item["finance_tag"] == "1/2", item
assert item["employeeNum"] == 2 and "Go奖励" in item["tasks"], item

assert show["code"] == 200, show
show_data = show["data"]
assert show_data["id"] == 900001 and show_data["qrcode_url"] == "https://qr.example/fission.png", show_data
expected_link = f"http://127.0.0.1:{php_port}/auth/workFission?id=900001&target=%2FworkFission%3Fid%3D900001"
assert show_data["link"] == expected_link, show_data
assert show_data["welcome_text"] == "欢迎参加Go裂变" and show_data["welcome_url"] == "work-fission/welcome.png", show_data

assert info["code"] == 200, info
info_data = info["data"]
assert info_data["fission"]["auto_pass"] == "true" and info_data["fission"]["auto_add_tag"] == "true", info_data
assert info_data["welcome"]["link_cover_url"] == "http://127.0.0.1:9501/static/work-fission/welcome.png", info_data
assert info_data["poster"]["cover_pic"] == "http://127.0.0.1:9501/static/work-fission/poster.png", info_data
assert info_data["poster"]["avatar_show"] == "true" and info_data["poster"]["nickname_show"] == "true", info_data
assert info_data["push"]["msg_complex"]["image"] == "http://127.0.0.1:9501/static/work-fission/push.png", info_data
assert info_data["invite"]["link_pic"] == "http://127.0.0.1:9501/static/work-fission/invite.png", info_data

assert statistics["code"] == 200, statistics
user = statistics["data"]["user"]
assert user["user_count"] == 2 and user["loss_count"] == 1 and user["insert_count"] == 1, user
assert user["new_increase_count"] == 2 and user["new_loss_count"] == 1 and user["net_increase"] == 1, user
assert user["invite_count"] == 2 and user["fission_rate"] == "50.00%" and user["insert_rate"] == "50.00%" and user["share_rate"] == "100.00%", user
assert statistics["data"]["level"] == {"first_level": 1, "second_level": 1, "third_level": 0}, statistics
assert any(active["active_name"] == "Go迁移裂变活动" for active in statistics["data"]["active"]), statistics
assert statistics["data"]["employee"][0]["name"] == "Go迁移员工", statistics

assert choose["code"] == 200 and choose["data"][0] >= 1, choose

assert invite_data["code"] == 200, invite_data
invite_item = invite_data["data"]["list"][0]
assert invite_item["id"] == 900002 and invite_item["active_name"] == "Go迁移裂变活动", invite_item
assert invite_item["employees"] == "Go迁移员工" and invite_item["loss"] == "已流失" and invite_item["status"] == "未完成", invite_item
assert invite_item["contact_id"] == 900002 and invite_item["employee_id"] == 1, invite_item

assert invite_detail["code"] == 200, invite_detail
detail = invite_detail["data"]
assert detail["total_count"] == 2 and detail["new_count"] == 1 and detail["loss"] == 1 and detail["insert"] == 1, detail
assert detail["user_list"][0]["id"] == 900002 and detail["user_list"][0]["nickname"] == "裂变子客户", detail

assert invite["code"] == 200 and invite["data"] == [], invite
updated_invite = (work / "work_fission_invite_updated.tsv").read_text(encoding="utf-8").strip().split("\t")
assert updated_invite == ["Go迁移裂变邀请新增", "裂变邀请标题新增", "裂变邀请描述新增", "work-fission/invite-new.png"], updated_invite
assert store["code"] == 200 and int(store["data"][0]) > 0, store
created_row = (work / "work_fission_store_created.tsv").read_text(encoding="utf-8").strip().split("\t")
assert created_row[:5] == ["Go迁移裂变新增", "Go迁移裂变欢迎新增", "Go迁移裂变转发新增", "Go迁移裂变推送新增", "Go迁移裂变客户邀请新增"], created_row
assert created_row[5] == "go-channel-code-config-created", created_row
assert update["code"] == 200 and update["data"] == [], update
updated_row = (work / "work_fission_update_updated.tsv").read_text(encoding="utf-8").strip().split("\t")
assert updated_row == ["Go迁移裂变更新", "Go迁移裂变欢迎更新", "Go迁移裂变转发更新", "Go迁移裂变推送更新", "1"], updated_row
requests = [json.loads(line) for line in request_log.read_text(encoding="utf-8").splitlines() if line.strip()]
assert any("/cgi-bin/externalcontact/add_msg_template" in item["path"] and "Go迁移裂变邀请新增" in item["body"] and "external-user-900001" in item["body"] and "auth/workFission" in item["body"] for item in requests), requests
assert any("/cgi-bin/externalcontact/add_contact_way" in item["path"] and "go-migrate-user" in item["body"] for item in requests), requests
assert sum(1 for item in requests if "/cgi-bin/media/uploadimg" in item["path"]) >= 3, requests
assert any("/cgi-bin/component/api_component_token" in item["path"] and "component-ticket" in item["body"] for item in requests), requests
assert any("/sns/oauth2/component/access_token" in item["path"] and "oauth-code" in item["path"] for item in requests), requests
assert any("/sns/userinfo" in item["path"] and "openid-work-fission" in item["path"] for item in requests), requests

assert destroy["code"] == 200 and destroy["data"] == [], destroy
for name in [
    "work_fission_deleted.tsv",
    "work_fission_poster_deleted.tsv",
    "work_fission_welcome_deleted.tsv",
    "work_fission_push_deleted.tsv",
    "work_fission_invite_deleted.tsv",
]:
    assert (work / name).read_text(encoding="utf-8").strip() == "1", name

oauth_location = location("go_operation_work_fission_auth_redirect.headers")
assert oauth_location.startswith("https://open.weixin.qq.com/connect/oauth2/authorize?"), oauth_location
oauth_query = parse_qs(urlparse(oauth_location).query)
assert oauth_query["appid"] == ["official-authorizer-91"], oauth_query
assert oauth_query["component_appid"] == ["component-appid"], oauth_query
assert oauth_query["scope"] == ["snsapi_userinfo"], oauth_query
assert oauth_query["response_type"] == ["code"], oauth_query
redirect_uri = oauth_query["redirect_uri"][0]
assert redirect_uri.startswith(f"http://127.0.0.1:{php_port}/auth/workFission?target="), redirect_uri
assert unquote(parse_qs(urlparse(redirect_uri).query)["target"][0]) == f"http://127.0.0.1:{php_port}/workFission?id=900001", redirect_uri
callback_location = location("go_operation_work_fission_auth_callback.headers")
assert callback_location == f"http://127.0.0.1:{php_port}/workFission?id=900001", callback_location
assert operation_open_user_info["code"] == 200, operation_open_user_info
wechat_user = operation_open_user_info["data"]
assert wechat_user["openid"] == "openid-work-fission" and wechat_user["unionid"] == "union-fission-parent", wechat_user
assert wechat_user["nickname"] == "裂变微信用户", wechat_user
assert operation_open_user_info_empty["code"] == 200 and operation_open_user_info_empty["data"] == [], operation_open_user_info_empty

assert operation_task_data["code"] == 200, operation_task_data
task_data = operation_task_data["data"]
assert task_data["invite_count"] == 1 and task_data["differ_count"] == 1 and task_data["end_time"] > 0, task_data
assert task_data["task"][0]["count"] == 2 and task_data["task"][0]["status"] == 0, task_data
assert task_data["task"][0]["receive_status"] == 0 and task_data["task"][0]["gift_type"] == 1, task_data
assert task_data["task"][0]["gift_url"] == "https://example.com/prize", task_data

assert operation_invite_friends["code"] == 200, operation_invite_friends
friends = operation_invite_friends["data"]
assert len(friends) == 1 and friends[0]["id"] == 900002, friends
assert friends[0]["nickname"] == "裂变子客户" and friends[0]["unionId"] == "union-fission-child", friends
assert friends[0]["fail"] == 0 and friends[0]["createdAt"] == "2026-07-03 12:05:00", friends

assert operation_poster_existing["code"] == 200, operation_poster_existing
existing_poster = operation_poster_existing["data"]
assert existing_poster["posterType"] == 1 and existing_poster["qrcodeUrl"] == "https://qr.example/parent.png", existing_poster
assert existing_poster["coverPic"] == "http://127.0.0.1:9501/static/work-fission/poster.png", existing_poster
assert existing_poster["cardCorpLogo"] == "http://127.0.0.1:9501/static/work-fission/logo.png", existing_poster

assert operation_receive["code"] == 200 and operation_receive["data"] == [], operation_receive
assert (work / "work_fission_receive_level.tsv").read_text(encoding="utf-8").strip() == "2"

assert operation_poster_new["code"] == 200, operation_poster_new
new_poster = operation_poster_new["data"]
assert new_poster["qrcodeUrl"] == "https://wecom.example/channel-code-created.png", new_poster
new_contact = (work / "work_fission_operation_new_contact.tsv").read_text(encoding="utf-8").strip().split("\t")
assert new_contact == ["1", "external-user-fission-new", "https://wecom.example/channel-code-created.png", "https://wecom.example/channel-code-created.png"], new_contact
assert any("/cgi-bin/externalcontact/add_contact_way" in item["path"] and "fission-" in item["body"] and "go-migrate-user" in item["body"] for item in requests), requests
PY
curl -sS -f -H "Authorization: Bearer $sidebar_token" "http://$GO_ADDR/sidebar/medium/mediaIdUpdate?mediumId=900001" >"$WORK_DIR/go_sidebar_medium_media_id_update.json"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT media_id, last_upload_time > 0 FROM mc_medium WHERE id = 900001;" >"$WORK_DIR/sidebar_medium_after_update.tsv"
curl -sS -f -H "Authorization: Bearer $sidebar_token" "http://$GO_ADDR/sidebar/workRoom/roomManage?roomId=go-room-900001" >"$WORK_DIR/go_sidebar_work_room_manage.json"
php_sidebar_work_room_manage_code="$(curl -sS -o "$WORK_DIR/php_sidebar_work_room_manage.json" -w '%{http_code}' -H "Authorization: Bearer $sidebar_token" "http://127.0.0.1:$MOCHAT_PHP_PORT/sidebar/workRoom/roomManage?roomId=go-room-900001")"
python3 - "$WORK_DIR/go_sidebar_work_room_manage.json" "$WORK_DIR/php_sidebar_work_room_manage.json" "$php_sidebar_work_room_manage_code" <<'PY'
import json
import sys

go = json.load(open(sys.argv[1], encoding="utf-8"))
php = json.load(open(sys.argv[2], encoding="utf-8"))
php_code = sys.argv[3]
assert php_code == "200", (php_code, php)
assert go["code"] == 200 and php["code"] == 200, (go, php)
assert go["data"] == php["data"] == [], (go, php)
PY
curl -sS -f -H "Authorization: Bearer $sidebar_token" "http://$GO_ADDR/sidebar/workContactTag/allTag?groupId=900001" >"$WORK_DIR/go_sidebar_work_contact_tag_all.json"
php_sidebar_work_contact_tag_all_code="$(curl -sS -o "$WORK_DIR/php_sidebar_work_contact_tag_all.json" -w '%{http_code}' -H "Authorization: Bearer $sidebar_token" "http://127.0.0.1:$MOCHAT_PHP_PORT/sidebar/workContactTag/allTag?groupId=900001")"
curl -sS -f -H "Authorization: Bearer $sidebar_token" "http://$GO_ADDR/sidebar/workContact/detail?wxExternalUserid=external-user-900001" >"$WORK_DIR/go_sidebar_work_contact_detail.json"
php_sidebar_work_contact_detail_code="$(curl -sS -o "$WORK_DIR/php_sidebar_work_contact_detail.json" -w '%{http_code}' -H "Authorization: Bearer $sidebar_token" "http://127.0.0.1:$MOCHAT_PHP_PORT/sidebar/workContact/detail?wxExternalUserid=external-user-900001")"
curl -sS -f -H "Authorization: Bearer $sidebar_token" "http://$GO_ADDR/sidebar/workContact/show?contactId=900001" >"$WORK_DIR/go_sidebar_work_contact_show.json"
php_sidebar_work_contact_show_code="$(curl -sS -o "$WORK_DIR/php_sidebar_work_contact_show.json" -w '%{http_code}' -H "Authorization: Bearer $sidebar_token" "http://127.0.0.1:$MOCHAT_PHP_PORT/sidebar/workContact/show?contactId=900001")"
curl -sS -f -H "Authorization: Bearer $sidebar_token" "http://$GO_ADDR/sidebar/workContact/track?contactId=900001" >"$WORK_DIR/go_sidebar_work_contact_track.json"
php_sidebar_work_contact_track_code="$(curl -sS -o "$WORK_DIR/php_sidebar_work_contact_track.json" -w '%{http_code}' -H "Authorization: Bearer $sidebar_token" "http://127.0.0.1:$MOCHAT_PHP_PORT/sidebar/workContact/track?contactId=900001")"
curl -sS -f -H "Authorization: Bearer $sidebar_token" "http://$GO_ADDR/sidebar/contactProcessStatus/index" >"$WORK_DIR/go_sidebar_contact_process_status_index.json"
php_sidebar_contact_process_status_index_code="$(curl -sS -o "$WORK_DIR/php_sidebar_contact_process_status_index.json" -w '%{http_code}' -H "Authorization: Bearer $sidebar_token" "http://127.0.0.1:$MOCHAT_PHP_PORT/sidebar/contactProcessStatus/index")"
process_status_update_payload="$(python3 - "$WORK_DIR/go_sidebar_contact_process_status_index.json" <<'PY'
import json
import sys

body = json.load(open(sys.argv[1], encoding="utf-8"))
for item in body.get("data") or []:
    if item.get("name") == "意向客户":
        print(json.dumps({"contactId": 900001, "statusId": item["id"]}, ensure_ascii=False))
        break
else:
    raise SystemExit("missing 意向客户 process status")
PY
)"
curl -sS -f -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $sidebar_token" -d "$process_status_update_payload" "http://$GO_ADDR/sidebar/contactProcessStatus/update" >"$WORK_DIR/go_sidebar_contact_process_status_update.json"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT follow_up_status FROM mc_work_contact WHERE id = 900001;" >"$WORK_DIR/contact_process_status_after_update.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT event, employee_id, corp_id, content FROM mc_contact_employee_track WHERE contact_id = 900001 AND event = 5 ORDER BY id DESC LIMIT 1;" >"$WORK_DIR/contact_process_track_after_update.tsv"
contact_field_pivot_update_payload='{"contactId":900001,"userPortrait":"[{\"contactFieldPivotId\":900001,\"contactFieldId\":900001,\"name\":\"Go迁移字段\",\"type\":3,\"value\":\"选项B\"}]"}'
curl -sS -f -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$contact_field_pivot_update_payload" "http://$GO_ADDR/dashboard/contactFieldPivot/update" >"$WORK_DIR/go_contact_field_pivot_update.json"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT value FROM mc_contact_field_pivot WHERE id = 900001;" >"$WORK_DIR/contact_field_pivot_value_after_dashboard_update.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT event, employee_id, corp_id, content FROM mc_contact_employee_track WHERE contact_id = 900001 AND event = 4 ORDER BY id DESC LIMIT 1;" >"$WORK_DIR/contact_field_pivot_track_after_dashboard_update.tsv"
sidebar_contact_field_pivot_update_payload='{"contactId":900001,"userPortrait":"[{\"contactFieldPivotId\":900003,\"contactFieldId\":900003,\"name\":\"Go迁移多选字段\",\"type\":2,\"value\":[\"多选B\"]}]"}'
curl -sS -f -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $sidebar_token" -d "$sidebar_contact_field_pivot_update_payload" "http://$GO_ADDR/sidebar/contactFieldPivot/update" >"$WORK_DIR/go_sidebar_contact_field_pivot_update.json"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT value FROM mc_contact_field_pivot WHERE id = 900003;" >"$WORK_DIR/contact_field_pivot_value_after_sidebar_update.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT event, employee_id, corp_id, content FROM mc_contact_employee_track WHERE contact_id = 900001 AND event = 4 ORDER BY id DESC LIMIT 1;" >"$WORK_DIR/contact_field_pivot_track_after_sidebar_update.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -e "INSERT INTO mc_work_contact_tag (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at) VALUES (900002, 'go-contact-tag-update', 1, 'Go迁移更新标签', 2, 900001, NOW(), NOW()) ON DUPLICATE KEY UPDATE wx_contact_tag_id=VALUES(wx_contact_tag_id), corp_id=VALUES(corp_id), name=VALUES(name), \`order\`=VALUES(\`order\`), contact_tag_group_id=VALUES(contact_tag_group_id), updated_at=NOW(), deleted_at=NULL;"
work_contact_update_payload='{"contactId":900001,"employeeId":1,"remark":"Go迁移备注更新","description":"Go迁移描述更新","businessNo":"GO-UPDATED","tag":[900001,900002]}'
curl -sS -f -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$work_contact_update_payload" "http://$GO_ADDR/dashboard/workContact/update" >"$WORK_DIR/go_work_contact_update.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT remark, description FROM mc_work_contact_employee WHERE contact_id = 900001 AND employee_id = 1 AND deleted_at IS NULL;" >"$WORK_DIR/work_contact_employee_after_update.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT business_no FROM mc_work_contact WHERE id = 900001 AND deleted_at IS NULL;" >"$WORK_DIR/work_contact_after_update.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact_tag_pivot WHERE contact_id = 900001 AND employee_id = 1 AND contact_tag_id = 900002 AND deleted_at IS NULL;" >"$WORK_DIR/work_contact_update_tag_pivot_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT event, employee_id, corp_id, content FROM mc_contact_employee_track WHERE contact_id = 900001 AND content IN ('修改用户资料：备注', '修改用户资料：描述', '修改用户资料：客户编号', '系统对该客户打标签【Go迁移更新标签】') ORDER BY content;" >"$WORK_DIR/work_contact_update_tracks.tsv"
work_contact_batch_labeling_payload='{"contactId":"900001,900002","tagId":"900001,900002"}'
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$work_contact_batch_labeling_payload" "http://$GO_ADDR/dashboard/workContact/batchLabeling" >"$WORK_DIR/go_work_contact_batch_labeling.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact_tag_pivot WHERE contact_id = 900002 AND employee_id = 1 AND contact_tag_id = 900002 AND deleted_at IS NULL;" >"$WORK_DIR/work_contact_batch_labeling_pivot_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -e "INSERT INTO mc_work_contact_tag (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at) VALUES (900003, 'go-contact-tag-sidebar', 1, 'Go迁移侧边栏标签', 3, 900001, NOW(), NOW()) ON DUPLICATE KEY UPDATE wx_contact_tag_id=VALUES(wx_contact_tag_id), corp_id=VALUES(corp_id), name=VALUES(name), \`order\`=VALUES(\`order\`), contact_tag_group_id=VALUES(contact_tag_group_id), updated_at=NOW(), deleted_at=NULL;"
sidebar_work_contact_update_payload='{"contactId":900001,"employeeId":2,"remark":"Go迁移侧边栏备注更新","description":"Go迁移侧边栏描述更新","businessNo":"GO-SIDEBAR","tag":[900003]}'
curl -sS -f -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $sidebar_token" -d "$sidebar_work_contact_update_payload" "http://$GO_ADDR/sidebar/workContact/update" >"$WORK_DIR/go_sidebar_work_contact_update.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT remark, description FROM mc_work_contact_employee WHERE contact_id = 900001 AND employee_id = 1 AND deleted_at IS NULL;" >"$WORK_DIR/work_contact_employee_after_sidebar_update.tsv"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT business_no FROM mc_work_contact WHERE id = 900001 AND deleted_at IS NULL;" >"$WORK_DIR/work_contact_after_sidebar_update.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact_tag_pivot WHERE contact_id = 900001 AND employee_id = 1 AND contact_tag_id = 900003 AND deleted_at IS NULL;" >"$WORK_DIR/work_contact_sidebar_update_tag_pivot_count.txt"
work_room_batch_update_payload='{"workRoomIds":"900001","workRoomGroupId":900001}'
curl -sS -f -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "$work_room_batch_update_payload" "http://$GO_ADDR/dashboard/workRoom/batchUpdate" >"$WORK_DIR/go_work_room_batch_update.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT room_group_id FROM mc_work_room WHERE id = 900001 AND deleted_at IS NULL;" >"$WORK_DIR/work_room_group_after_batch_update.txt"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/workRoom/index?workRoomName=%E8%BF%81%E7%A7%BB&page=1&perPage=10" >"$WORK_DIR/go_work_room_index_after_batch_update.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactField/index?status=1&page=1&perPage=10" >"$WORK_DIR/go_contact_field_index.json"
php_contact_field_index_code="$(curl -sS -o "$WORK_DIR/php_contact_field_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/contactField/index?status=1&page=1&perPage=10")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactField/show?id=900001" >"$WORK_DIR/go_contact_field_show.json"
php_contact_field_show_code="$(curl -sS -o "$WORK_DIR/php_contact_field_show.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/contactField/show?id=900001")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactField/portrait" >"$WORK_DIR/go_contact_field_portrait.json"
php_contact_field_portrait_code="$(curl -sS -o "$WORK_DIR/php_contact_field_portrait.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/contactField/portrait")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/contactFieldPivot/index?contactId=900001" >"$WORK_DIR/go_contact_field_pivot_index.json"
php_contact_field_pivot_index_code="$(curl -sS -o "$WORK_DIR/php_contact_field_pivot_index.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/contactFieldPivot/index?contactId=900001")"
curl -sS -f -H "Authorization: Bearer $sidebar_token" "http://$GO_ADDR/sidebar/contactFieldPivot/index?contactId=900001" >"$WORK_DIR/go_sidebar_contact_field_pivot_index.json"
php_sidebar_contact_field_pivot_index_code="$(curl -sS -o "$WORK_DIR/php_sidebar_contact_field_pivot_index.json" -w '%{http_code}' -H "Authorization: Bearer $sidebar_token" "http://127.0.0.1:$MOCHAT_PHP_PORT/sidebar/contactFieldPivot/index?contactId=900001")"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/role/permissionByUser" >"$WORK_DIR/go_permission.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/role/select" >"$WORK_DIR/go_role_select.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/role/index?page=1&perPage=10" >"$WORK_DIR/go_role_index.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/role/show?roleId=1" >"$WORK_DIR/go_role_show.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/role/permissionShow?roleId=1" >"$WORK_DIR/go_role_permission_show.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/role/showEmployee?roleId=1&page=1&perPage=10" >"$WORK_DIR/go_role_show_employee.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/menu/iconIndex" >"$WORK_DIR/go_menu_icon_index.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/menu/select" >"$WORK_DIR/go_menu_select.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/menu/index?page=1&perPage=10" >"$WORK_DIR/go_menu_index.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/menu/show?menuId=1" >"$WORK_DIR/go_menu_show.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/chatTool/config" >"$WORK_DIR/go_chat_tool_config.json"
curl -sS -f "http://$GO_ADDR/WW_verify_ABCDEF1234567890.txt" >"$WORK_DIR/go_txt_verify.txt"
printf '%s' 'ABCDEF1234567890' >"$WORK_DIR/verify.txt"
curl -sS -f -F 'file=@'"$WORK_DIR/verify.txt"';type=text/plain;filename=WW_verify_ABCDEF1234567890.txt' "http://$GO_ADDR/dashboard/agent/txtVerifyUpload" >"$WORK_DIR/go_txt_verify_upload.json"
curl -sS -f -H "Authorization: Bearer $normal_token" "http://$GO_ADDR/dashboard/corp/select" >"$WORK_DIR/go_normal_corp_select.json"
normal_forbidden_code="$(curl -sS -o "$WORK_DIR/go_normal_forbidden_bind.json" -w '%{http_code}' -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $normal_token" -d '{"corpId":2}' "http://$GO_ADDR/dashboard/corp/bind")"
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $normal_token" -d '{"corpId":1}' "http://$GO_ADDR/dashboard/corp/bind" >"$WORK_DIR/go_normal_corp_bind.json"
curl -sS -f -H "Authorization: Bearer $normal_token" "http://$GO_ADDR/dashboard/user/loginShow" >"$WORK_DIR/go_normal_login_after_bind.json"
normal_user_cache="$(compose exec -T redis redis-cli GET mc:user.2 || true)"
php_login_code="$(curl -sS -o "$WORK_DIR/php_login_show.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/user/loginShow")"
php_permission_code="$(curl -sS -o "$WORK_DIR/php_permission.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/role/permissionByUser")"
curl -sS -f -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d '{"corpId":1}' "http://$GO_ADDR/dashboard/corp/bind" >"$WORK_DIR/go_corp_bind.json"
curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/user/loginShow" >"$WORK_DIR/go_login_after_bind.json"
user_cache="$(compose exec -T redis redis-cli GET mc:user.1 || true)"
curl -sS -f -X PUT -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/user/logout" >"$WORK_DIR/go_logout.json"
after_go_code="$(curl -sS -o "$WORK_DIR/go_login_after_logout.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR/dashboard/user/loginShow")"
after_php_code="$(curl -sS -o "$WORK_DIR/php_login_after_logout.json" -w '%{http_code}' -H "Authorization: Bearer $token" "http://127.0.0.1:$MOCHAT_PHP_PORT/dashboard/user/loginShow")"
redis_keys="$(compose exec -T redis redis-cli --scan | sort | tr '\n' ',')"

python3 - "$WORK_DIR" "$php_room_tag_pull_index_code" "$php_room_tag_pull_show_code" "$php_room_tag_pull_show_contact_code" "$php_room_tag_pull_show_employee_code" "$php_room_tag_pull_room_list_code" "$php_room_tag_pull_choose_contact_code" "$php_room_tag_pull_filter_contact_code" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
codes = sys.argv[2:]
for code in codes:
    if code != "200":
        for name in (
            "php_room_tag_pull_index.json",
            "php_room_tag_pull_show.json",
            "php_room_tag_pull_show_contact.json",
            "php_room_tag_pull_show_employee.json",
            "php_room_tag_pull_room_list.json",
            "php_room_tag_pull_choose_contact.json",
            "php_room_tag_pull_filter_contact.json",
        ):
            path = work / name
            if path.exists():
                print(f"{name}: {path.read_text(encoding='utf-8', errors='replace')[:2000]}", file=sys.stderr)
        raise SystemExit(f"unexpected roomTagPull PHP status codes: {codes}")

def load(name):
    return json.loads((work / name).read_text(encoding="utf-8"))

go_index = load("go_room_tag_pull_index.json")
php_index = load("php_room_tag_pull_index.json")
go_show = load("go_room_tag_pull_show.json")
php_show = load("php_room_tag_pull_show.json")
go_contact = load("go_room_tag_pull_show_contact.json")
php_contact = load("php_room_tag_pull_show_contact.json")
go_employee = load("go_room_tag_pull_show_employee.json")
php_employee = load("php_room_tag_pull_show_employee.json")
go_rooms = load("go_room_tag_pull_room_list.json")
php_rooms = load("php_room_tag_pull_room_list.json")
go_choose = load("go_room_tag_pull_choose_contact.json")
php_choose = load("php_room_tag_pull_choose_contact.json")
go_filter = load("go_room_tag_pull_filter_contact.json")
php_filter = load("php_room_tag_pull_filter_contact.json")
store = load("go_room_tag_pull_store.json")
destroy = load("go_room_tag_pull_destroy.json")
sidebar_media = load("go_sidebar_medium_media_id_update.json")
created = (work / "room_tag_pull_created.tsv").read_text(encoding="utf-8").strip().split("\t")
created_contacts = (work / "room_tag_pull_created_contacts.tsv").read_text(encoding="utf-8").strip()
deleted = (work / "room_tag_pull_deleted.tsv").read_text(encoding="utf-8").strip()
medium_after = (work / "sidebar_medium_after_update.tsv").read_text(encoding="utf-8").strip().split("\t")

assert go_index["code"] == 200 and php_index["code"] == 200
go_index_item = next(item for item in go_index["data"]["list"] if item["id"] == 900001)
php_index_item = next(item for item in php_index["data"]["list"] if item["id"] == 900001)
for key in ("id", "name", "employees", "rooms", "invite_num", "join_room_num", "no_send_num", "no_invite_num", "created_at"):
    assert go_index_item[key] == php_index_item[key], (key, go_index_item, php_index_item)

assert go_show["code"] == 200 and php_show["code"] == 200
go_show_data = go_show["data"]
php_show_data = php_show["data"]
for key in ("join_room_num", "no_join_room_num", "invite_num", "no_invite_num", "send_num", "no_send_num"):
    assert go_show_data[key] == php_show_data[key], (key, go_show_data, php_show_data)
assert go_show_data["employees"][0]["wxUserId"] == php_show_data["employees"][0]["wxUserId"], (go_show_data, php_show_data)
assert go_show_data["rooms"][0]["id"] == php_show_data["rooms"][0]["id"], (go_show_data, php_show_data)
assert go_show_data["rooms"][0]["contact_num"] == php_show_data["rooms"][0]["contact_num"], (go_show_data, php_show_data)

assert go_contact["code"] == 200 and php_contact["code"] == 200
go_contact_item = go_contact["data"]["list"][0]
php_contact_item = php_contact["data"]["list"][0]
for key in ("contact_name", "employee_name", "send_status", "room_name", "is_join_room"):
    assert go_contact_item[key] == php_contact_item[key], (key, go_contact_item, php_contact_item)

assert go_employee["code"] == 200 and php_employee["code"] == 200
assert len(go_employee["data"]["list"]) == len(php_employee["data"]["list"]), (go_employee, php_employee)

assert go_rooms["code"] == 200 and php_rooms["code"] == 200
go_room = go_rooms["data"][0]
php_room = php_rooms["data"][0]
for key in ("id", "wxChatId", "name", "ownerId", "roomMax", "contact_num"):
    assert go_room[key] == php_room[key], (key, go_room, php_room)

assert go_choose["code"] == 200 and php_choose["code"] == 200
assert go_choose["data"] == php_choose["data"], (go_choose, php_choose)
assert go_filter["code"] == 200 and php_filter["code"] == 200
assert go_filter["data"] == php_filter["data"], (go_filter, php_filter)
assert store["code"] == 200 and store["data"][0] > 0, store
assert created[0] == "Go迁移标签建群新增" and created[1] == "1" and created[2] == "1", created
assert created_contacts == "1", created_contacts
assert destroy["code"] == 200, destroy
assert deleted == "1", deleted
assert sidebar_media["code"] == 200 and sidebar_media["data"]["mediaId"] == "go-sidebar-medium-media-id", sidebar_media
assert medium_after == ["go-sidebar-medium-media-id", "1"], medium_after
PY

python3 - "$WORK_DIR" "$php_login_code" "$php_permission_code" "$php_work_employee_index_code" "$php_work_contact_tag_group_index_code" "$php_work_contact_tag_group_detail_code" "$php_sidebar_work_contact_tag_group_index_code" "$php_work_contact_tag_index_code" "$php_work_contact_tag_detail_code" "$php_work_contact_tag_list_code" "$php_work_contact_tag_all_code" "$php_channel_code_group_index_code" "$php_channel_code_group_detail_code" "$php_work_contact_index_code" "$php_work_contact_loss_code" "$php_work_contact_source_code" "$php_work_contact_show_code" "$php_work_contact_track_code" "$php_work_contact_room_index_code" "$php_work_contact_room_index_status0_code" "$php_work_room_index_code" "$php_work_room_room_index_code" "$php_work_room_statistics_code" "$php_work_room_statistics_index_code" "$php_work_room_auto_pull_index_code" "$php_work_room_auto_pull_show_code" "$php_sidebar_work_contact_tag_all_code" "$php_sidebar_work_contact_detail_code" "$php_sidebar_work_contact_show_code" "$php_sidebar_work_contact_track_code" "$php_sidebar_contact_process_status_index_code" "$php_contact_field_index_code" "$php_contact_field_show_code" "$php_contact_field_portrait_code" "$php_contact_field_pivot_index_code" "$php_sidebar_contact_field_pivot_index_code" "$after_go_code" "$after_php_code" "$redis_keys" "$user_cache" "$normal_forbidden_code" "$normal_user_cache" "$channel_group_created_count" "$channel_group_updated_name" "$channel_code_moved_group" <<'PY'
import base64
import json
import pathlib
import sys
import urllib.parse
from datetime import date, timedelta

work = pathlib.Path(sys.argv[1])
php_login_code, php_permission_code, php_work_employee_index_code, php_work_contact_tag_group_index_code, php_work_contact_tag_group_detail_code, php_sidebar_work_contact_tag_group_index_code, php_work_contact_tag_index_code, php_work_contact_tag_detail_code, php_work_contact_tag_list_code, php_work_contact_tag_all_code, php_channel_code_group_index_code, php_channel_code_group_detail_code, php_work_contact_index_code, php_work_contact_loss_code, php_work_contact_source_code, php_work_contact_show_code, php_work_contact_track_code, php_work_contact_room_index_code, php_work_contact_room_index_status0_code, php_work_room_index_code, php_work_room_room_index_code, php_work_room_statistics_code, php_work_room_statistics_index_code, php_work_room_auto_pull_index_code, php_work_room_auto_pull_show_code, php_sidebar_work_contact_tag_all_code, php_sidebar_work_contact_detail_code, php_sidebar_work_contact_show_code, php_sidebar_work_contact_track_code, php_sidebar_contact_process_status_index_code, php_contact_field_index_code, php_contact_field_show_code, php_contact_field_portrait_code, php_contact_field_pivot_index_code, php_sidebar_contact_field_pivot_index_code, after_go_code, after_php_code, redis_keys, user_cache, normal_forbidden_code, normal_user_cache, channel_group_created_count, channel_group_updated_name, channel_code_moved_group = sys.argv[2:]
today = date.today().isoformat()
today_date = date.today()
yesterday_date = today_date - timedelta(days=1)
yesterday = yesterday_date.isoformat()
same_month_yesterday = yesterday_date.year == today_date.year and yesterday_date.month == today_date.month
last_month_end = today_date.replace(day=1) - timedelta(days=1)
yesterday_in_last_month = yesterday_date.year == last_month_end.year and yesterday_date.month == last_month_end.month
expected_month_add_friends = 5 + (4 if same_month_yesterday else 0)
expected_month_add_room = 2 + (1 if same_month_yesterday else 0)
expected_month_add_room_member = 3 + (2 if same_month_yesterday else 0)
expected_last_month_add_friends = 7 + (4 if yesterday_in_last_month else 0)
expected_last_month_add_room = 3 + (1 if yesterday_in_last_month else 0)
expected_last_month_add_room_member = 4 + (2 if yesterday_in_last_month else 0)

def load(name):
    return json.loads((work / name).read_text(encoding="utf-8"))

readyz = load("readyz.json")
go_auth = load("go_auth.json")
go_corp_select = load("go_corp_select.json")
go_corp_index = load("go_corp_index.json")
go_corp_show = load("go_corp_show.json")
go_corp_update = load("go_corp_update.json")
go_corp_show_after_update = load("go_corp_show_after_update.json")
go_login = load("go_login_show.json")
go_corp_data_index = load("go_corp_data_index.json")
go_corp_data_line_chat = load("go_corp_data_line_chat.json")
go_work_employee_index = load("go_work_employee_index.json")
php_work_employee_index = load("php_work_employee_index.json")
go_work_employee_search_condition = load("go_work_employee_search_condition.json")
go_work_department_index = load("go_work_department_index.json")
go_work_department_member_index = load("go_work_department_member_index.json")
go_work_department_select_by_phone = load("go_work_department_select_by_phone.json")
go_work_department_page_index = load("go_work_department_page_index.json")
go_work_department_show_employee = load("go_work_department_show_employee.json")
go_work_contact_tag_group_index = load("go_work_contact_tag_group_index.json")
php_work_contact_tag_group_index = load("php_work_contact_tag_group_index.json")
go_work_contact_tag_group_detail = load("go_work_contact_tag_group_detail.json")
php_work_contact_tag_group_detail = load("php_work_contact_tag_group_detail.json")
go_sidebar_work_contact_tag_group_index = load("go_sidebar_work_contact_tag_group_index.json")
php_sidebar_work_contact_tag_group_index = load("php_sidebar_work_contact_tag_group_index.json")
go_work_contact_tag_index = load("go_work_contact_tag_index.json")
php_work_contact_tag_index = load("php_work_contact_tag_index.json")
go_work_contact_tag_detail = load("go_work_contact_tag_detail.json")
php_work_contact_tag_detail = load("php_work_contact_tag_detail.json")
go_work_contact_tag_list = load("go_work_contact_tag_list.json")
php_work_contact_tag_list = load("php_work_contact_tag_list.json")
go_work_contact_tag_all = load("go_work_contact_tag_all.json")
php_work_contact_tag_all = load("php_work_contact_tag_all.json")
go_channel_code_index = load("go_channel_code_index.json")
php_channel_code_index = load("php_channel_code_index.json")
go_channel_code_show = load("go_channel_code_show.json")
php_channel_code_show = load("php_channel_code_show.json")
go_channel_code_contact = load("go_channel_code_contact.json")
php_channel_code_contact = load("php_channel_code_contact.json")
go_channel_code_statistics = load("go_channel_code_statistics.json")
php_channel_code_statistics = load("php_channel_code_statistics.json")
go_channel_code_statistics_index = load("go_channel_code_statistics_index.json")
php_channel_code_statistics_index = load("php_channel_code_statistics_index.json")
go_channel_code_group_index = load("go_channel_code_group_index.json")
php_channel_code_group_index = load("php_channel_code_group_index.json")
go_channel_code_group_detail = load("go_channel_code_group_detail.json")
php_channel_code_group_detail = load("php_channel_code_group_detail.json")
go_channel_code_group_store = load("go_channel_code_group_store.json")
go_channel_code_group_update = load("go_channel_code_group_update.json")
go_channel_code_group_move = load("go_channel_code_group_move.json")
go_channel_code_store = load("go_channel_code_store.json")
go_channel_code_update = load("go_channel_code_update.json")
go_work_contact_index = load("go_work_contact_index.json")
php_work_contact_index = load("php_work_contact_index.json")
go_work_contact_loss = load("go_work_contact_loss.json")
php_work_contact_loss = load("php_work_contact_loss.json")
go_work_contact_source = load("go_work_contact_source.json")
php_work_contact_source = load("php_work_contact_source.json")
go_work_contact_show = load("go_work_contact_show.json")
php_work_contact_show = load("php_work_contact_show.json")
go_work_contact_track = load("go_work_contact_track.json")
php_work_contact_track = load("php_work_contact_track.json")
go_work_contact_update = load("go_work_contact_update.json")
go_work_contact_batch_labeling = load("go_work_contact_batch_labeling.json")
go_sidebar_work_contact_update = load("go_sidebar_work_contact_update.json")
go_work_room_batch_update = load("go_work_room_batch_update.json")
go_work_room_index_after_batch_update = load("go_work_room_index_after_batch_update.json")
go_work_contact_room_index = load("go_work_contact_room_index.json")
php_work_contact_room_index = load("php_work_contact_room_index.json")
go_work_contact_room_index_status0 = load("go_work_contact_room_index_status0.json")
php_work_contact_room_index_status0 = load("php_work_contact_room_index_status0.json")
go_work_room_index = load("go_work_room_index.json")
php_work_room_index = load("php_work_room_index.json")
go_work_room_room_index = load("go_work_room_room_index.json")
php_work_room_room_index = load("php_work_room_room_index.json")
go_work_room_statistics = load("go_work_room_statistics.json")
php_work_room_statistics = load("php_work_room_statistics.json")
go_work_room_statistics_index = load("go_work_room_statistics_index.json")
php_work_room_statistics_index = load("php_work_room_statistics_index.json")
go_work_room_auto_pull_store = load("go_work_room_auto_pull_store.json")
go_work_room_auto_pull_update = load("go_work_room_auto_pull_update.json")
go_work_room_auto_pull_index = load("go_work_room_auto_pull_index.json")
php_work_room_auto_pull_index = load("php_work_room_auto_pull_index.json")
go_work_room_auto_pull_show = load("go_work_room_auto_pull_show.json")
php_work_room_auto_pull_show = load("php_work_room_auto_pull_show.json")
go_sidebar_work_contact_tag_all = load("go_sidebar_work_contact_tag_all.json")
php_sidebar_work_contact_tag_all = load("php_sidebar_work_contact_tag_all.json")
go_sidebar_work_contact_detail = load("go_sidebar_work_contact_detail.json")
php_sidebar_work_contact_detail = load("php_sidebar_work_contact_detail.json")
go_sidebar_work_contact_show = load("go_sidebar_work_contact_show.json")
php_sidebar_work_contact_show = load("php_sidebar_work_contact_show.json")
go_sidebar_work_contact_track = load("go_sidebar_work_contact_track.json")
php_sidebar_work_contact_track = load("php_sidebar_work_contact_track.json")
go_sidebar_contact_process_status_index = load("go_sidebar_contact_process_status_index.json")
php_sidebar_contact_process_status_index = load("php_sidebar_contact_process_status_index.json")
go_sidebar_contact_process_status_update = load("go_sidebar_contact_process_status_update.json")
go_contact_field_index = load("go_contact_field_index.json")
php_contact_field_index = load("php_contact_field_index.json")
go_contact_field_show = load("go_contact_field_show.json")
php_contact_field_show = load("php_contact_field_show.json")
go_contact_field_portrait = load("go_contact_field_portrait.json")
php_contact_field_portrait = load("php_contact_field_portrait.json")
go_contact_field_pivot_index = load("go_contact_field_pivot_index.json")
php_contact_field_pivot_index = load("php_contact_field_pivot_index.json")
go_sidebar_contact_field_pivot_index = load("go_sidebar_contact_field_pivot_index.json")
php_sidebar_contact_field_pivot_index = load("php_sidebar_contact_field_pivot_index.json")
go_contact_field_pivot_update = load("go_contact_field_pivot_update.json")
go_sidebar_contact_field_pivot_update = load("go_sidebar_contact_field_pivot_update.json")
go_permission = load("go_permission.json")
go_role_select = load("go_role_select.json")
go_role_index = load("go_role_index.json")
go_role_show = load("go_role_show.json")
go_role_permission_show = load("go_role_permission_show.json")
go_role_show_employee = load("go_role_show_employee.json")
go_menu_icon_index = load("go_menu_icon_index.json")
go_menu_select = load("go_menu_select.json")
go_menu_index = load("go_menu_index.json")
go_menu_show = load("go_menu_show.json")
go_chat_tool_config = load("go_chat_tool_config.json")
go_txt_verify_upload = load("go_txt_verify_upload.json")
go_agent_store = load("go_agent_store.json")
go_sidebar_agent_oauth_url = load("go_sidebar_agent_oauth_url.json")
go_sidebar_agent_oauth_token = load("go_sidebar_agent_oauth_token.json")
go_sidebar_agent_jssdk_config = load("go_sidebar_agent_jssdk_config.json")
go_sidebar_wx_jssdk_config = load("go_sidebar_wx_jssdk_config.json")
go_sidebar_wx_jssdk_corp_config = load("go_sidebar_wx_jssdk_corp_config.json")
go_normal_auth = load("go_normal_auth.json")
go_normal_corp_select = load("go_normal_corp_select.json")
go_normal_forbidden_bind = load("go_normal_forbidden_bind.json")
go_normal_corp_bind = load("go_normal_corp_bind.json")
go_normal_login_after_bind = load("go_normal_login_after_bind.json")
go_corp_bind = load("go_corp_bind.json")
go_login_after_bind = load("go_login_after_bind.json")
go_logout = load("go_logout.json")

assert readyz["php_upstream_ready"] is True, readyz
assert go_auth["code"] == 200 and go_auth["data"]["token"], go_auth
assert go_corp_select["code"] == 200 and go_corp_select["data"], go_corp_select
assert go_corp_index["code"] == 200 and go_corp_index["data"]["page"]["total"] >= 1, go_corp_index
assert 1 in [corp["corpId"] for corp in go_corp_index["data"]["list"]], go_corp_index
assert go_corp_show["code"] == 200 and go_corp_show["data"]["corpId"] == 1, go_corp_show
assert go_corp_show["data"]["eventCallback"].endswith("?cid=1"), go_corp_show
assert go_corp_update["code"] == 200, go_corp_update
assert go_corp_show_after_update["code"] == 200, go_corp_show_after_update
assert go_corp_show_after_update["data"]["corpName"] == "Go迁移更新企业", go_corp_show_after_update
assert go_corp_show_after_update["data"]["employeeSecret"] == "employee-secret-updated", go_corp_show_after_update
assert go_corp_show_after_update["data"]["contactSecret"] == "contact-secret-updated", go_corp_show_after_update
assert go_login["code"] == 200 and go_login["data"]["corpId"] == 1, go_login
assert go_login["data"]["employeeId"] == 1, go_login
assert go_corp_data_index["code"] == 200, go_corp_data_index
corp_data = go_corp_data_index["data"]
assert corp_data["addContactNum"] == 5, go_corp_data_index
assert corp_data["lastAddContactNum"] == 4, go_corp_data_index
assert corp_data["addIntoRoomNum"] == 3, go_corp_data_index
assert corp_data["lastAddIntoRoomNum"] == 2, go_corp_data_index
assert corp_data["lossContactNum"] == 1, go_corp_data_index
assert corp_data["quitRoomNum"] == 1, go_corp_data_index
assert corp_data["addFriendsNum"] == expected_month_add_friends, go_corp_data_index
assert corp_data["lastAddFriendsNum"] == expected_last_month_add_friends, go_corp_data_index
assert corp_data["monthAddRoomNum"] == expected_month_add_room, go_corp_data_index
assert corp_data["lastMonthAddRoomNum"] == expected_last_month_add_room, go_corp_data_index
assert corp_data["monthAddRoomMemberNum"] == expected_month_add_room_member, go_corp_data_index
assert corp_data["lastMonthAddRoomMemberNum"] == expected_last_month_add_room_member, go_corp_data_index
assert corp_data["monthLossContactNum"] == 1, go_corp_data_index
assert corp_data["lastMonthLossContactNum"] == 2, go_corp_data_index
assert corp_data["corpMemberNum"] >= 2, go_corp_data_index
assert corp_data["updateTime"] == "2026-07-02 12:00:00", go_corp_data_index
assert go_corp_data_line_chat["code"] == 200, go_corp_data_line_chat
points = go_corp_data_line_chat["data"]
assert any(item["date"].startswith(today) and item["addContactNum"] == 5 and item["addIntoRoomNum"] == 3 for item in points), go_corp_data_line_chat
assert any(item["date"].startswith(yesterday) and item["addContactNum"] == 4 and item["addIntoRoomNum"] == 2 for item in points), go_corp_data_line_chat
assert go_work_employee_index["code"] == 200, go_work_employee_index
employee_index = go_work_employee_index["data"]
assert employee_index["page"]["total"] >= 1, go_work_employee_index
assert any(item["id"] == 1 and item["name"] == "Go迁移员工" and item["statusName"] == "已激活" and item["contactAuthName"] == "否" and item["messageNums"] == 8 and item["sendMessageNums"] == 5 and item["replyMessageRatio"] == "0.75" and item["addNums"] == 3 and item["applyNums"] == 2 and item["invalidContact"] == 1 and item["averageReply"] == 10 for item in employee_index["list"]), go_work_employee_index
assert php_work_employee_index_code == "200", php_work_employee_index_code
assert php_work_employee_index["code"] == 200, php_work_employee_index
php_employee_index = php_work_employee_index["data"]
go_employee = next(item for item in employee_index["list"] if item["id"] == 1)
php_employee = next(item for item in php_employee_index["list"] if item["id"] == 1)
for key in ("id", "name", "status", "statusName", "contactAuth", "contactAuthName", "wxUserId", "corpId", "gender", "messageNums", "sendMessageNums", "replyMessageRatio", "addNums", "applyNums", "invalidContact", "averageReply"):
    assert go_employee[key] == php_employee[key], (key, go_employee, php_employee)
assert go_employee["thumbAvatar"] == php_employee["thumbAvatar"] == "", (go_employee, php_employee)
assert go_work_employee_search_condition["code"] == 200, go_work_employee_search_condition
search_condition = go_work_employee_search_condition["data"]
assert search_condition["syncTime"] == "2026-07-02 09:00:00", go_work_employee_search_condition
assert {"id": 1, "name": "已激活"} in search_condition["status"], go_work_employee_search_condition
assert {"id": 2, "name": "否"} in search_condition["contactAuth"], go_work_employee_search_condition
assert go_work_department_index["code"] == 200, go_work_department_index
department_data = go_work_department_index["data"]
assert any(dept["id"] == 900001 and dept["name"] == "Go迁移总部" and dept.get("son") for dept in department_data["department"]), go_work_department_index
assert any(employee["employeeId"] == 1 and employee["name"] == "Go迁移员工" for employee in department_data["employee"]), go_work_department_index
assert go_work_department_member_index["code"] == 200, go_work_department_member_index
member_data = go_work_department_member_index["data"]
assert any(item["employeeId"] == 1 and item["departmentId"] == 900001 and item["employeeName"] == "Go迁移员工" and item["departmentName"] == "Go迁移总部" for item in member_data), go_work_department_member_index
assert any(item["employeeId"] == 2 and item["departmentId"] == 900002 and item["employeeName"] == "Go迁移普通员工" and item["departmentName"] == "Go迁移销售部" for item in member_data), go_work_department_member_index
assert go_work_department_select_by_phone["code"] == 200, go_work_department_select_by_phone
assert {"corpId": 1, "workDepartmentId": 900001, "workDepartmentName": "Go迁移总部"} in go_work_department_select_by_phone["data"], go_work_department_select_by_phone
assert go_work_department_page_index["code"] == 200, go_work_department_page_index
department_page = go_work_department_page_index["data"]
assert department_page["page"]["total"] >= 1, go_work_department_page_index
assert any(item["departmentId"] == 900001 and item["level"] == "一级部门" and item.get("children") for item in department_page["list"]), go_work_department_page_index
assert go_work_department_show_employee["code"] == 200, go_work_department_show_employee
department_employees = go_work_department_show_employee["data"]
assert department_employees["page"]["total"] >= 1, go_work_department_show_employee
assert any(item["employeeId"] == 1 and item["employeeName"] == "Go迁移员工" and item["phone"] == "13800000000" and item["roleName"] == "Go迁移管理员" for item in department_employees["list"]), go_work_department_show_employee
assert go_work_contact_tag_group_index["code"] == 200, go_work_contact_tag_group_index
assert php_work_contact_tag_group_index_code == "200", php_work_contact_tag_group_index_code
assert php_work_contact_tag_group_index["code"] == 200, php_work_contact_tag_group_index
assert go_work_contact_tag_group_index["data"] == php_work_contact_tag_group_index["data"], (go_work_contact_tag_group_index, php_work_contact_tag_group_index)
assert {"groupId": 900001, "groupName": "Go迁移标签组"} in go_work_contact_tag_group_index["data"], go_work_contact_tag_group_index
assert {"groupId": 0, "groupName": "未分组"} in go_work_contact_tag_group_index["data"], go_work_contact_tag_group_index
assert go_work_contact_tag_group_detail["code"] == 200, go_work_contact_tag_group_detail
assert php_work_contact_tag_group_detail_code == "200", php_work_contact_tag_group_detail_code
assert php_work_contact_tag_group_detail["code"] == 200, php_work_contact_tag_group_detail
assert go_work_contact_tag_group_detail["data"] == php_work_contact_tag_group_detail["data"], (go_work_contact_tag_group_detail, php_work_contact_tag_group_detail)
assert go_work_contact_tag_group_detail["data"] == {"id": 900001, "groupName": "Go迁移标签组"}, go_work_contact_tag_group_detail
assert go_sidebar_work_contact_tag_group_index["code"] == 200, go_sidebar_work_contact_tag_group_index
assert php_sidebar_work_contact_tag_group_index_code == "200", php_sidebar_work_contact_tag_group_index_code
assert php_sidebar_work_contact_tag_group_index["code"] == 200, php_sidebar_work_contact_tag_group_index
assert go_sidebar_work_contact_tag_group_index["data"] == php_sidebar_work_contact_tag_group_index["data"], (go_sidebar_work_contact_tag_group_index, php_sidebar_work_contact_tag_group_index)
assert {"groupId": 900001, "groupName": "Go迁移标签组"} in go_sidebar_work_contact_tag_group_index["data"], go_sidebar_work_contact_tag_group_index
assert {"groupId": 0, "groupName": "未分组"} in go_sidebar_work_contact_tag_group_index["data"], go_sidebar_work_contact_tag_group_index
assert go_work_contact_tag_index["code"] == 200, go_work_contact_tag_index
assert php_work_contact_tag_index_code == "200", php_work_contact_tag_index_code
assert php_work_contact_tag_index["code"] == 200, php_work_contact_tag_index
assert go_work_contact_tag_index["data"] == php_work_contact_tag_index["data"], (go_work_contact_tag_index, php_work_contact_tag_index)
assert go_work_contact_tag_index["data"]["syncTagTime"] == "2026-07-02 10:00:00", go_work_contact_tag_index
assert go_work_contact_tag_index["data"]["page"]["perPage"] == 10, go_work_contact_tag_index
assert {"id": 900001, "name": "Go迁移标签", "contactNum": 3} in go_work_contact_tag_index["data"]["list"], go_work_contact_tag_index
assert go_work_contact_tag_detail["code"] == 200, go_work_contact_tag_detail
assert php_work_contact_tag_detail_code == "200", php_work_contact_tag_detail_code
assert php_work_contact_tag_detail["code"] == 200, php_work_contact_tag_detail
assert go_work_contact_tag_detail["data"] == php_work_contact_tag_detail["data"], (go_work_contact_tag_detail, php_work_contact_tag_detail)
assert go_work_contact_tag_detail["data"] == {"tagId": 900001, "tagName": "Go迁移标签", "groupId": 900001}, go_work_contact_tag_detail
assert go_work_contact_tag_list["code"] == 200, go_work_contact_tag_list
assert php_work_contact_tag_list_code == "200", php_work_contact_tag_list_code
assert php_work_contact_tag_list["code"] == 200, php_work_contact_tag_list
assert go_work_contact_tag_list["data"] == php_work_contact_tag_list["data"], (go_work_contact_tag_list, php_work_contact_tag_list)
expected_group = {
    "id": 900001,
    "wxGroupId": "go-tag-group",
    "groupName": "Go迁移标签组",
    "tags": [{"id": 900001, "wxContactTagId": "go-contact-tag", "name": "Go迁移标签", "contactTagGroupId": 900001}],
}
assert expected_group in go_work_contact_tag_list["data"], go_work_contact_tag_list
assert go_work_contact_tag_all["code"] == 200, go_work_contact_tag_all
assert php_work_contact_tag_all_code == "200", php_work_contact_tag_all_code
assert php_work_contact_tag_all["code"] == 200, php_work_contact_tag_all
assert go_work_contact_tag_all["data"] == php_work_contact_tag_all["data"], (go_work_contact_tag_all, php_work_contact_tag_all)
assert {"id": 900001, "name": "Go迁移标签"} in go_work_contact_tag_all["data"], go_work_contact_tag_all
assert go_channel_code_index["code"] == 200, go_channel_code_index
assert php_channel_code_index["code"] == 200, php_channel_code_index
go_channel_code_item = next(item for item in go_channel_code_index["data"]["list"] if item["channelCodeId"] == 900003)
php_channel_code_item = next(item for item in php_channel_code_index["data"]["list"] if item["channelCodeId"] == 900003)
for key in ["channelCodeId", "groupId", "groupName", "name", "autoAddFriend", "tags", "type", "contactNum"]:
    assert go_channel_code_item[key] == php_channel_code_item[key], (key, go_channel_code_item, php_channel_code_item)
assert go_channel_code_item["name"] == "Go迁移渠道码", go_channel_code_item
assert go_channel_code_item["tags"] == ["Go迁移标签"], go_channel_code_item
assert go_channel_code_show["code"] == 200, go_channel_code_show
assert php_channel_code_show["code"] == 200, php_channel_code_show
assert go_channel_code_show["data"]["baseInfo"] == php_channel_code_show["data"]["baseInfo"], (go_channel_code_show, php_channel_code_show)
assert go_channel_code_show["data"]["baseInfo"]["selectedTags"] == [900001], go_channel_code_show
assert go_channel_code_contact["code"] == 200, go_channel_code_contact
assert php_channel_code_contact["code"] == 200, php_channel_code_contact
go_channel_code_contact_item = go_channel_code_contact["data"]["list"][0]
php_channel_code_contact_item = php_channel_code_contact["data"]["list"][0]
for key in ["contactId", "employeeId", "createTime", "name", "employees"]:
    assert go_channel_code_contact_item[key] == php_channel_code_contact_item[key], (key, go_channel_code_contact_item, php_channel_code_contact_item)
assert go_channel_code_statistics["code"] == 200, go_channel_code_statistics
assert php_channel_code_statistics["code"] == 200, php_channel_code_statistics
for key in ["addNumLong", "defriendNumLong", "deleteNumLong", "netNumLong"]:
    assert go_channel_code_statistics["data"][key] == php_channel_code_statistics["data"][key], (key, go_channel_code_statistics, php_channel_code_statistics)
assert go_channel_code_statistics["data"]["list"] == php_channel_code_statistics["data"]["list"], (go_channel_code_statistics, php_channel_code_statistics)
assert go_channel_code_statistics_index["code"] == 200, go_channel_code_statistics_index
assert php_channel_code_statistics_index["code"] == 200, php_channel_code_statistics_index
assert go_channel_code_statistics_index["data"] == php_channel_code_statistics_index["data"], (go_channel_code_statistics_index, php_channel_code_statistics_index)
assert go_channel_code_group_index["code"] == 200, go_channel_code_group_index
assert php_channel_code_group_index_code == "200", php_channel_code_group_index_code
assert php_channel_code_group_index["code"] == 200, php_channel_code_group_index
assert go_channel_code_group_index["data"] == php_channel_code_group_index["data"], (go_channel_code_group_index, php_channel_code_group_index)
assert {"groupId": 900001, "name": "Go迁移渠道分组"} in go_channel_code_group_index["data"], go_channel_code_group_index
assert {"groupId": 0, "name": "未分组"} in go_channel_code_group_index["data"], go_channel_code_group_index
assert go_channel_code_group_detail["code"] == 200, go_channel_code_group_detail
assert php_channel_code_group_detail_code == "200", php_channel_code_group_detail_code
assert php_channel_code_group_detail["code"] == 200, php_channel_code_group_detail
assert go_channel_code_group_detail["data"] == php_channel_code_group_detail["data"], (go_channel_code_group_detail, php_channel_code_group_detail)
assert go_channel_code_group_detail["data"] == {"id": 900001, "name": "Go迁移渠道分组", "groupId": 900001}, go_channel_code_group_detail
assert go_channel_code_group_store["code"] == 200, go_channel_code_group_store
assert go_channel_code_group_update["code"] == 200, go_channel_code_group_update
assert go_channel_code_group_move["code"] == 200, go_channel_code_group_move
assert int(channel_group_created_count.strip()) == 1, channel_group_created_count
assert channel_group_updated_name.strip() == "Go迁移渠道分组更新", channel_group_updated_name
assert int(channel_code_moved_group.strip()) == 900001, channel_code_moved_group
assert go_channel_code_store["code"] == 200, go_channel_code_store
assert go_channel_code_update["code"] == 200, go_channel_code_update
channel_code_created = (work / "channel_code_created.tsv").read_text(encoding="utf-8").strip().split("\t")
assert channel_code_created[1:] == [
    "Go迁移渠道码更新",
    "https://wecom.example/channel-code-created.png",
    "go-channel-code-config-created",
    "2",
    "900001",
    "1",
    "[900001]",
], channel_code_created
channel_code_logs = [line.split("\t") for line in (work / "channel_code_business_logs.tsv").read_text(encoding="utf-8").strip().splitlines()]
assert any(row[0] == "100" and row[1] == "1" and row[2] == channel_code_created[0] for row in channel_code_logs), channel_code_logs
assert any(row[0] == "101" and row[1] == "1" and row[2] == channel_code_created[0] for row in channel_code_logs), channel_code_logs
assert go_work_contact_index["code"] == 200, go_work_contact_index
assert php_work_contact_index_code == "200", php_work_contact_index_code
assert php_work_contact_index["code"] == 200, php_work_contact_index
assert go_work_contact_index["data"]["page"] == php_work_contact_index["data"]["page"], (go_work_contact_index, php_work_contact_index)
assert go_work_contact_index["data"]["page"] == {"perPage": 20, "total": 2, "totalPage": 1}, go_work_contact_index
assert go_work_contact_index["data"]["syncContactTime"] == php_work_contact_index["data"]["syncContactTime"] == "2026-07-02 11:00:00", (go_work_contact_index, php_work_contact_index)
go_contact_item = next(item for item in go_work_contact_index["data"]["list"] if item["id"] == 900001)
php_contact_item = next(item for item in php_work_contact_index["data"]["list"] if item["id"] == 900001)
contact_index_keys = [
    "id", "employeeId", "contactId", "remark", "createTime", "addWay", "addWayText",
    "genderText", "businessNo", "name", "avatar", "gender", "roomName", "employeeName",
    "tag", "isContact",
]
assert {key: go_contact_item[key] for key in contact_index_keys} == {key: php_contact_item[key] for key in contact_index_keys}, (go_contact_item, php_contact_item)
assert go_contact_item["id"] == 900001, go_contact_item
assert go_contact_item["employeeId"] == 1 and go_contact_item["contactId"] == 900001, go_contact_item
assert go_contact_item["remark"] == "Go迁移备注", go_contact_item
assert go_contact_item["addWay"] == 1 and go_contact_item["addWayText"] == "扫描二维码", go_contact_item
assert go_contact_item["gender"] == 1 and go_contact_item["genderText"] == "男", go_contact_item
assert go_contact_item["businessNo"] == "GO-900001" and go_contact_item["name"] == "Go迁移客户", go_contact_item
assert go_contact_item["avatar"] == "http://127.0.0.1:9501/static/avatar/contact.png", go_contact_item
assert go_contact_item["roomName"] == ["Go迁移客户群"], go_contact_item
assert go_contact_item["employeeName"] == "Go迁移员工", go_contact_item
assert go_contact_item["tag"] == ["Go迁移标签"], go_contact_item
assert go_contact_item["isContact"] == 1, go_contact_item
assert go_work_contact_loss["code"] == 200, go_work_contact_loss
assert php_work_contact_loss_code == "200", php_work_contact_loss_code
assert php_work_contact_loss["code"] == 200, php_work_contact_loss
assert go_work_contact_loss["data"]["page"] == php_work_contact_loss["data"]["page"], (go_work_contact_loss, php_work_contact_loss)
assert go_work_contact_loss["data"]["page"] == {"perPage": 20, "total": 1, "totalPage": 1}, go_work_contact_loss
go_loss_item = go_work_contact_loss["data"]["list"][0]
php_loss_item = php_work_contact_loss["data"]["list"][0]
loss_contact_keys = [
    "id", "employeeId", "contactId", "deletedAt", "avatar", "name", "tag",
    "employeeName", "remark",
]
assert {key: go_loss_item[key] for key in loss_contact_keys} == {key: php_loss_item[key] for key in loss_contact_keys}, (go_loss_item, php_loss_item)
assert go_loss_item["id"] == 900002, go_loss_item
assert go_loss_item["employeeId"] == 1 and go_loss_item["contactId"] == 900002, go_loss_item
assert go_loss_item["deletedAt"] == "2026-07-03 10:00:00", go_loss_item
assert go_loss_item["avatar"] == "", go_loss_item
assert go_loss_item["name"] == "Go迁移客户二", go_loss_item
assert go_loss_item["tag"] == ["Go迁移标签"], go_loss_item
assert go_loss_item["employeeName"] == "Go迁移更新企业 Go迁移员工", go_loss_item
assert go_loss_item["remark"] == "Go迁移员工", go_loss_item
expected_sources = [
    {"addWay": 0, "addWayText": "其他渠道"},
    {"addWay": 1, "addWayText": "扫描二维码"},
    {"addWay": 1001, "addWayText": "渠道活码"},
    {"addWay": 1002, "addWayText": "自动拉群"},
    {"addWay": 1003, "addWayText": "裂变引流"},
    {"addWay": 2, "addWayText": "搜索手机号"},
    {"addWay": 3, "addWayText": "名片分享"},
    {"addWay": 4, "addWayText": "群聊"},
    {"addWay": 5, "addWayText": "手机通讯录"},
    {"addWay": 6, "addWayText": "微信联系人"},
    {"addWay": 7, "addWayText": "来自微信的添加好友申请"},
    {"addWay": 8, "addWayText": "安装第三方应用时自动添加的客服人员"},
    {"addWay": 9, "addWayText": "搜索邮箱"},
    {"addWay": 201, "addWayText": "内部成员共享"},
    {"addWay": 202, "addWayText": "管理员/负责人分配"},
]
assert go_work_contact_source["code"] == 200, go_work_contact_source
assert php_work_contact_source_code == "200", php_work_contact_source_code
assert php_work_contact_source["code"] == 200, php_work_contact_source
assert go_work_contact_source["data"] == php_work_contact_source["data"], (go_work_contact_source, php_work_contact_source)
assert go_work_contact_source["data"] == expected_sources, go_work_contact_source
expected_contact_show = {
    "name": "Go迁移客户",
    "avatar": "http://127.0.0.1:9501/static/avatar/contact.png",
    "gender": 1,
    "genderText": "男",
    "businessNo": "GO-900001",
    "remark": "Go迁移备注",
    "description": "Go迁移描述",
    "tag": [{"tagId": 900001, "tagName": "Go迁移标签"}],
    "roomName": ["Go迁移客户群"],
    "employeeName": ["Go迁移更新企业 Go迁移员工"],
}
assert go_work_contact_show["code"] == 200, go_work_contact_show
assert php_work_contact_show_code == "200", php_work_contact_show_code
assert php_work_contact_show["code"] == 200, php_work_contact_show
assert go_work_contact_show["data"] == php_work_contact_show["data"], (go_work_contact_show, php_work_contact_show)
assert go_work_contact_show["data"] == expected_contact_show, go_work_contact_show
expected_contact_tracks = [
    {"id": 900001, "content": "Go迁移互动轨迹", "createdAt": "2026-07-03 09:30:00"},
    {"id": 900002, "content": "Go迁移互动轨迹二", "createdAt": "2026-07-03 09:20:00"},
]
assert go_work_contact_track["code"] == 200, go_work_contact_track
assert php_work_contact_track_code == "200", php_work_contact_track_code
assert php_work_contact_track["code"] == 200, php_work_contact_track
assert go_work_contact_track["data"] == php_work_contact_track["data"], (go_work_contact_track, php_work_contact_track)
assert go_work_contact_track["data"] == expected_contact_tracks, go_work_contact_track
assert go_work_contact_update["code"] == 200 and go_work_contact_update["data"] == [], go_work_contact_update
contact_employee_after_update = (work / "work_contact_employee_after_update.tsv").read_text(encoding="utf-8").strip().split("\t")
assert contact_employee_after_update == ["Go迁移备注更新", "Go迁移描述更新"], contact_employee_after_update
assert (work / "work_contact_after_update.txt").read_text(encoding="utf-8").strip() == "GO-UPDATED"
assert (work / "work_contact_update_tag_pivot_count.txt").read_text(encoding="utf-8").strip() == "1"
track_rows = {
    tuple(line.split("\t"))
    for line in (work / "work_contact_update_tracks.tsv").read_text(encoding="utf-8").strip().splitlines()
    if line.strip()
}
expected_update_tracks = {
    ("2", "1", "1", "系统对该客户打标签【Go迁移更新标签】"),
    ("3", "1", "1", "修改用户资料：备注"),
    ("3", "1", "1", "修改用户资料：描述"),
    ("3", "1", "1", "修改用户资料：客户编号"),
}
assert expected_update_tracks <= track_rows, track_rows
assert go_work_contact_batch_labeling["code"] == 200 and go_work_contact_batch_labeling["data"] == [], go_work_contact_batch_labeling
assert (work / "work_contact_batch_labeling_pivot_count.txt").read_text(encoding="utf-8").strip() == "1"
assert go_sidebar_work_contact_update["code"] == 200 and go_sidebar_work_contact_update["data"] == [], go_sidebar_work_contact_update
sidebar_contact_employee_after_update = (work / "work_contact_employee_after_sidebar_update.tsv").read_text(encoding="utf-8").strip().split("\t")
assert sidebar_contact_employee_after_update == ["Go迁移侧边栏备注更新", "Go迁移侧边栏描述更新"], sidebar_contact_employee_after_update
assert (work / "work_contact_after_sidebar_update.txt").read_text(encoding="utf-8").strip() == "GO-SIDEBAR"
assert (work / "work_contact_sidebar_update_tag_pivot_count.txt").read_text(encoding="utf-8").strip() == "1"
assert go_work_room_batch_update["code"] == 200 and go_work_room_batch_update["data"] == [], go_work_room_batch_update
assert (work / "work_room_group_after_batch_update.txt").read_text(encoding="utf-8").strip() == "900001"
updated_work_room_item = go_work_room_index_after_batch_update["data"]["list"][0]
assert updated_work_room_item["workRoomId"] == 900001, updated_work_room_item
assert updated_work_room_item["roomGroup"] == "Go迁移客户群分组", updated_work_room_item
expected_contact_room_index = {
    "memberNum": 1,
    "outRoomNum": 0,
    "page": {"perPage": "10", "total": 1, "totalPage": 1},
    "list": [{
        "workContactRoomId": 900001,
        "name": "Go迁移客户",
        "avatar": "http://127.0.0.1:9501/static/avatar/contact.png",
        "isOwner": 1,
        "joinTime": f"{today} 08:00:00",
        "outRoomTime": "",
        "otherRooms": [],
        "joinScene": 3,
        "joinSceneText": "通过扫描群二维码入群",
        "type": 2,
        "contactId": 900001,
        "employeeId": 0,
        "contactEmployeeId": 1,
    }],
}
assert go_work_contact_room_index["code"] == 200, go_work_contact_room_index
assert php_work_contact_room_index_code == "200", php_work_contact_room_index_code
assert php_work_contact_room_index["code"] == 200, php_work_contact_room_index
assert go_work_contact_room_index["data"] == php_work_contact_room_index["data"], (go_work_contact_room_index, php_work_contact_room_index)
assert go_work_contact_room_index["data"] == expected_contact_room_index, go_work_contact_room_index
assert go_work_contact_room_index_status0["code"] == 200, go_work_contact_room_index_status0
assert php_work_contact_room_index_status0_code == "200", php_work_contact_room_index_status0_code
assert php_work_contact_room_index_status0["code"] == 200, php_work_contact_room_index_status0
assert go_work_contact_room_index_status0["data"] == php_work_contact_room_index_status0["data"], (go_work_contact_room_index_status0, php_work_contact_room_index_status0)
assert go_work_contact_room_index_status0["data"] == expected_contact_room_index, go_work_contact_room_index_status0
assert go_work_room_index["code"] == 200, go_work_room_index
assert php_work_room_index_code == "200", php_work_room_index_code
assert php_work_room_index["code"] == 200, php_work_room_index
assert go_work_room_index["data"] == php_work_room_index["data"], (go_work_room_index, php_work_room_index)
assert go_work_room_index["data"]["page"] == {"perPage": "10", "total": 1, "totalPage": 1}, go_work_room_index
work_room_item = go_work_room_index["data"]["list"][0]
assert work_room_item["workRoomId"] == 900001, work_room_item
assert work_room_item["memberNum"] == 1, work_room_item
assert work_room_item["roomName"] == "Go迁移客户群", work_room_item
assert work_room_item["ownerName"] == "Go迁移更新企业-Go迁移员工", work_room_item
assert work_room_item["roomGroup"] == "", work_room_item
assert work_room_item["status"] == 0 and work_room_item["statusText"] == "正常", work_room_item
assert work_room_item["inRoomNum"] == 1 and work_room_item["outRoomNum"] == 0, work_room_item
assert work_room_item["notice"] == "", work_room_item
assert work_room_item["createTime"], work_room_item
expected_work_room_room_index = {
    "total": 1,
    "list": [{
        "roomMax": 500,
        "roomId": 900001,
        "roomName": "Go迁移客户群",
        "currentNum": 1,
    }],
}
assert go_work_room_room_index["code"] == 200, go_work_room_room_index
assert php_work_room_room_index_code == "200", php_work_room_room_index_code
assert php_work_room_room_index["code"] == 200, php_work_room_room_index
assert go_work_room_room_index["data"] == php_work_room_room_index["data"], (go_work_room_room_index, php_work_room_room_index)
assert go_work_room_room_index["data"] == expected_work_room_room_index, go_work_room_room_index
expected_work_room_statistics = {
    "addNum": 1,
    "outNum": 0,
    "total": 1,
    "outTotal": 0,
    "addNumRange": 1,
    "outNumRange": 0,
    "list": [{"time": today, "addNum": 1, "outNum": 0}],
}
assert go_work_room_statistics["code"] == 200, go_work_room_statistics
assert php_work_room_statistics_code == "200", php_work_room_statistics_code
assert php_work_room_statistics["code"] == 200, php_work_room_statistics
assert go_work_room_statistics["data"] == php_work_room_statistics["data"], (go_work_room_statistics, php_work_room_statistics)
assert go_work_room_statistics["data"] == expected_work_room_statistics, go_work_room_statistics
expected_work_room_statistics_index = {
    "page": {"perPage": "10", "total": 1, "totalPage": 1},
    "list": [{"time": today, "addNum": 1, "outNum": 0, "total": 1, "outTotal": 0}],
}
assert go_work_room_statistics_index["code"] == 200, go_work_room_statistics_index
assert php_work_room_statistics_index_code == "200", php_work_room_statistics_index_code
assert php_work_room_statistics_index["code"] == 200, php_work_room_statistics_index
assert go_work_room_statistics_index["data"] == php_work_room_statistics_index["data"], (go_work_room_statistics_index, php_work_room_statistics_index)
assert go_work_room_statistics_index["data"] == expected_work_room_statistics_index, go_work_room_statistics_index
assert go_work_room_auto_pull_store["code"] == 200, go_work_room_auto_pull_store
created_qrcode = (work / "work_room_auto_pull_created_qrcode.tsv").read_text(encoding="utf-8").strip().split("\t")
assert created_qrcode == ["https://wecom.example/auto-pull-created.png", "go-auto-pull-created"], created_qrcode
assert go_work_room_auto_pull_update["code"] == 200, go_work_room_auto_pull_update
auto_pull_after_update = (work / "work_room_auto_pull_after_update.tsv").read_text(encoding="utf-8").strip().split("\t")
assert auto_pull_after_update[0] == "1", auto_pull_after_update
assert json.loads(auto_pull_after_update[1]) == ["1"], auto_pull_after_update
assert json.loads(auto_pull_after_update[2]) == ["900001"], auto_pull_after_update
assert json.loads(auto_pull_after_update[3])[0]["maxNum"] == 40, auto_pull_after_update
auto_pull_logs = [line.split("\t") for line in (work / "work_room_auto_pull_business_logs.tsv").read_text(encoding="utf-8").strip().splitlines()]
assert any(row[0] == "200" and row[1] == "1" for row in auto_pull_logs), auto_pull_logs
assert any(row[0] == "201" and row[1] == "1" and row[2] == "900001" for row in auto_pull_logs), auto_pull_logs
wecom_requests = [json.loads(line) for line in (work / "wecom_requests.ndjson").read_text(encoding="utf-8").splitlines() if line.strip()]
assert any("/cgi-bin/agent/get" in item["path"] and "agentid=1000003" in item["path"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/auth/getuserinfo" in item["path"] and "code=go-oauth-code" in item["path"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/auth/getuserinfo" in item["path"] and "code=go-sidebar-code" in item["path"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/get_jsapi_ticket" in item["path"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/ticket/get" in item["path"] and "type=agent_config" in item["path"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/externalcontact/add_contact_way" in item["path"] and "channelCode-" in item["body"] and "go-migrate-user" in item["body"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/externalcontact/update_contact_way" in item["path"] and "go-channel-code-config-created" in item["body"] and "channelCode-" in item["body"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/externalcontact/contact_way/create" in item["path"] and "workRoomAutoPullId-" in item["body"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/externalcontact/contact_way/update" in item["path"] and "config-old" in item["body"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/media/uploadimg" in item["path"] for item in wecom_requests), wecom_requests
assert any(item["path"].startswith("/cgi-bin/media/upload?") and "type=image" in item["path"] and "go-media.png" in item["body"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/externalcontact/add_msg_template" in item["path"] and "external-user-900001" in item["body"] and "go-migrate-user" in item["body"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/externalcontact/remark" in item["path"] and "go-migrate-user" in item["body"] and "external-user-900001" in item["body"] and "Go迁移备注更新" in item["body"] and "Go迁移描述更新" in item["body"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/externalcontact/mark_tag" in item["path"] and "go-migrate-user" in item["body"] and "external-user-900001" in item["body"] and "go-contact-tag-update" in item["body"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/externalcontact/remark" in item["path"] and "go-migrate-user" in item["body"] and "external-user-900001" in item["body"] and "Go迁移侧边栏备注更新" in item["body"] and "Go迁移侧边栏描述更新" in item["body"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/externalcontact/mark_tag" in item["path"] and "go-migrate-user" in item["body"] and "external-user-900001" in item["body"] and "go-contact-tag-sidebar" in item["body"] for item in wecom_requests), wecom_requests
assert any("/cgi-bin/message/send" in item["path"] and "go-migrate-user" in item["body"] and "管理员提醒你发送群发任务" in item["body"] for item in wecom_requests), wecom_requests
assert go_work_room_auto_pull_index["code"] == 200, go_work_room_auto_pull_index
assert php_work_room_auto_pull_index_code == "200", php_work_room_auto_pull_index_code
assert php_work_room_auto_pull_index["code"] == 200, php_work_room_auto_pull_index
assert go_work_room_auto_pull_index["data"]["page"] == php_work_room_auto_pull_index["data"]["page"], (go_work_room_auto_pull_index, php_work_room_auto_pull_index)
go_auto_pull_item = next(item for item in go_work_room_auto_pull_index["data"]["list"] if item["workRoomAutoPullId"] == 900001)
php_auto_pull_item = next(item for item in php_work_room_auto_pull_index["data"]["list"] if item["workRoomAutoPullId"] == 900001)
for key in ("workRoomAutoPullId", "qrcodeName", "leadingWords", "tags", "employees", "rooms", "contactNum", "createdAt"):
    assert go_auto_pull_item[key] == php_auto_pull_item[key], (key, go_auto_pull_item, php_auto_pull_item)
assert go_auto_pull_item["qrcodeName"] == "Go迁移自动拉群", go_auto_pull_item
assert go_auto_pull_item["tags"] == ["Go迁移标签"], go_auto_pull_item
assert go_auto_pull_item["employees"] == ["Go迁移员工"], go_auto_pull_item
assert go_auto_pull_item["rooms"] == [{"roomName": "Go迁移客户群", "stateText": "拉人中"}], go_auto_pull_item
assert go_work_room_auto_pull_show["code"] == 200, go_work_room_auto_pull_show
assert php_work_room_auto_pull_show_code == "200", php_work_room_auto_pull_show_code
assert php_work_room_auto_pull_show["code"] == 200, php_work_room_auto_pull_show
go_auto_pull_show = go_work_room_auto_pull_show["data"]
php_auto_pull_show = php_work_room_auto_pull_show["data"]
for key in ("workRoomAutoPullId", "qrcodeName", "isVerified", "roomNum", "leadingWords", "createdAt", "employees", "tags", "selectedTags"):
    assert go_auto_pull_show[key] == php_auto_pull_show[key], (key, go_auto_pull_show, php_auto_pull_show)
go_auto_pull_room = go_auto_pull_show["rooms"][0]
php_auto_pull_room = php_auto_pull_show["rooms"][0]
for key in ("roomId", "roomName", "roomMax", "num", "maxNum", "roomQrcodeUrl", "state"):
    assert go_auto_pull_room[key] == php_auto_pull_room[key], (key, go_auto_pull_room, php_auto_pull_room)
assert go_auto_pull_show["workRoomAutoPullId"] == 900001 and go_auto_pull_show["roomNum"] == 1, go_auto_pull_show
assert go_auto_pull_room["state"] == 2 and go_auto_pull_room["num"] == 1, go_auto_pull_room
assert go_sidebar_work_contact_tag_all["code"] == 200, go_sidebar_work_contact_tag_all
assert php_sidebar_work_contact_tag_all_code == "200", php_sidebar_work_contact_tag_all_code
assert php_sidebar_work_contact_tag_all["code"] == 200, php_sidebar_work_contact_tag_all
assert go_sidebar_work_contact_tag_all["data"] == php_sidebar_work_contact_tag_all["data"], (go_sidebar_work_contact_tag_all, php_sidebar_work_contact_tag_all)
assert {"id": 900001, "name": "Go迁移标签"} in go_sidebar_work_contact_tag_all["data"], go_sidebar_work_contact_tag_all
assert go_sidebar_work_contact_detail["code"] == 200, go_sidebar_work_contact_detail
assert php_sidebar_work_contact_detail_code == "200", php_sidebar_work_contact_detail_code
assert php_sidebar_work_contact_detail["code"] == 200, php_sidebar_work_contact_detail
assert go_sidebar_work_contact_detail["data"] == php_sidebar_work_contact_detail["data"], (go_sidebar_work_contact_detail, php_sidebar_work_contact_detail)
assert go_sidebar_work_contact_detail["data"] == {"id": 900001, "name": "Go迁移客户", "avatar": "http://127.0.0.1:9501/static/avatar/contact.png", "corpId": 1}, go_sidebar_work_contact_detail
assert go_sidebar_work_contact_show["code"] == 200, go_sidebar_work_contact_show
assert php_sidebar_work_contact_show_code == "200", php_sidebar_work_contact_show_code
assert php_sidebar_work_contact_show["code"] == 200, php_sidebar_work_contact_show
assert go_sidebar_work_contact_show["data"] == php_sidebar_work_contact_show["data"], (go_sidebar_work_contact_show, php_sidebar_work_contact_show)
assert go_sidebar_work_contact_show["data"] == expected_contact_show, go_sidebar_work_contact_show
assert go_sidebar_work_contact_track["code"] == 200, go_sidebar_work_contact_track
assert php_sidebar_work_contact_track_code == "200", php_sidebar_work_contact_track_code
assert php_sidebar_work_contact_track["code"] == 200, php_sidebar_work_contact_track
assert go_sidebar_work_contact_track["data"] == php_sidebar_work_contact_track["data"], (go_sidebar_work_contact_track, php_sidebar_work_contact_track)
assert go_sidebar_work_contact_track["data"] == expected_contact_tracks, go_sidebar_work_contact_track
expected_process_names = ["新客户", "初步沟通", "意向客户", "付款客户", "无意向客户"]
assert go_sidebar_contact_process_status_index["code"] == 200, go_sidebar_contact_process_status_index
assert php_sidebar_contact_process_status_index_code == "200", php_sidebar_contact_process_status_index_code
assert php_sidebar_contact_process_status_index["code"] == 200, php_sidebar_contact_process_status_index
assert go_sidebar_contact_process_status_index["data"] == php_sidebar_contact_process_status_index["data"], (go_sidebar_contact_process_status_index, php_sidebar_contact_process_status_index)
assert [item["name"] for item in go_sidebar_contact_process_status_index["data"]] == expected_process_names, go_sidebar_contact_process_status_index
target_process = next(item for item in go_sidebar_contact_process_status_index["data"] if item["name"] == "意向客户")
assert go_sidebar_contact_process_status_update["code"] == 200, go_sidebar_contact_process_status_update
status_after_update = (work / "contact_process_status_after_update.txt").read_text(encoding="utf-8").strip()
assert status_after_update == str(target_process["id"]), (status_after_update, target_process)
track_after_update = (work / "contact_process_track_after_update.tsv").read_text(encoding="utf-8").strip().split("\t")
assert track_after_update == ["5", "1", "1", "编辑用户跟进状态：意向客户"], track_after_update
assert go_contact_field_index["code"] == 200, go_contact_field_index
assert php_contact_field_index_code == "200", php_contact_field_index_code
assert php_contact_field_index["code"] == 200, php_contact_field_index
go_field = next(item for item in go_contact_field_index["data"]["list"] if item["id"] == 900001)
php_field = next(item for item in php_contact_field_index["data"]["list"] if item["id"] == 900001)
for key in ("id", "name", "label", "type", "options", "status", "order", "isSys", "typeText"):
    assert go_field[key] == php_field[key], (key, go_field, php_field)
assert go_contact_field_show["code"] == 200, go_contact_field_show
assert php_contact_field_show_code == "200", php_contact_field_show_code
assert php_contact_field_show["code"] == 200, php_contact_field_show
for key in ("id", "name", "label", "type", "options", "status", "order", "isSys", "typeText"):
    assert go_contact_field_show["data"][key] == php_contact_field_show["data"][key], (key, go_contact_field_show, php_contact_field_show)
assert go_contact_field_portrait["code"] == 200, go_contact_field_portrait
assert php_contact_field_portrait_code == "200", php_contact_field_portrait_code
assert php_contact_field_portrait["code"] == 200, php_contact_field_portrait
assert go_contact_field_portrait["data"] == php_contact_field_portrait["data"], (go_contact_field_portrait, php_contact_field_portrait)
assert any(item.get("fieldId") == 900001 and item.get("name") == "Go迁移字段" and item.get("typeText") == "下拉" for item in go_contact_field_portrait["data"] if isinstance(item, dict)), go_contact_field_portrait
assert not any(item.get("fieldId") == 900002 for item in go_contact_field_portrait["data"] if isinstance(item, dict)), go_contact_field_portrait
assert go_contact_field_pivot_index["code"] == 200, go_contact_field_pivot_index
assert php_contact_field_pivot_index_code == "200", php_contact_field_pivot_index_code
assert php_contact_field_pivot_index["code"] == 200, php_contact_field_pivot_index
assert go_contact_field_pivot_index["data"] == php_contact_field_pivot_index["data"], (go_contact_field_pivot_index, php_contact_field_pivot_index)
pivot_select = next(item for item in go_contact_field_pivot_index["data"] if item["contactFieldId"] == 900001)
assert pivot_select["name"] == "Go迁移字段" and pivot_select["value"] == "选项B" and pivot_select["contactFieldPivotId"] == 900001, pivot_select
pivot_picture = next(item for item in go_contact_field_pivot_index["data"] if item["contactFieldId"] == 900002)
assert pivot_picture["pictureFlag"] == "http://127.0.0.1:9501/static/portrait/a.png", pivot_picture
pivot_checkbox = next(item for item in go_contact_field_pivot_index["data"] if item["contactFieldId"] == 900003)
assert pivot_checkbox["value"] == ["多选B"], pivot_checkbox
assert go_sidebar_contact_field_pivot_index["code"] == 200, go_sidebar_contact_field_pivot_index
assert php_sidebar_contact_field_pivot_index_code == "200", php_sidebar_contact_field_pivot_index_code
assert php_sidebar_contact_field_pivot_index["code"] == 200, php_sidebar_contact_field_pivot_index
assert go_sidebar_contact_field_pivot_index["data"] == php_sidebar_contact_field_pivot_index["data"], (go_sidebar_contact_field_pivot_index, php_sidebar_contact_field_pivot_index)
sidebar_picture = next(item for item in go_sidebar_contact_field_pivot_index["data"] if item["contactFieldId"] == 900002)
assert sidebar_picture["pictureFlag"] == "http://127.0.0.1:9501/static/portrait/a.png", sidebar_picture
assert go_contact_field_pivot_update["code"] == 200, go_contact_field_pivot_update
assert go_sidebar_contact_field_pivot_update["code"] == 200, go_sidebar_contact_field_pivot_update
assert (work / "contact_field_pivot_value_after_dashboard_update.txt").read_text(encoding="utf-8").strip() == "选项B"
assert (work / "contact_field_pivot_value_after_sidebar_update.txt").read_text(encoding="utf-8").strip() == "多选B"
dashboard_pivot_track = (work / "contact_field_pivot_track_after_dashboard_update.tsv").read_text(encoding="utf-8").rstrip("\n").split("\t")
assert dashboard_pivot_track == ["4", "1", "1", "编辑用户画像：Go迁移字段 "], dashboard_pivot_track
sidebar_pivot_track = (work / "contact_field_pivot_track_after_sidebar_update.tsv").read_text(encoding="utf-8").rstrip("\n").split("\t")
assert sidebar_pivot_track == ["4", "1", "1", "编辑用户画像：Go迁移多选字段 "], sidebar_pivot_track
assert go_permission["code"] == 200 and isinstance(go_permission["data"], list) and go_permission["data"], go_permission
assert go_role_select["code"] == 200, go_role_select
assert {"roleId": 1, "name": "Go迁移管理员"} in go_role_select["data"], go_role_select
assert go_role_index["code"] == 200 and go_role_index["data"]["page"]["total"] >= 1, go_role_index
assert any(role["roleId"] == 1 and role["employeeNum"] >= 1 for role in go_role_index["data"]["list"]), go_role_index
assert go_role_show["code"] == 200 and go_role_show["data"]["roleId"] == 1 and go_role_show["data"]["name"] == "Go迁移管理员", go_role_show
assert go_role_permission_show["code"] == 200 and isinstance(go_role_permission_show["data"], list) and go_role_permission_show["data"], go_role_permission_show
assert go_role_show_employee["code"] == 200 and go_role_show_employee["data"]["page"]["total"] >= 1, go_role_show_employee
assert any(employee["employeeId"] == 1 for employee in go_role_show_employee["data"]["list"]), go_role_show_employee
assert go_menu_icon_index["code"] == 200 and "line-chart" in go_menu_icon_index["data"], go_menu_icon_index
assert go_menu_select["code"] == 200 and isinstance(go_menu_select["data"], list) and go_menu_select["data"], go_menu_select
assert any(item["menuId"] == 1 and item["children"] for item in go_menu_select["data"]), go_menu_select
assert go_menu_index["code"] == 200 and go_menu_index["data"]["page"]["total"] >= 1, go_menu_index
assert any(item["menuId"] == 1 and item["menuPath"] == "1" for item in go_menu_index["data"]["list"]), go_menu_index
assert go_menu_show["code"] == 200 and go_menu_show["data"]["menuId"] == 1, go_menu_show
assert go_menu_show["data"]["firstMenuId"] in ("1", 1), go_menu_show
assert go_chat_tool_config["code"] == 200, go_chat_tool_config
assert go_chat_tool_config["data"]["whiteDomains"] == ["http://sidebar.local", "http://127.0.0.1:9501"], go_chat_tool_config
agent = go_chat_tool_config["data"]["agents"][0]
assert agent["id"] == 1 and agent["name"] == "Go迁移侧边栏", go_chat_tool_config
assert agent["chatTools"][0]["pageUrl"] == "http://sidebar.local/contact?agentId=1", go_chat_tool_config
assert agent["chatTools"][1]["pageUrl"] == "http://sidebar.local/medium?agentId=1", go_chat_tool_config
assert go_agent_store["code"] == 200, go_agent_store
agent_created = (work / "agent_store_created.tsv").read_text(encoding="utf-8").strip().split("\t")
assert agent_created == [
    "1",
    "1000003",
    "agent-secret-created",
    "Go迁移企业应用新增",
    "https://wecom.example/agent-created-logo.png",
    "Go迁移企业应用描述",
    "0",
    "go-agent.example.com",
    "1",
    "1",
    "https://go-agent.example.com/home",
], agent_created
assert go_sidebar_agent_oauth_url["code"] == 200, go_sidebar_agent_oauth_url
oauth_url = go_sidebar_agent_oauth_url["data"]["url"]
assert oauth_url.startswith("https://open.weixin.qq.com/connect/oauth2/authorize?"), oauth_url
assert "appid=wx-test-corp" in oauth_url and "redirect_uri=" in oauth_url, oauth_url
assert go_sidebar_agent_oauth_token["code"] == 200 and go_sidebar_agent_oauth_token["data"]["token"], go_sidebar_agent_oauth_token
assert int(go_sidebar_agent_oauth_token["data"]["expire"]) == 604800, go_sidebar_agent_oauth_token
auth_redirect_status, auth_redirect_url = (work / "go_sidebar_agent_auth_redirect.txt").read_text(encoding="utf-8").strip().split(" ", 1)
assert auth_redirect_status == "302", auth_redirect_status
assert auth_redirect_url.startswith("https://open.weixin.qq.com/connect/oauth2/authorize?"), auth_redirect_url
auth_callback_status, auth_callback_url = (work / "go_sidebar_agent_auth_callback.txt").read_text(encoding="utf-8").strip().split(" ", 1)
assert auth_callback_status == "302", auth_callback_status
parsed_callback = urllib.parse.urlparse(auth_callback_url)
assert parsed_callback.scheme == "http" and parsed_callback.netloc == "sidebar.local" and parsed_callback.path == "/auth", auth_callback_url
callback_query = urllib.parse.parse_qs(parsed_callback.query)
callback_state = json.loads(base64.b64decode(callback_query["state"][0]).decode("utf-8"))
assert callback_state["code"] == 200 and callback_state["data"]["token"], callback_state
assert callback_query["target"][0] == "http://sidebar.local/contact", callback_query
assert go_sidebar_agent_jssdk_config["code"] == 200, go_sidebar_agent_jssdk_config
agent_jssdk = go_sidebar_agent_jssdk_config["data"]
assert agent_jssdk["agentid"] == "1000002" and agent_jssdk["corpid"] == "wx-test-corp", agent_jssdk
assert agent_jssdk["signature"] and "navigateToAddCustomer" in agent_jssdk["jsApiList"], agent_jssdk
assert go_sidebar_wx_jssdk_config["code"] == 200, go_sidebar_wx_jssdk_config
wx_jssdk = go_sidebar_wx_jssdk_config["data"]
assert wx_jssdk["agentid"] == "1000002" and wx_jssdk["corpid"] == "wx-test-corp", wx_jssdk
assert wx_jssdk["signature"] and "sendChatMessage" in wx_jssdk["jsApiList"], wx_jssdk
assert go_sidebar_wx_jssdk_corp_config["code"] == 200, go_sidebar_wx_jssdk_corp_config
wx_corp_jssdk = go_sidebar_wx_jssdk_corp_config["data"]
assert wx_corp_jssdk["appId"] == "wx-test-corp" and wx_corp_jssdk["corpId"] == "wx-test-corp", wx_corp_jssdk
assert wx_corp_jssdk["signature"] and "getCurExternalContact" in wx_corp_jssdk["jsApiList"], wx_corp_jssdk
assert (work / "go_txt_verify.txt").read_text(encoding="utf-8") == "ABCDEF1234567890"
assert go_txt_verify_upload["code"] == 200, go_txt_verify_upload
assert (work / "upload/wx_txt_verify/WW_verify_ABCDEF1234567890.txt").read_text(encoding="utf-8") == "ABCDEF1234567890"
assert go_normal_auth["code"] == 200 and go_normal_auth["data"]["token"], go_normal_auth
assert go_normal_corp_select["code"] == 200, go_normal_corp_select
assert [corp["corpId"] for corp in go_normal_corp_select["data"]] == [1], go_normal_corp_select
assert normal_forbidden_code == "400", normal_forbidden_code
assert go_normal_forbidden_bind["code"] == 400, go_normal_forbidden_bind
assert go_normal_corp_bind["code"] == 200, go_normal_corp_bind
assert go_normal_login_after_bind["code"] == 200 and go_normal_login_after_bind["data"]["corpId"] == 1, go_normal_login_after_bind
assert go_normal_login_after_bind["data"]["employeeId"] == 2, go_normal_login_after_bind
assert normal_user_cache.strip() == "1-2", normal_user_cache
assert go_corp_bind["code"] == 200, go_corp_bind
assert go_login_after_bind["code"] == 200 and go_login_after_bind["data"]["corpId"] == 1, go_login_after_bind
assert go_login_after_bind["data"]["employeeId"] == 0, go_login_after_bind
assert user_cache.strip() == "1-0", user_cache
assert go_logout["code"] == 200, go_logout
assert php_login_code == "200", php_login_code
assert php_permission_code == "200", php_permission_code
assert after_go_code == "401", after_go_code
assert after_php_code == "401", after_php_code
assert "[jwt:blacklist:mc_jwt_" in redis_keys, redis_keys
print("real php auth chain passed")
PY

curl -sS -f -X PUT -H "Authorization: Bearer $normal_token" "http://$GO_ADDR/dashboard/workEmployee/synEmployee" >"$WORK_DIR/go_work_employee_sync.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_department WHERE corp_id = 1 AND wx_department_id = 3 AND name = 'Go迁移同步新部门' AND deleted_at IS NULL;" >"$WORK_DIR/work_employee_sync_department_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_employee we JOIN mc_work_department wd ON wd.id = we.main_department_id WHERE we.corp_id = 1 AND we.wx_user_id = 'go-sync-user' AND we.name = 'Go迁移同步新员工' AND we.mobile = '13700000000' AND we.contact_auth = 1 AND wd.wx_department_id = 3 AND we.deleted_at IS NULL AND wd.deleted_at IS NULL;" >"$WORK_DIR/work_employee_sync_employee_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_user WHERE phone = '13700000000' AND tenant_id = 1 AND password <> '' AND deleted_at IS NULL;" >"$WORK_DIR/work_employee_sync_user_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_employee_department wed JOIN mc_work_employee we ON we.id = wed.employee_id JOIN mc_work_department wd ON wd.id = wed.department_id WHERE we.wx_user_id = 'go-sync-user' AND wd.wx_department_id = 3 AND wed.deleted_at IS NULL;" >"$WORK_DIR/work_employee_sync_relation_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT deleted_at IS NOT NULL FROM mc_work_employee_department WHERE id = 900001;" >"$WORK_DIR/work_employee_sync_old_relation_deleted.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_employee WHERE id = 1 AND name = 'Go迁移员工同步更新' AND avatar = 'https://wecom.example/avatar-updated.png' AND status = 4 AND contact_auth = 1 AND deleted_at IS NULL;" >"$WORK_DIR/work_employee_sync_existing_updated_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_update_time WHERE corp_id = 1 AND type = 1 AND last_update_time IS NOT NULL;" >"$WORK_DIR/work_employee_sync_time_count.txt"
python3 - "$WORK_DIR" "$WORK_DIR/wecom_requests.ndjson" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
request_log = pathlib.Path(sys.argv[2])

def text(name):
    return (work / name).read_text(encoding="utf-8").strip()

sync = json.loads(text("go_work_employee_sync.json"))
assert sync["code"] == 200 and sync["data"] == [], sync
assert text("work_employee_sync_department_count.txt") == "1"
assert text("work_employee_sync_employee_count.txt") == "1"
assert text("work_employee_sync_user_count.txt") == "1"
assert text("work_employee_sync_relation_count.txt") == "1"
assert text("work_employee_sync_old_relation_deleted.txt") == "1"
assert text("work_employee_sync_existing_updated_count.txt") == "1"
assert text("work_employee_sync_time_count.txt") != "0"
requests = [json.loads(line) for line in request_log.read_text(encoding="utf-8").splitlines() if line.strip()]
assert any("/cgi-bin/department/list" in item["path"] for item in requests), requests
assert any("/cgi-bin/user/list" in item["path"] and "department_id=3" in item["path"] for item in requests), requests
assert any("/cgi-bin/externalcontact/get_follow_user_list" in item["path"] for item in requests), requests
print("work employee sync smoke passed")
PY

curl -sS -f -X POST -H 'Content-Type: application/json' -d '{"phone":"13800000000","password":"secret123"}' "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/go_auth_after_employee_sync.json"
admin_sync_token="$(python3 - "$WORK_DIR/go_auth_after_employee_sync.json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1], encoding="utf-8"))["data"]["token"])
PY
)"
curl -sS -f -X PUT -H "Authorization: Bearer $admin_sync_token" "http://$GO_ADDR/dashboard/workContact/synContact" >"$WORK_DIR/go_work_contact_sync.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact WHERE corp_id = 1 AND wx_external_userid = 'go-sync-contact' AND name = 'Go迁移同步新客户' AND avatar = 'https://wecom.example/contact-created.png' AND gender = 1 AND deleted_at IS NULL;" >"$WORK_DIR/work_contact_sync_new_contact_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact WHERE id = 900001 AND wx_external_userid = 'external-user-900001' AND name = 'Go迁移客户同步更新' AND avatar = 'https://wecom.example/contact-updated.png' AND gender = 2 AND unionid = 'union-go-900001' AND deleted_at IS NULL;" >"$WORK_DIR/work_contact_sync_existing_contact_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact_employee rel JOIN mc_work_contact c ON c.id = rel.contact_id WHERE c.wx_external_userid = 'go-sync-contact' AND rel.employee_id = 1 AND rel.remark = 'Go同步新客户备注' AND rel.description = 'Go同步新客户描述' AND rel.add_way = 2 AND rel.status = 1 AND rel.deleted_at IS NULL AND c.deleted_at IS NULL;" >"$WORK_DIR/work_contact_sync_new_relation_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact_employee WHERE id = 900001 AND employee_id = 1 AND contact_id = 900001 AND remark = 'Go同步备注' AND description = 'Go同步描述' AND add_way = 3 AND status = 1 AND deleted_at IS NULL;" >"$WORK_DIR/work_contact_sync_existing_relation_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT deleted_at IS NOT NULL FROM mc_work_contact_employee WHERE id = 900003;" >"$WORK_DIR/work_contact_sync_old_relation_deleted.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact_tag_pivot pivot JOIN mc_work_contact c ON c.id = pivot.contact_id JOIN mc_work_contact_tag tag ON tag.id = pivot.contact_tag_id WHERE c.wx_external_userid = 'go-sync-contact' AND pivot.employee_id = 1 AND tag.wx_contact_tag_id = 'go-contact-tag-sync' AND pivot.deleted_at IS NULL AND c.deleted_at IS NULL AND tag.deleted_at IS NULL;" >"$WORK_DIR/work_contact_sync_new_tag_pivot_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_update_time WHERE corp_id = 1 AND type = 2 AND last_update_time IS NOT NULL;" >"$WORK_DIR/work_contact_sync_time_count.txt"
python3 - "$WORK_DIR" "$WORK_DIR/wecom_requests.ndjson" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
request_log = pathlib.Path(sys.argv[2])

def text(name):
    return (work / name).read_text(encoding="utf-8").strip()

sync = json.loads(text("go_work_contact_sync.json"))
assert sync["code"] == 200 and sync["data"] == [], sync
assert text("work_contact_sync_new_contact_count.txt") == "1"
assert text("work_contact_sync_existing_contact_count.txt") == "1"
assert text("work_contact_sync_new_relation_count.txt") == "1"
assert text("work_contact_sync_existing_relation_count.txt") == "1"
assert text("work_contact_sync_old_relation_deleted.txt") == "1"
assert text("work_contact_sync_new_tag_pivot_count.txt") == "1"
assert text("work_contact_sync_time_count.txt") != "0"
requests = [json.loads(line) for line in request_log.read_text(encoding="utf-8").splitlines() if line.strip()]
assert any("/cgi-bin/externalcontact/list" in item["path"] and "userid=go-migrate-user" in item["path"] for item in requests), requests
assert any("/cgi-bin/externalcontact/list" in item["path"] and "userid=go-sync-user" in item["path"] for item in requests), requests
assert any("/cgi-bin/externalcontact/get" in item["path"] and "external_userid=go-sync-contact" in item["path"] for item in requests), requests
print("work contact sync smoke passed")
PY

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
INSERT INTO mc_work_room (id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, room_group_id, created_at, updated_at)
VALUES (900010, 1, 'go-room-sync-old-delete', 'Go迁移同步待删除客户群', 1, '', 0, NOW(), 200, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), wx_chat_id=VALUES(wx_chat_id), name=VALUES(name), owner_id=VALUES(owner_id), notice=VALUES(notice), status=VALUES(status), create_time=VALUES(create_time), room_max=VALUES(room_max), room_group_id=VALUES(room_group_id), updated_at=NOW(), deleted_at=NULL;
INSERT INTO mc_work_contact_room (id, wx_user_id, contact_id, employee_id, unionid, room_id, join_scene, type, status, join_time, out_time, created_at, updated_at)
VALUES (900011, 'external-user-900002', 900002, 0, '', 900001, 3, 2, 1, '$smoke_today 08:10:00', '', NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_user_id=VALUES(wx_user_id), contact_id=VALUES(contact_id), employee_id=VALUES(employee_id), unionid=VALUES(unionid), room_id=VALUES(room_id), join_scene=VALUES(join_scene), type=VALUES(type), status=VALUES(status), join_time=VALUES(join_time), out_time=VALUES(out_time), updated_at=NOW(), deleted_at=NULL;
SQL

curl -sS -f -X PUT -H "Authorization: Bearer $admin_sync_token" "http://$GO_ADDR/dashboard/workRoom/syn" >"$WORK_DIR/go_work_room_sync.json"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_room WHERE id = 900001 AND corp_id = 1 AND wx_chat_id = 'go-room-900001' AND name = 'Go迁移客户群同步更新' AND owner_id = 1 AND notice = 'Go同步群公告' AND status = 0 AND deleted_at IS NULL;" >"$WORK_DIR/work_room_sync_existing_room_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_room room JOIN mc_work_employee employee ON employee.id = room.owner_id WHERE room.corp_id = 1 AND room.wx_chat_id = 'go-room-sync-created' AND room.name = 'Go迁移同步新客户群' AND room.notice = 'Go同步新群公告' AND room.status = 0 AND employee.wx_user_id = 'go-sync-user' AND room.deleted_at IS NULL AND employee.deleted_at IS NULL;" >"$WORK_DIR/work_room_sync_new_room_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT deleted_at IS NOT NULL FROM mc_work_room WHERE id = 900010;" >"$WORK_DIR/work_room_sync_old_room_deleted.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact_room WHERE room_id = 900001 AND wx_user_id = 'go-migrate-user' AND contact_id = 0 AND employee_id = 1 AND type = 1 AND status = 1 AND out_time = '' AND deleted_at IS NULL;" >"$WORK_DIR/work_room_sync_existing_employee_member_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact_room WHERE id = 900001 AND room_id = 900001 AND wx_user_id = 'external-user-900001' AND contact_id = 900001 AND employee_id = 0 AND unionid = 'union-go-900001' AND join_scene = 2 AND type = 2 AND status = 1 AND out_time = '' AND deleted_at IS NULL;" >"$WORK_DIR/work_room_sync_existing_contact_member_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact_room rel JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE rel.room_id = 900001 AND rel.wx_user_id = 'go-sync-contact' AND contact.wx_external_userid = 'go-sync-contact' AND rel.employee_id = 0 AND rel.unionid = 'union-go-sync-contact' AND rel.type = 2 AND rel.status = 1 AND rel.deleted_at IS NULL AND contact.deleted_at IS NULL;" >"$WORK_DIR/work_room_sync_new_existing_room_contact_member_count.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT status = 2 AND out_time <> '' FROM mc_work_contact_room WHERE id = 900011;" >"$WORK_DIR/work_room_sync_old_member_quit.txt"
compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM mc_work_contact_room rel JOIN mc_work_room room ON room.id = rel.room_id JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE room.wx_chat_id = 'go-room-sync-created' AND rel.wx_user_id = 'go-sync-contact' AND contact.wx_external_userid = 'go-sync-contact' AND rel.type = 2 AND rel.status = 1 AND room.deleted_at IS NULL AND rel.deleted_at IS NULL AND contact.deleted_at IS NULL;" >"$WORK_DIR/work_room_sync_new_room_contact_member_count.txt"
python3 - "$WORK_DIR" "$WORK_DIR/wecom_requests.ndjson" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
request_log = pathlib.Path(sys.argv[2])

def text(name):
    return (work / name).read_text(encoding="utf-8").strip()

sync = json.loads(text("go_work_room_sync.json"))
assert sync["code"] == 200 and sync["data"] == [], sync
assert text("work_room_sync_existing_room_count.txt") == "1"
assert text("work_room_sync_new_room_count.txt") == "1"
assert text("work_room_sync_old_room_deleted.txt") == "1"
assert text("work_room_sync_existing_employee_member_count.txt") == "1"
assert text("work_room_sync_existing_contact_member_count.txt") == "1"
assert text("work_room_sync_new_existing_room_contact_member_count.txt") == "1"
assert text("work_room_sync_old_member_quit.txt") == "1"
assert text("work_room_sync_new_room_contact_member_count.txt") == "1"
requests = [json.loads(line) for line in request_log.read_text(encoding="utf-8").splitlines() if line.strip()]
assert any("/cgi-bin/externalcontact/groupchat/list" in item["path"] for item in requests), requests
assert any("/cgi-bin/externalcontact/groupchat/get" in item["path"] and "go-room-sync-created" in item["body"] for item in requests), requests
print("work room sync smoke passed")
PY
