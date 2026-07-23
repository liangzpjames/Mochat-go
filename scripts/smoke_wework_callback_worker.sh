#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-wework-worker-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18086}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13318}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26391}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19053}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-wework-worker.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
WECOM_PID=""
GO_PID=""

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
  [ -f "$GO_LOG" ] && tail -80 "$GO_LOG" >&2 || true
  [ -f "$WECOM_LOG" ] && tail -80 "$WECOM_LOG" >&2 || true
  exit 1
}

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "$1" | tr -d '\r'
}

redis_scalar() {
  compose exec -T redis redis-cli --raw "$@" | tr -d '\r'
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
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  [ -f "$WECOM_LOG" ] && tail -120 "$WECOM_LOG" >&2 || true
  exit 1
}

wait_redis_scalar() {
  local expected="$1"
  shift
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local value
    value="$(redis_scalar "$@" || true)"
    if [ "$value" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for Redis command to return $expected" >&2
  echo "redis-cli $*" >&2
  echo "last value: $(redis_scalar "$@" || true)" >&2
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  [ -f "$WECOM_LOG" ] && tail -120 "$WECOM_LOG" >&2 || true
  exit 1
}

wait_file_contains() {
  local pattern="$1"
  local file="$2"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if [ -f "$file" ] && grep -q "$pattern" "$file"; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $file to contain $pattern" >&2
  [ -f "$file" ] && tail -120 "$file" >&2 || true
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
}

assert_port_free "$GO_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"
assert_port_free "$WECOM_ADDR"

cat >"$WORK_DIR/fake_wecom.py" <<'PY'
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse


class Handler(BaseHTTPRequestHandler):
    user_get_calls = 0
    tag_get_calls = 0
    contact_get_calls = 0

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
        if path.path == "/healthz":
            self.json_response({"ok": True})
            return
        if path.path == "/cgi-bin/gettoken":
            self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "fake-token", "expires_in": 7200})
            return
        if path.path == "/cgi-bin/department/list":
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "department": [
                    {"id": 1, "name": "Go回调总部", "parentid": 0, "order": 100},
                    {"id": 2, "name": "Go回调销售部", "parentid": 1, "order": 90}
                ]
            })
            return
        if path.path == "/cgi-bin/user/list":
            department_id = parse_qs(path.query).get("department_id", [""])[0]
            users = []
            if department_id in {"1", "2"}:
                users.append({
                    "userid": "go-worker-user",
                    "name": "Go回调员工",
                    "mobile": "13900000000",
                    "position": "回调同步",
                    "gender": "1",
                    "email": "worker@example.com",
                    "avatar": "https://wecom.example/worker.png",
                    "thumb_avatar": "https://wecom.example/worker-thumb.png",
                    "telephone": "",
                    "alias": "worker",
                    "extattr": {"attrs": []},
                    "status": 1,
                    "qr_code": "https://wecom.example/worker-qr.png",
                    "external_profile": {},
                    "external_position": "回调负责人",
                    "address": "",
                    "open_userid": "open-go-worker-user",
                    "main_department": 2,
                    "department": [2],
                    "is_leader_in_dept": [0],
                    "order": [10]
                })
            self.json_response({"errcode": 0, "errmsg": "ok", "userlist": users})
            return
        if path.path == "/cgi-bin/user/get":
            user_id = parse_qs(path.query).get("userid", [""])[0]
            if user_id != "go-worker-user":
                self.json_response({"errcode": 60111, "errmsg": "userid not found"})
                return
            Handler.user_get_calls += 1
            name = "Go回调员工" if Handler.user_get_calls == 1 else "Go回调员工更新"
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "userid": "go-worker-user",
                "name": name,
                "mobile": "13900000000",
                "position": "回调同步",
                "gender": "1",
                "email": "worker@example.com",
                "avatar": "https://wecom.example/worker.png",
                "thumb_avatar": "https://wecom.example/worker-thumb.png",
                "telephone": "",
                "alias": "worker",
                "extattr": {"attrs": []},
                "status": 1,
                "qr_code": "https://wecom.example/worker-qr.png",
                "external_profile": {},
                "external_position": "回调负责人",
                "address": "",
                "open_userid": "open-go-worker-user",
                "main_department": 2,
                "department": [2],
                "is_leader_in_dept": [0],
                "order": [10]
            })
            return
        if path.path == "/cgi-bin/externalcontact/get_follow_user_list":
            self.json_response({"errcode": 0, "errmsg": "ok", "follow_user": ["go-worker-user"]})
            return
        if path.path == "/cgi-bin/externalcontact/get":
            external_userid = parse_qs(path.query).get("external_userid", [""])[0]
            if external_userid == "external-auto-pull":
                self.json_response({
                    "errcode": 0,
                    "errmsg": "ok",
                    "external_contact": {
                        "external_userid": "external-auto-pull",
                        "name": "Go回调自动拉群客户",
                        "avatar": "https://wecom.example/contact-auto-pull.png",
                        "type": 1,
                        "gender": 1,
                        "unionid": "union-external-auto-pull",
                        "position": "",
                        "corp_name": "",
                        "corp_full_name": "",
                        "external_profile": {},
                        "business_no": "GO-AUTO-PULL-1"
                    },
                    "follow_user": [
                        {
                            "userid": "go-worker-user",
                            "remark": "自动拉群客户备注",
                            "description": "自动拉群客户描述",
                            "remark_corp_name": "Go回调企业备注",
                            "remark_mobiles": ["13900000001"],
                            "add_way": 2,
                            "oper_userid": "go-worker-user",
                            "state": "workRoomAutoPullId-901",
                            "createtime": 1710000300,
                            "tags": []
                        }
                    ]
                })
                return
            if external_userid == "external-fission":
                self.json_response({
                    "errcode": 0,
                    "errmsg": "ok",
                    "external_contact": {
                        "external_userid": "external-fission",
                        "name": "Go回调裂变客户",
                        "avatar": "https://wecom.example/contact-fission.png",
                        "type": 1,
                        "gender": 1,
                        "unionid": "union-external-fission",
                        "position": "",
                        "corp_name": "",
                        "corp_full_name": "",
                        "external_profile": {},
                        "business_no": "GO-FISSION-1"
                    },
                    "follow_user": [
                        {
                            "userid": "go-worker-user",
                            "remark": "裂变客户备注",
                            "description": "裂变客户描述",
                            "remark_corp_name": "Go回调企业备注",
                            "remark_mobiles": ["13900000002"],
                            "add_way": 2,
                            "oper_userid": "go-worker-user",
                            "state": "fission-903",
                            "createtime": 1710000400,
                            "tags": []
                        }
                    ]
                })
                return
            if external_userid != "external-callback":
                self.json_response({"errcode": 84061, "errmsg": "external user not found"})
                return
            Handler.contact_get_calls += 1
            if Handler.contact_get_calls == 1:
                name = "Go回调客户"
                remark = "Go回调客户备注"
                description = "Go回调客户描述"
                business_no = "GO-CALLBACK-1"
                add_way = 2
            elif Handler.contact_get_calls == 2:
                name = "Go回调客户更新"
                remark = "Go回调客户备注更新"
                description = "Go回调客户描述更新"
                business_no = "GO-CALLBACK-2"
                add_way = 3
            else:
                name = "Go回调客户恢复"
                remark = "Go回调客户备注恢复"
                description = "Go回调客户描述恢复"
                business_no = "GO-CALLBACK-3"
                add_way = 4
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "external_contact": {
                    "external_userid": "external-callback",
                    "name": name,
                    "avatar": "https://wecom.example/contact-callback.png",
                    "type": 1,
                    "gender": 1,
                    "unionid": "union-external-callback",
                    "position": "",
                    "corp_name": "",
                    "corp_full_name": "",
                    "external_profile": {},
                    "business_no": business_no
                },
                "follow_user": [
                    {
                        "userid": "go-worker-user",
                        "remark": remark,
                        "description": description,
                        "remark_corp_name": "Go回调企业备注",
                        "remark_mobiles": ["13900000000"],
                        "add_way": add_way,
                        "oper_userid": "go-worker-user",
                        "state": "callback-state",
                        "createtime": 1710000200,
                        "tags": [
                            {"tag_id": "tag-callback", "group_name": "Go回调标签组", "tag_name": "Go回调标签更新", "type": 1}
                        ]
                    }
                ]
            })
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        path = urlparse(self.path)
        length = int(self.headers.get("Content-Length", "0") or "0")
        raw = self.rfile.read(length) if length else b"{}"
        if path.path == "/cgi-bin/media/upload":
            print("UPLOAD_TEMP_IMAGE", flush=True)
            self.json_response({"errcode": 0, "errmsg": "ok", "type": "image", "media_id": "auto-pull-room-media"})
            return
        payload = json.loads(raw.decode("utf-8") or "{}")
        if path.path == "/cgi-bin/externalcontact/send_welcome_msg":
            welcome_code = payload.get("welcome_code")
            if welcome_code == "auto-pull-welcome-code":
                assert payload.get("text", {}).get("content") == "自动拉群Go回调自动拉群客户", payload
                attachments = payload.get("attachments") or []
                assert attachments and attachments[0].get("msgtype") == "image", payload
                assert attachments[0].get("image", {}).get("media_id") == "auto-pull-room-media", payload
                print("SEND_AUTO_PULL_WELCOME " + json.dumps(payload, ensure_ascii=False), flush=True)
                self.json_response({"errcode": 0, "errmsg": "ok"})
                return
            if welcome_code == "fission-welcome-code":
                assert payload.get("text", {}).get("content") == "裂变欢迎Go回调裂变客户", payload
                attachments = payload.get("attachments") or []
                assert attachments and attachments[0].get("msgtype") == "link", payload
                link = attachments[0].get("link") or {}
                assert link.get("title") == "裂变任务", payload
                assert link.get("desc") == "查看助力进度", payload
                assert "/auth/workFission?id=903" in link.get("url", ""), payload
                assert "target=%2FworkFission%3Fid%3D903" in link.get("url", ""), payload
                print("SEND_FISSION_WELCOME " + json.dumps(payload, ensure_ascii=False), flush=True)
                self.json_response({"errcode": 0, "errmsg": "ok"})
                return
            assert welcome_code == "welcome-code", payload
            assert payload.get("text", {}).get("content") == "渠道欢迎Go回调客户", payload
            attachments = payload.get("attachments") or []
            assert attachments and attachments[0].get("msgtype") == "link", payload
            print("SEND_WELCOME " + json.dumps(payload, ensure_ascii=False), flush=True)
            self.json_response({"errcode": 0, "errmsg": "ok"})
            return
        if path.path == "/cgi-bin/externalcontact/mark_tag":
            assert payload.get("userid") == "go-worker-user", payload
            if payload.get("external_userid") == "external-callback":
                assert payload.get("add_tag") == ["wx-channel-config-tag"], payload
                print("MARK_CHANNEL_TAG " + json.dumps(payload, ensure_ascii=False), flush=True)
            elif payload.get("external_userid") == "external-auto-pull":
                assert payload.get("add_tag") == ["wx-auto-pull-config-tag"], payload
                print("MARK_AUTO_PULL_TAG " + json.dumps(payload, ensure_ascii=False), flush=True)
            elif payload.get("external_userid") == "external-fission":
                assert payload.get("add_tag") == ["wx-fission-config-tag"], payload
                print("MARK_FISSION_TAG " + json.dumps(payload, ensure_ascii=False), flush=True)
            else:
                raise AssertionError(payload)
            self.json_response({"errcode": 0, "errmsg": "ok"})
            return
        if path.path == "/cgi-bin/message/send":
            assert payload.get("agentid") == "100001", payload
            assert payload.get("msgtype") == "text", payload
            content = payload.get("text", {}).get("content")
            if payload.get("touser") == "service-a|service-b":
                assert content == "客户【Go回调裂变客户】已完成裂变任务：Go裂变活动", payload
                print("SEND_FISSION_EMPLOYEE_REMINDER " + json.dumps(payload, ensure_ascii=False), flush=True)
            elif payload.get("touser") == "go-worker-user":
                assert content.startswith("【删除提醒】您已将客户删除哦！"), payload
                assert "客户昵称：" in content, payload
                assert "/contact?agentId={{agentId}}&contactId=" in content, payload
                print("SEND_CONTACT_DELETE_REMINDER " + json.dumps(payload, ensure_ascii=False), flush=True)
            else:
                raise AssertionError(payload)
            self.json_response({"errcode": 0, "errmsg": "ok"})
            return
        if path.path == "/cgi-bin/externalcontact/add_msg_template":
            assert payload.get("chat_type") == "single", payload
            assert payload.get("sender") == "go-worker-user", payload
            assert payload.get("external_userid") == ["external-fission-parent"], payload
            assert payload.get("text", {}).get("content") == "裂变完成%NICKNAME%", payload
            attachments = payload.get("attachments") or []
            assert attachments and attachments[0].get("msgtype") == "image", payload
            assert attachments[0].get("image", {}).get("media_id") == "auto-pull-room-media", payload
            print("SEND_FISSION_CUSTOMER_PUSH " + json.dumps(payload, ensure_ascii=False), flush=True)
            self.json_response({"errcode": 0, "errmsg": "ok", "msgid": "msg-fission-customer-push"})
            return
        if path.path == "/cgi-bin/externalcontact/get_corp_tag_list":
            tag_ids = payload.get("tag_id") or []
            if "tag-callback" in tag_ids:
                Handler.tag_get_calls += 1
            tag_name = "Go回调标签" if Handler.tag_get_calls <= 1 else "Go回调标签更新"
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "tag_group": [
                    {
                        "group_id": "tag-group-callback",
                        "group_name": "Go回调标签组",
                        "order": 30,
                        "tag": [
                            {"id": "tag-callback", "name": tag_name, "order": 31}
                        ]
                    }
                ]
            })
            return
        if path.path == "/cgi-bin/externalcontact/groupchat/list":
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "group_chat_list": [
                    {"chat_id": "room-callback", "status": 1},
                    {"chat_id": "room-stay", "status": 0}
                ],
                "next_cursor": ""
            })
            return
        if path.path == "/cgi-bin/externalcontact/groupchat/get":
            assert payload.get("chat_id") == "room-callback", payload
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "group_chat": {
                    "chat_id": "room-callback",
                    "name": "Go回调单群更新",
                    "owner": "go-worker-user",
                    "notice": "只同步回调指定客户群",
                    "create_time": 1710000000,
                    "member_list": [
                        {"userid": "go-worker-user", "type": 1, "join_time": 1710000010, "join_scene": 1, "unionid": ""},
                        {"userid": "external-stay", "type": 2, "join_time": 1710000020, "join_scene": 3, "unionid": "union-external-stay"}
                    ]
                }
            })
            return
        self.send_response(404)
        self.end_headers()


host = sys.argv[1]
port = int(sys.argv[2])
ThreadingHTTPServer((host, port), Handler).serve_forever()
PY

python3 "$WORK_DIR/fake_wecom.py" "${WECOM_ADDR%:*}" "${WECOM_ADDR##*:}" >"$WECOM_LOG" 2>&1 &
WECOM_PID="$!"
wait_url "http://$WECOM_ADDR/healthz" 200

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
INSERT INTO mc_corp (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES (1, 'Go回调企业', 'ww-worker', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  wx_corpid = VALUES(wx_corpid),
  employee_secret = VALUES(employee_secret),
  contact_secret = VALUES(contact_secret),
  token = VALUES(token),
  encoding_aes_key = VALUES(encoding_aes_key),
  tenant_id = VALUES(tenant_id),
  updated_at = NOW(),
  deleted_at = NULL;
SQL

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

mkdir -p "$WORK_DIR/upload/room" "$WORK_DIR/upload/fission"
printf 'fake room qrcode image' >"$WORK_DIR/upload/room/qrcode-auto-pull.png"
printf 'fake fission push image' >"$WORK_DIR/upload/fission/push-image.png"

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="worker-secret" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_FILE_STORAGE_ROOT="$WORK_DIR/upload" \
  MOCHAT_GO_ENABLE_WEWORK_CALLBACK_WORKER=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
curl -sS -f "http://$GO_ADDR/compat/status" >"$WORK_DIR/status.json"
python3 - "$WORK_DIR/status.json" <<'PY'
import json
import pathlib
import sys

status = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tasks = {task.get("name"): task for task in status.get("background_tasks", [])}
for name in ("wework-callback", "contact-welcome"):
    task = tasks.get(name)
    assert task, status
    assert task.get("status") == "running", task
    assert task.get("started_at"), task
PY

cat >"$WORK_DIR/event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_contact.create_user","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_contact","ChangeType":"create_user","UserID":"go-worker-user"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:00"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_department WHERE corp_id = 1 AND wx_department_id = 2 AND name = 'Go回调销售部' AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_employee we JOIN mc_work_department wd ON wd.id = we.main_department_id WHERE we.corp_id = 1 AND we.wx_user_id = 'go-worker-user' AND we.name = 'Go回调员工' AND we.mobile = '13900000000' AND we.contact_auth = 1 AND wd.wx_department_id = 2 AND we.deleted_at IS NULL AND wd.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_user WHERE phone = '13900000000' AND tenant_id = 1 AND password <> '' AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_employee_department wed JOIN mc_work_employee we ON we.id = wed.employee_id JOIN mc_work_department wd ON wd.id = wed.department_id WHERE we.wx_user_id = 'go-worker-user' AND wd.wx_department_id = 2 AND wed.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_update_time WHERE corp_id = 1 AND type = 1 AND last_update_time IS NOT NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/update-user-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_contact.update_user","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_contact","ChangeType":"update_user","UserID":"go-worker-user"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:01"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/update-user-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_employee WHERE corp_id = 1 AND wx_user_id = 'go-worker-user' AND name = 'Go回调员工更新' AND deleted_at IS NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/create-party-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_contact.create_party","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_contact","ChangeType":"create_party","Id":"3","Name":"Go回调售后部","ParentId":"1","Order":"70"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:02"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/create-party-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_department child JOIN mc_work_department parent ON parent.id = child.parent_id WHERE child.corp_id = 1 AND child.wx_department_id = 3 AND child.name = 'Go回调售后部' AND child.wx_parentid = 1 AND child.\`order\` = 70 AND child.level = 1 AND child.path <> '' AND child.deleted_at IS NULL AND parent.wx_department_id = 1 AND parent.deleted_at IS NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/update-party-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_contact.update_party","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_contact","ChangeType":"update_party","Id":"3","Name":"Go回调售后部更新","ParentId":"2","Order":"60"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:03"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/update-party-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_department child JOIN mc_work_department parent ON parent.id = child.parent_id WHERE child.corp_id = 1 AND child.wx_department_id = 3 AND child.name = 'Go回调售后部更新' AND child.wx_parentid = 2 AND child.\`order\` = 60 AND child.level = 2 AND child.path <> '' AND child.deleted_at IS NULL AND parent.wx_department_id = 2 AND parent.deleted_at IS NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/delete-party-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_contact.delete_party","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_contact","ChangeType":"delete_party","Id":"3"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:04"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/delete-party-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_department WHERE corp_id = 1 AND wx_department_id = 3 AND deleted_at IS NOT NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/create-tag-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_tag.create","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_tag","ChangeType":"create","TagType":"tag","Id":"tag-callback"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:05"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/create-tag-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag tag JOIN mc_work_contact_tag_group grp ON grp.id = tag.contact_tag_group_id WHERE tag.corp_id = 1 AND tag.wx_contact_tag_id = 'tag-callback' AND tag.name = 'Go回调标签' AND tag.\`order\` = 31 AND tag.deleted_at IS NULL AND grp.wx_group_id = 'tag-group-callback' AND grp.group_name = 'Go回调标签组' AND grp.deleted_at IS NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/update-tag-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_tag.update","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_tag","ChangeType":"update","TagType":"tag","Id":"tag-callback"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:06"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/update-tag-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag WHERE corp_id = 1 AND wx_contact_tag_id = 'tag-callback' AND name = 'Go回调标签更新' AND deleted_at IS NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
INSERT INTO mc_medium (
  id, media_id, last_upload_time, type, is_sync, content, corp_id, medium_group_id, user_id, user_name, created_at, updated_at, deleted_at
) VALUES (
  900, '', 0, 3, 1, JSON_OBJECT('title', '渠道资料', 'imageLink', 'https://example.com/channel', 'description', '渠道说明'), 1, 0, 0, '', NOW(), NOW(), NULL
) ON DUPLICATE KEY UPDATE
  type = VALUES(type),
  content = VALUES(content),
  corp_id = VALUES(corp_id),
  updated_at = NOW(),
  deleted_at = NULL;
INSERT INTO mc_work_contact_tag_group (
  id, corp_id, wx_group_id, group_name, `order`, created_at, updated_at, deleted_at
) VALUES (
  992, 1, 'wx-channel-config-group', '渠道配置标签组', 992, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  group_name = VALUES(group_name),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_contact_tag (
  id, wx_contact_tag_id, corp_id, name, `order`, contact_tag_group_id, created_at, updated_at, deleted_at
) VALUES (
  992, 'wx-channel-config-tag', 1, '渠道配置标签', 992, 992, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  wx_contact_tag_id = VALUES(wx_contact_tag_id),
  name = VALUES(name),
  contact_tag_group_id = VALUES(contact_tag_group_id),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_channel_code (
  id, corp_id, group_id, name, qrcode_url, wx_config_id, auto_add_friend, tags, type, drainage_employee, welcome_message, created_at, updated_at, deleted_at
) VALUES (
  900, 1, 0, 'Go回调渠道码', '', '', 1, JSON_ARRAY(992), 1, JSON_OBJECT(),
  JSON_OBJECT(
    'scanCodePush', 1,
    'messageDetail', JSON_ARRAY(JSON_OBJECT('type', 1, 'welcomeContent', '渠道欢迎##客户名称##', 'mediumId', 900))
  ),
  NOW(), NOW(), NULL
) ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  welcome_message = VALUES(welcome_message),
  updated_at = NOW(),
  deleted_at = NULL;
INSERT INTO mc_work_room (
  id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, created_at, updated_at, deleted_at
) VALUES
  (901, 1, 'room-auto-full', '已满自动拉群', 0, '', 0, '2026-07-04 00:00:00', 200, NOW(), NOW(), NULL),
  (902, 1, 'room-auto-open', '可用自动拉群', 0, '', 0, '2026-07-04 00:00:00', 200, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  room_max = VALUES(room_max),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_contact_room (
  wx_user_id, contact_id, employee_id, unionid, room_id, join_scene, type, status, join_time, out_time, created_at, updated_at, deleted_at
) VALUES
  ('member-full-1', 0, 0, '', 901, 3, 1, 1, NOW(), '', NOW(), NOW(), NULL),
  ('member-full-2', 0, 0, '', 901, 3, 1, 1, NOW(), '', NOW(), NOW(), NULL);
INSERT INTO mc_work_contact_tag_group (
  id, corp_id, wx_group_id, group_name, `order`, created_at, updated_at, deleted_at
) VALUES (
  990, 1, 'wx-auto-pull-config-group', '自动拉群配置标签组', 990, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  group_name = VALUES(group_name),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_contact_tag (
  id, wx_contact_tag_id, corp_id, name, `order`, contact_tag_group_id, created_at, updated_at, deleted_at
) VALUES (
  990, 'wx-auto-pull-config-tag', 1, '自动拉群配置标签', 990, 990, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  wx_contact_tag_id = VALUES(wx_contact_tag_id),
  name = VALUES(name),
  contact_tag_group_id = VALUES(contact_tag_group_id),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_contact_tag_group (
  id, corp_id, wx_group_id, group_name, `order`, created_at, updated_at, deleted_at
) VALUES (
  991, 1, 'wx-fission-config-group', '裂变配置标签组', 991, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  group_name = VALUES(group_name),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_contact_tag (
  id, wx_contact_tag_id, corp_id, name, `order`, contact_tag_group_id, created_at, updated_at, deleted_at
) VALUES (
  991, 'wx-fission-config-tag', 1, '裂变配置标签', 991, 991, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  wx_contact_tag_id = VALUES(wx_contact_tag_id),
  name = VALUES(name),
  contact_tag_group_id = VALUES(contact_tag_group_id),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_agent (
  id, corp_id, wx_agent_id, wx_secret, name, square_logo_url, description, close, redirect_domain, report_location_flag, is_reportenter, home_url, created_at, updated_at, deleted_at
) VALUES (
  9901, 1, '100001', 'agent-secret', '裂变提醒应用', '', '', 0, '', 0, 0, '', NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  wx_agent_id = VALUES(wx_agent_id),
  wx_secret = VALUES(wx_secret),
  name = VALUES(name),
  close = VALUES(close),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_fission (
  id, corp_id, active_name, service_employees, auto_pass, auto_add_tag, contact_tags, end_time, qr_code_invalid, tasks, new_friend, delete_invalid, receive_prize, receive_prize_employees, receive_links, receive_qrcode, create_user_id, created_at, updated_at, deleted_at
) VALUES (
  903, 1, 'Go裂变活动', JSON_ARRAY(JSON_OBJECT('wxUserId', 'service-a'), JSON_OBJECT('wxUserId', 'service-b')), 1, 1, JSON_ARRAY(991), '2026-12-31 23:59:59', 0, JSON_ARRAY(JSON_OBJECT('count', 1)), 1, 0, 0, JSON_ARRAY(), JSON_ARRAY(), JSON_OBJECT(), 0, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  service_employees = VALUES(service_employees),
  contact_tags = VALUES(contact_tags),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_fission_contact (
  id, fission_id, union_id, nickname, avatar, contact_superior_user_parent, level, employee, invite_count, loss, status, receive_level, is_new, external_user_id, qrcode_id, qrcode_url, created_at, updated_at, deleted_at
) VALUES (
  903, 903, 'union-fission-parent', '裂变上级客户', '', 0, 0, 'go-worker-user', 0, 0, 0, 0, 1, 'external-fission-parent', '', '', NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  fission_id = VALUES(fission_id),
  contact_superior_user_parent = VALUES(contact_superior_user_parent),
  level = VALUES(level),
  employee = VALUES(employee),
  invite_count = VALUES(invite_count),
  loss = VALUES(loss),
  status = VALUES(status),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_fission_welcome (
  id, fission_id, msg_text, link_title, link_desc, link_cover_url, link_wx_url, created_at, updated_at, deleted_at
) VALUES (
  903, 903, '裂变欢迎##客户名称##', '裂变任务', '查看助力进度', 'fission/welcome-cover.png', '', NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  msg_text = VALUES(msg_text),
  link_title = VALUES(link_title),
  link_desc = VALUES(link_desc),
  link_cover_url = VALUES(link_cover_url),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_fission_push (
  id, fission_id, push_employee, push_contact, msg_text, msg_complex, msg_complex_type, created_at, updated_at, deleted_at
) VALUES (
  903, 903, 1, 1, '裂变完成[用户昵称]', JSON_OBJECT('image', 'fission/push-image.png'), 'image', NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  push_employee = VALUES(push_employee),
  push_contact = VALUES(push_contact),
  msg_text = VALUES(msg_text),
  msg_complex = VALUES(msg_complex),
  msg_complex_type = VALUES(msg_complex_type),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_room_auto_pull (
  id, corp_id, qrcode_name, qrcode_url, wx_config_id, is_verified, leading_words, tags, employees, rooms, created_at, updated_at, deleted_at
) VALUES (
  901, 1, 'Go自动拉群', '', '', 2, '自动拉群##客户名称##', JSON_ARRAY(990), JSON_ARRAY(),
  JSON_ARRAY(
    JSON_OBJECT('roomId', 901, 'maxNum', 2, 'roomQrcodeUrl', 'room/full.png'),
    JSON_OBJECT('roomId', 902, 'maxNum', 80, 'roomQrcodeUrl', 'room/qrcode-auto-pull.png')
  ),
  NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  leading_words = VALUES(leading_words),
  rooms = VALUES(rooms),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_contact (
  corp_id, wx_external_userid, name, avatar, type, gender, unionid, position, corp_name, corp_full_name, external_profile, business_no, created_at, updated_at, deleted_at
) VALUES (
  1, 'external-stay', '不应删除客户', '', 1, 0, 'union-external-stay', '', '', '', JSON_OBJECT(), 'STAY-1', NOW(), NOW(), NULL
);
INSERT INTO mc_work_contact_employee (
  employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles, add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at, deleted_at
)
SELECT employee.id, contact.id, '不应删除关系', '', '', JSON_ARRAY(), 1, 'go-worker-user', '', 1, 1, NOW(), NOW(), NOW(), NULL
FROM mc_work_employee employee
JOIN mc_work_contact contact ON contact.corp_id = employee.corp_id AND contact.wx_external_userid = 'external-stay'
WHERE employee.corp_id = 1 AND employee.wx_user_id = 'go-worker-user' AND employee.deleted_at IS NULL AND contact.deleted_at IS NULL
LIMIT 1;
INSERT INTO mc_work_contact_tag_group (
  id, corp_id, wx_group_id, group_name, `order`, created_at, updated_at, deleted_at
) VALUES (
  994, 1, 'wx-contact-time-auto-tag-group', '分时段自动标签组', 994, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  group_name = VALUES(group_name),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_contact_tag (
  id, wx_contact_tag_id, corp_id, name, `order`, contact_tag_group_id, created_at, updated_at, deleted_at
) VALUES (
  994, 'wx-contact-time-auto-tag', 1, '分时段行为标签', 994, 994, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  wx_contact_tag_id = VALUES(wx_contact_tag_id),
  name = VALUES(name),
  contact_tag_group_id = VALUES(contact_tag_group_id),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_auto_tag (
  id, type, name, employees, fuzzy_match_keyword, exact_match_keyword, tag_rule, tags, on_off, mark_tag_count, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at
) VALUES (
  9901, 3, 'Go回调分时段自动标签', JSON_ARRAY(), JSON_ARRAY(), JSON_ARRAY(),
  JSON_ARRAY(
    JSON_OBJECT(
      'id', 1,
      'time_type', 1,
      'start_time', '00:03:00',
      'end_time', '00:03:59',
      'schedule', JSON_ARRAY(),
      'tags', JSON_ARRAY(JSON_OBJECT('tagid', 994, 'tagname', '分时段行为标签'))
    ),
    JSON_OBJECT(
      'id', 2,
      'time_type', 1,
      'start_time', '16:03:00',
      'end_time', '16:03:59',
      'schedule', JSON_ARRAY(),
      'tags', JSON_ARRAY(JSON_OBJECT('tagid', 994, 'tagname', '分时段行为标签'))
    )
  ),
  JSON_ARRAY(994), 1, 0, 1, 1, 1, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  tag_rule = VALUES(tag_rule),
  tags = VALUES(tags),
  on_off = VALUES(on_off),
  deleted_at = NULL,
  updated_at = NOW();
SQL

cat >"$WORK_DIR/add-contact-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_contact.add_external_contact","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_contact","ChangeType":"add_external_contact","UserID":"go-worker-user","ExternalUserID":"external-callback","State":"channelCode-900","WelcomeCode":"welcome-code"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:07"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/add-contact-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact WHERE corp_id = 1 AND wx_external_userid = 'external-callback' AND name = 'Go回调客户' AND business_no = 'GO-CALLBACK-1' AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_employee rel JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE rel.corp_id = 1 AND contact.wx_external_userid = 'external-callback' AND rel.employee_id = (SELECT id FROM mc_work_employee WHERE corp_id = 1 AND wx_user_id = 'go-worker-user' LIMIT 1) AND rel.remark = 'Go回调客户备注' AND rel.description = 'Go回调客户描述' AND rel.add_way = 2 AND rel.status = 1 AND rel.deleted_at IS NULL AND contact.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag_pivot pivot JOIN mc_work_contact contact ON contact.id = pivot.contact_id JOIN mc_work_contact_tag tag ON tag.id = pivot.contact_tag_id WHERE contact.wx_external_userid = 'external-callback' AND tag.wx_contact_tag_id = 'tag-callback' AND pivot.employee_id = (SELECT id FROM mc_work_employee WHERE corp_id = 1 AND wx_user_id = 'go-worker-user' LIMIT 1) AND pivot.deleted_at IS NULL AND contact.deleted_at IS NULL AND tag.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag_pivot pivot JOIN mc_work_contact contact ON contact.id = pivot.contact_id JOIN mc_work_contact_tag tag ON tag.id = pivot.contact_tag_id WHERE contact.wx_external_userid = 'external-callback' AND tag.wx_contact_tag_id = 'wx-channel-config-tag' AND pivot.employee_id = (SELECT id FROM mc_work_employee WHERE corp_id = 1 AND wx_user_id = 'go-worker-user' LIMIT 1) AND pivot.type = 1 AND pivot.deleted_at IS NULL AND contact.deleted_at IS NULL AND tag.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_contact_employee_track track JOIN mc_work_contact contact ON contact.id = track.contact_id WHERE contact.wx_external_userid = 'external-callback' AND track.employee_id = (SELECT id FROM mc_work_employee WHERE corp_id = 1 AND wx_user_id = 'go-worker-user' LIMIT 1) AND track.corp_id = 1 AND track.event = 2 AND track.content LIKE '%渠道配置标签%';" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_employee rel JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE rel.corp_id = 1 AND contact.wx_external_userid = 'external-stay' AND rel.deleted_at IS NULL AND contact.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_auto_tag_record record JOIN mc_work_contact contact ON contact.id = record.contact_id WHERE record.corp_id = 1 AND record.auto_tag_id = 9901 AND record.tag_rule_id IN (1, 2) AND contact.wx_external_userid = 'external-callback' AND record.employee_id = (SELECT id FROM mc_work_employee WHERE corp_id = 1 AND wx_user_id = 'go-worker-user' LIMIT 1) AND record.status = 0 AND CAST(record.tags AS CHAR) LIKE '%994%' AND record.deleted_at IS NULL AND contact.deleted_at IS NULL;" "1"
wait_redis_scalar "1" LLEN mochat-go:mark-tags
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_update_time WHERE corp_id = 1 AND type = 2 AND last_update_time IS NOT NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead
wait_mysql_scalar "SELECT IF(COUNT(*) >= 1, 1, 0) FROM mochat_go_background_task_executions WHERE tenant_id = 1 AND task_name = 'contact-welcome' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"
wait_redis_scalar "0" LLEN mochat-go:contact-welcome
wait_redis_scalar "0" LLEN mochat-go:contact-welcome:processing
wait_redis_scalar "0" LLEN mochat-go:contact-welcome:dead
wait_file_contains "MARK_CHANNEL_TAG" "$WECOM_LOG"
wait_file_contains "渠道欢迎Go回调客户" "$WECOM_LOG"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
INSERT INTO mc_contact_sop (
  id, corp_id, creator_id, name, setting, employee_ids, state, contact_ids, created_at, updated_at
)
SELECT
  9903, 1, 1, 'Go回调事件个人SOP',
  '{"content":[{"type":"text","value":"Go回调事件个人SOP提醒"}],"delayMinutes":0}',
  CONCAT('[', employee.id, ']'), 1, CONCAT('[', contact.id, ']'), NOW(), NOW()
FROM mc_work_employee employee
JOIN mc_work_contact contact ON contact.corp_id = employee.corp_id AND contact.wx_external_userid = 'external-callback'
WHERE employee.corp_id = 1 AND employee.wx_user_id = 'go-worker-user'
LIMIT 1
ON DUPLICATE KEY UPDATE
  setting = VALUES(setting),
  employee_ids = VALUES(employee_ids),
  contact_ids = VALUES(contact_ids),
  state = VALUES(state),
  updated_at = NOW();
SQL

cat >"$WORK_DIR/auto-pull-contact-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_contact.add_external_contact","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_contact","ChangeType":"add_external_contact","UserID":"go-worker-user","ExternalUserID":"external-auto-pull","State":"workRoomAutoPullId-901","WelcomeCode":"auto-pull-welcome-code"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:07"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/auto-pull-contact-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact WHERE corp_id = 1 AND wx_external_userid = 'external-auto-pull' AND name = 'Go回调自动拉群客户' AND business_no = 'GO-AUTO-PULL-1' AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_employee rel JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE rel.corp_id = 1 AND contact.wx_external_userid = 'external-auto-pull' AND rel.state = 'workRoomAutoPullId-901' AND rel.status = 1 AND rel.deleted_at IS NULL AND contact.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag_pivot pivot JOIN mc_work_contact contact ON contact.id = pivot.contact_id JOIN mc_work_contact_tag tag ON tag.id = pivot.contact_tag_id WHERE contact.wx_external_userid = 'external-auto-pull' AND tag.wx_contact_tag_id = 'wx-auto-pull-config-tag' AND pivot.employee_id = (SELECT id FROM mc_work_employee WHERE corp_id = 1 AND wx_user_id = 'go-worker-user' LIMIT 1) AND pivot.type = 1 AND pivot.deleted_at IS NULL AND contact.deleted_at IS NULL AND tag.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_contact_employee_track track JOIN mc_work_contact contact ON contact.id = track.contact_id WHERE contact.wx_external_userid = 'external-auto-pull' AND track.employee_id = (SELECT id FROM mc_work_employee WHERE corp_id = 1 AND wx_user_id = 'go-worker-user' LIMIT 1) AND track.corp_id = 1 AND track.event = 2 AND track.content LIKE '%自动拉群配置标签%';" "1"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 2, 1, 0) FROM mochat_go_background_task_executions WHERE tenant_id = 1 AND task_name = 'contact-welcome' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"
wait_redis_scalar "0" LLEN mochat-go:contact-welcome
wait_redis_scalar "0" LLEN mochat-go:contact-welcome:processing
wait_redis_scalar "0" LLEN mochat-go:contact-welcome:dead
wait_file_contains "MARK_AUTO_PULL_TAG" "$WECOM_LOG"
wait_file_contains "SEND_AUTO_PULL_WELCOME" "$WECOM_LOG"

cat >"$WORK_DIR/fission-contact-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_contact.add_external_contact","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_contact","ChangeType":"add_external_contact","UserID":"go-worker-user","ExternalUserID":"external-fission","State":"fission-903","WelcomeCode":"fission-welcome-code"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:07"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/fission-contact-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact WHERE corp_id = 1 AND wx_external_userid = 'external-fission' AND name = 'Go回调裂变客户' AND business_no = 'GO-FISSION-1' AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_employee rel JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE rel.corp_id = 1 AND contact.wx_external_userid = 'external-fission' AND rel.state = 'fission-903' AND rel.status = 1 AND rel.deleted_at IS NULL AND contact.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag_pivot pivot JOIN mc_work_contact contact ON contact.id = pivot.contact_id JOIN mc_work_contact_tag tag ON tag.id = pivot.contact_tag_id WHERE contact.wx_external_userid = 'external-fission' AND tag.wx_contact_tag_id = 'wx-fission-config-tag' AND pivot.employee_id = (SELECT id FROM mc_work_employee WHERE corp_id = 1 AND wx_user_id = 'go-worker-user' LIMIT 1) AND pivot.type = 1 AND pivot.deleted_at IS NULL AND contact.deleted_at IS NULL AND tag.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_contact_employee_track track JOIN mc_work_contact contact ON contact.id = track.contact_id WHERE contact.wx_external_userid = 'external-fission' AND track.employee_id = (SELECT id FROM mc_work_employee WHERE corp_id = 1 AND wx_user_id = 'go-worker-user' LIMIT 1) AND track.corp_id = 1 AND track.event = 2 AND track.content LIKE '%裂变配置标签%';" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_contact WHERE fission_id = 903 AND contact_superior_user_parent = 903 AND external_user_id = 'external-fission' AND union_id = 'union-external-fission' AND nickname = 'Go回调裂变客户' AND level = 2 AND employee = 'go-worker-user' AND is_new = 1 AND loss = 0 AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_contact WHERE id = 903 AND fission_id = 903 AND level = 1 AND invite_count = 1 AND employee = 'go-worker-user' AND status = 1 AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 3, 1, 0) FROM mochat_go_background_task_executions WHERE tenant_id = 1 AND task_name = 'contact-welcome' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"
wait_redis_scalar "0" LLEN mochat-go:contact-welcome
wait_redis_scalar "0" LLEN mochat-go:contact-welcome:processing
wait_redis_scalar "0" LLEN mochat-go:contact-welcome:dead
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead
wait_file_contains "MARK_FISSION_TAG" "$WECOM_LOG"
wait_file_contains "SEND_FISSION_WELCOME" "$WECOM_LOG"
wait_file_contains "SEND_FISSION_EMPLOYEE_REMINDER" "$WECOM_LOG"
wait_file_contains "SEND_FISSION_CUSTOMER_PUSH" "$WECOM_LOG"

cat >"$WORK_DIR/delete-fission-contact-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_contact.del_external_contact","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_contact","ChangeType":"del_external_contact","UserID":"go-worker-user","ExternalUserID":"external-fission"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:08"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/delete-fission-contact-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_contact WHERE fission_id = 903 AND contact_superior_user_parent = 903 AND external_user_id = 'external-fission' AND loss = 1 AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_contact WHERE id = 903 AND fission_id = 903 AND invite_count = 0 AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_employee rel JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE rel.corp_id = 1 AND contact.wx_external_userid = 'external-fission' AND rel.status = 2 AND rel.deleted_at IS NOT NULL;" "1"
wait_file_contains "SEND_CONTACT_DELETE_REMINDER" "$WECOM_LOG"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/edit-contact-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_contact.edit_external_contact","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_contact","ChangeType":"edit_external_contact","UserID":"go-worker-user","ExternalUserID":"external-callback"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:09"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/edit-contact-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact WHERE corp_id = 1 AND wx_external_userid = 'external-callback' AND name = 'Go回调客户更新' AND business_no = 'GO-CALLBACK-2' AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_employee rel JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE rel.corp_id = 1 AND contact.wx_external_userid = 'external-callback' AND rel.remark = 'Go回调客户备注更新' AND rel.description = 'Go回调客户描述更新' AND rel.add_way = 3 AND rel.status = 1 AND rel.deleted_at IS NULL AND contact.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_employee rel JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE rel.corp_id = 1 AND contact.wx_external_userid = 'external-stay' AND rel.deleted_at IS NULL AND contact.deleted_at IS NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/delete-contact-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_contact.del_external_contact","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_contact","ChangeType":"del_external_contact","UserID":"go-worker-user","ExternalUserID":"external-callback"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:10"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/delete-contact-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_employee rel JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE rel.corp_id = 1 AND contact.wx_external_userid = 'external-callback' AND rel.status = 2 AND rel.deleted_at IS NOT NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact WHERE corp_id = 1 AND wx_external_userid = 'external-callback' AND deleted_at IS NOT NULL;" "1"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 2, 1, 0) FROM mc_work_contact_tag_pivot pivot JOIN mc_work_contact contact ON contact.id = pivot.contact_id WHERE contact.wx_external_userid = 'external-callback' AND pivot.deleted_at IS NOT NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_employee rel JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE rel.corp_id = 1 AND contact.wx_external_userid = 'external-stay' AND rel.deleted_at IS NULL AND contact.deleted_at IS NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/readd-contact-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_contact.add_external_contact","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_contact","ChangeType":"add_external_contact","UserID":"go-worker-user","ExternalUserID":"external-callback","State":"callback-state"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:11"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/readd-contact-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact WHERE corp_id = 1 AND wx_external_userid = 'external-callback' AND name = 'Go回调客户恢复' AND business_no = 'GO-CALLBACK-3' AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_employee rel JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE rel.corp_id = 1 AND contact.wx_external_userid = 'external-callback' AND rel.remark = 'Go回调客户备注恢复' AND rel.add_way = 4 AND rel.status = 1 AND rel.deleted_at IS NULL AND contact.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag_pivot pivot JOIN mc_work_contact contact ON contact.id = pivot.contact_id JOIN mc_work_contact_tag tag ON tag.id = pivot.contact_tag_id WHERE contact.wx_external_userid = 'external-callback' AND tag.wx_contact_tag_id = 'tag-callback' AND pivot.deleted_at IS NULL AND contact.deleted_at IS NULL AND tag.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_contact_sop_log log JOIN mc_work_contact contact ON contact.wx_external_userid = log.contact WHERE log.corp_id = 1 AND log.contact_sop_id = 9903 AND log.employee = 'go-worker-user' AND log.contact = 'external-callback' AND log.task LIKE '%Go回调事件个人SOP提醒%' AND contact.deleted_at IS NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/delete-follow-contact-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_contact.del_follow_user","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_contact","ChangeType":"del_follow_user","UserID":"go-worker-user","ExternalUserID":"external-callback"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:11"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/delete-follow-contact-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_employee rel JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE rel.corp_id = 1 AND contact.wx_external_userid = 'external-callback' AND rel.status = 3 AND rel.deleted_at IS NOT NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact WHERE corp_id = 1 AND wx_external_userid = 'external-callback' AND deleted_at IS NOT NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_employee rel JOIN mc_work_contact contact ON contact.id = rel.contact_id WHERE rel.corp_id = 1 AND contact.wx_external_userid = 'external-stay' AND rel.deleted_at IS NULL AND contact.deleted_at IS NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/delete-tag-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_tag.delete","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_tag","ChangeType":"delete","TagType":"tag","Id":"tag-callback"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:12"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/delete-tag-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag WHERE corp_id = 1 AND wx_contact_tag_id = 'tag-callback' AND deleted_at IS NOT NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag_group WHERE corp_id = 1 AND wx_group_id = 'tag-group-callback' AND deleted_at IS NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/noop-shuffle-tag-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_tag.shuffle","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_tag","ChangeType":"shuffle"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:13"}
JSON
cat >"$WORK_DIR/noop-update-contact-tag-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_contact.update_tag","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_contact","ChangeType":"update_tag"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:14"}
JSON
cat >"$WORK_DIR/noop-half-contact-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_contact.add_half_external_contact","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_contact","ChangeType":"add_half_external_contact","UserID":"go-worker-user","ExternalUserID":"external-callback"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:15"}
JSON
cat >"$WORK_DIR/noop-transfer-fail-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_contact.transfer_fail","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_contact","ChangeType":"transfer_fail","UserID":"go-worker-user","ExternalUserID":"external-callback"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:16"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/noop-shuffle-tag-event.json" >/dev/null
compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/noop-update-contact-tag-event.json" >/dev/null
compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/noop-half-contact-event.json" >/dev/null
compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/noop-transfer-fail-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact WHERE corp_id = 1 AND wx_external_userid = 'external-callback' AND deleted_at IS NOT NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
INSERT INTO mc_work_room (
  id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, created_at, updated_at, deleted_at
) VALUES
  (903, 1, 'room-callback', '待同步客户群', 0, '', 0, '2026-07-04 00:00:00', 200, NOW(), NOW(), NULL),
  (904, 1, 'room-stay', '不应删除客户群', 0, '', 0, '2026-07-04 00:00:00', 200, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  wx_chat_id = VALUES(wx_chat_id),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_contact_tag_group (
  id, corp_id, wx_group_id, group_name, `order`, created_at, updated_at, deleted_at
) VALUES (
  993, 1, 'wx-room-join-auto-tag-group', '入群自动标签组', 993, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  group_name = VALUES(group_name),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_work_contact_tag (
  id, wx_contact_tag_id, corp_id, name, `order`, contact_tag_group_id, created_at, updated_at, deleted_at
) VALUES (
  993, 'wx-room-join-auto-tag', 1, '入群行为标签', 993, 993, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  wx_contact_tag_id = VALUES(wx_contact_tag_id),
  name = VALUES(name),
  contact_tag_group_id = VALUES(contact_tag_group_id),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_auto_tag (
  id, type, name, employees, fuzzy_match_keyword, exact_match_keyword, tag_rule, tags, on_off, mark_tag_count, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at
) VALUES (
  9900, 2, 'Go回调入群自动标签', JSON_ARRAY(), JSON_ARRAY(), JSON_ARRAY(),
  JSON_ARRAY(JSON_OBJECT(
    'id', 1,
    'rooms', JSON_ARRAY(JSON_OBJECT('id', 903, 'name', 'Go回调单群更新')),
    'tags', JSON_ARRAY(JSON_OBJECT('tagid', 993, 'tagname', '入群行为标签'))
  )),
  JSON_ARRAY(993), 1, 0, 1, 1, 1, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  tag_rule = VALUES(tag_rule),
  tags = VALUES(tags),
  on_off = VALUES(on_off),
  deleted_at = NULL,
  updated_at = NOW();
INSERT INTO mc_room_sop (id, corp_id, creator_id, name, setting, room_ids, state, created_at, updated_at)
VALUES (
  9904, 1, 1, 'Go回调事件入群后群SOP',
  '[{"content":[{"type":"text","value":"Go回调事件入群后群SOP提醒"}],"targetAnchor":"room_join","delayMinutes":0}]',
  '[903]', 1, NOW(), NOW()
)
ON DUPLICATE KEY UPDATE
  setting = VALUES(setting),
  room_ids = VALUES(room_ids),
  state = VALUES(state),
  updated_at = NOW();
SQL

cat >"$WORK_DIR/create-room-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_chat.create","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_chat","ChangeType":"create","ChatId":"room-callback"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:08"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/create-room-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_room room JOIN mc_work_employee employee ON employee.id = room.owner_id WHERE room.corp_id = 1 AND room.wx_chat_id = 'room-callback' AND room.name = 'Go回调单群更新' AND room.notice = '只同步回调指定客户群' AND room.status = 1 AND employee.wx_user_id = 'go-worker-user' AND room.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_room WHERE corp_id = 1 AND wx_chat_id = 'room-stay' AND name = '不应删除客户群' AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_auto_tag_record record JOIN mc_work_contact contact ON contact.id = record.contact_id JOIN mc_work_contact_room contact_room ON contact_room.id = record.contact_room_id WHERE record.corp_id = 1 AND record.auto_tag_id = 9900 AND record.tag_rule_id = 1 AND contact.wx_external_userid = 'external-stay' AND record.employee_id = (SELECT id FROM mc_work_employee WHERE corp_id = 1 AND wx_user_id = 'go-worker-user' LIMIT 1) AND contact_room.room_id = 903 AND record.status = 0 AND CAST(record.tags AS CHAR) LIKE '%993%' AND record.deleted_at IS NULL AND contact.deleted_at IS NULL AND contact_room.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_room_sop_log WHERE corp_id = 1 AND room_sop_id = 9904 AND room_id = 903 AND employee = 'go-worker-user' AND contact = 'external-stay' AND task LIKE '%Go回调事件入群后群SOP提醒%';" "1"
wait_redis_scalar "2" LLEN mochat-go:mark-tags
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/room-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_chat.update","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_chat","ChangeType":"update","ChatId":"room-callback"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:08"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/room-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_room room JOIN mc_work_employee employee ON employee.id = room.owner_id WHERE room.corp_id = 1 AND room.wx_chat_id = 'room-callback' AND room.name = 'Go回调单群更新' AND room.notice = '只同步回调指定客户群' AND room.status = 1 AND employee.wx_user_id = 'go-worker-user' AND room.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_room WHERE corp_id = 1 AND wx_chat_id = 'room-stay' AND name = '不应删除客户群' AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_auto_tag_record record JOIN mc_work_contact contact ON contact.id = record.contact_id JOIN mc_work_contact_room contact_room ON contact_room.id = record.contact_room_id WHERE record.corp_id = 1 AND record.auto_tag_id = 9900 AND record.tag_rule_id = 1 AND contact.wx_external_userid = 'external-stay' AND record.employee_id = (SELECT id FROM mc_work_employee WHERE corp_id = 1 AND wx_user_id = 'go-worker-user' LIMIT 1) AND contact_room.room_id = 903 AND record.status = 0 AND CAST(record.tags AS CHAR) LIKE '%993%' AND record.deleted_at IS NULL AND contact.deleted_at IS NULL AND contact_room.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_room_sop_log WHERE corp_id = 1 AND room_sop_id = 9904 AND room_id = 903 AND employee = 'go-worker-user' AND contact = 'external-stay' AND task LIKE '%Go回调事件入群后群SOP提醒%';" "1"
wait_redis_scalar "2" LLEN mochat-go:mark-tags
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_update_time WHERE corp_id = 1 AND type = 3 AND last_update_time IS NOT NULL;" "1"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 20, 1, 0) FROM mochat_go_background_task_executions WHERE tenant_id = 1 AND task_name = 'wework-callback' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/dismiss-room-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_external_chat.dismiss","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_external_chat","ChangeType":"dismiss","ChatId":"room-callback"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:08"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/dismiss-room-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_room WHERE corp_id = 1 AND wx_chat_id = 'room-callback' AND deleted_at IS NOT NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_room WHERE corp_id = 1 AND wx_chat_id = 'room-stay' AND name = '不应删除客户群' AND deleted_at IS NULL;" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

cat >"$WORK_DIR/delete-user-event.json" <<'JSON'
{"corpId":1,"wxCorpId":"ww-worker","eventPath":"event.change_contact.delete_user","message":{"ToUserName":"ww-worker","MsgType":"event","Event":"change_contact","ChangeType":"delete_user","UserID":"go-worker-user"},"rawXml":"<xml/>","receivedAt":"2026-07-04 00:00:09"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:wework-callback <"$WORK_DIR/delete-user-event.json" >/dev/null

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_employee WHERE corp_id = 1 AND wx_user_id = 'go-worker-user' AND deleted_at IS NOT NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_employee_department wed JOIN mc_work_employee we ON we.id = wed.employee_id WHERE we.corp_id = 1 AND we.wx_user_id = 'go-worker-user' AND wed.deleted_at IS NOT NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_user WHERE phone = '13900000000' AND tenant_id = 1 AND status = 2 AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_update_time WHERE corp_id = 1 AND type = 1 AND last_update_time IS NOT NULL;" "1"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 21, 1, 0) FROM mochat_go_background_task_executions WHERE tenant_id = 1 AND task_name = 'wework-callback' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"
wait_mysql_scalar "SELECT IF(counter.used_value = (SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE tenant_id = 1 AND kind = 'queue_item' AND status IN ('succeeded', 'failed', 'stopped')), 1, 0) FROM mochat_go_saas_usage_counters counter WHERE counter.tenant_id = 1 AND counter.metric = 'async_executions' AND counter.period_key = 'lifetime' AND counter.updated_by = 'runtime' AND counter.deleted_at IS NULL" "1"
wait_redis_scalar "0" LLEN mochat-go:wework-callback
wait_redis_scalar "0" LLEN mochat-go:wework-callback:processing
wait_redis_scalar "0" LLEN mochat-go:wework-callback:dead

grep -q "go worker enabled: WeWork callback Redis consumer" "$GO_LOG"

echo "wework callback worker smoke passed"
