#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/local/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18080}"
PHP_ADDR="${MOCHAT_FAKE_PHP_ADDR:-127.0.0.1:19051}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-stack.XXXXXX")"
GO_LOG="$WORK_DIR/go.log"
PHP_LOG="$WORK_DIR/fake-php.log"
GO_BIN="$WORK_DIR/mochat-go"
GO_PID=""
PHP_PID=""

compose() {
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ -n "$PHP_PID" ] && kill -0 "$PHP_PID" 2>/dev/null; then
    kill "$PHP_PID" 2>/dev/null || true
    wait "$PHP_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR"
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

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
    code="$(curl -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $url to return $expected" >&2
  [ -f "$GO_LOG" ] && tail -80 "$GO_LOG" >&2 || true
  [ -f "$PHP_LOG" ] && tail -80 "$PHP_LOG" >&2 || true
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
assert_port_free "$PHP_ADDR"

python3 - "$PHP_ADDR" >"$PHP_LOG" 2>&1 <<'PY' &
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

host, port = sys.argv[1].rsplit(":", 1)

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        payload = b"Hello MoChat " if self.path == "/" else b'{"code":0,"msg":"fake php fallback","data":{}}'
        self.send_response(200)
        self.send_header("Content-Type", "text/plain; charset=utf-8")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, format, *args):
        return

ThreadingHTTPServer((host, int(port)), Handler).serve_forever()
PY
PHP_PID="$!"

wait_url "http://$PHP_ADDR/" 200

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

compose exec -T mysql mariadb -umochat -pmochat_pass mochat -e "SHOW TABLES LIKE 'mc_user';" | grep -q 'mc_user'
compose exec -T mysql mariadb -umochat -pmochat_pass mochat -N -e "SELECT COUNT(*) FROM mc_rbac_menu;" | grep -q '458'
compose exec -T redis redis-cli ping | grep -q 'PONG'
go build -o "$GO_BIN" ./cmd/mochat-go

env \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_SOURCE_ROOT=../mochat \
  MOCHAT_COMPAT_MANIFEST=../docs/migration/compat_manifest.json \
  MOCHAT_PHP_UPSTREAM="http://$PHP_ADDR" \
  MOCHAT_FILE_STORAGE_ROOT="$WORK_DIR/upload" \
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

wait_url "http://$GO_ADDR/healthz" 200
wait_url "http://$GO_ADDR/readyz" 200

curl -sS "http://$GO_ADDR/compat/status" >"$WORK_DIR/status.json"
python3 - "$WORK_DIR/status.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    status = json.load(f)

expected = {
    "POST /dashboard/user/auth",
    "GET /dashboard/user/loginShow",
    "PUT /dashboard/user/logout",
    "GET /dashboard/role/permissionByUser",
    "GET /dashboard/corp/select",
    "POST /dashboard/corp/bind",
    "GET /dashboard/corp/index",
    "GET /dashboard/corp/show",
    "POST /dashboard/corp/store",
    "PUT /dashboard/corp/update",
    "GET /dashboard/corp/weWorkCallback",
    "POST /dashboard/corp/weWorkCallback",
    "GET /weWork/callback",
    "POST /weWork/callback",
    "GET /dashboard/chatTool/config",
    "GET /{wxVerifyTxt:WW_verify_[0-9a-zA-Z]{16}\\.txt$}",
    "POST /dashboard/agent/txtVerifyUpload",
    "POST /dashboard/agent/store",
    "GET /dashboard/role/select",
    "GET /dashboard/role/index",
    "GET /dashboard/role/show",
    "GET /dashboard/role/permissionShow",
    "GET /dashboard/role/showEmployee",
    "GET /dashboard/menu/iconIndex",
    "GET /dashboard/menu/select",
    "GET /dashboard/menu/index",
    "GET /dashboard/menu/show",
    "GET /dashboard/corpData/index",
    "GET /dashboard/corpData/lineChat",
    "GET /dashboard/workEmployee/index",
    "GET /dashboard/workEmployee/searchCondition",
    "PUT /dashboard/workEmployee/synEmployee",
    "GET /dashboard/workDepartment/index",
    "GET /dashboard/workEmployeeDepartment/memberIndex",
    "GET /dashboard/workDepartment/selectByPhone",
    "GET /dashboard/workDepartment/pageIndex",
    "GET /dashboard/workDepartment/showEmployee",
    "GET /dashboard/workContactTagGroup/index",
    "GET /dashboard/workContactTagGroup/detail",
    "GET /sidebar/workContactTagGroup/index",
    "GET /dashboard/workContactTag/index",
    "GET /dashboard/workContactTag/detail",
    "GET /dashboard/workContactTag/contactTagList",
    "GET /dashboard/workContactTag/allTag",
    "PUT /dashboard/workContactTag/synContactTag",
    "PUT /dashboard/workContact/synContact",
    "GET /dashboard/workContact/index",
    "GET /dashboard/workContact/lossContact",
    "GET /dashboard/workContact/source",
    "GET /dashboard/workContact/show",
    "GET /dashboard/workContact/track",
    "PUT /dashboard/workContact/update",
    "POST /dashboard/workContact/batchLabeling",
    "GET /dashboard/workContactRoom/index",
    "GET /dashboard/workRoom/index",
    "GET /dashboard/workRoom/roomIndex",
    "GET /dashboard/workRoom/statistics",
    "GET /dashboard/workRoom/statisticsIndex",
    "PUT /dashboard/workRoom/syn",
    "PUT /dashboard/workRoom/batchUpdate",
    "GET /sidebar/workRoom/roomManage",
    "GET /dashboard/contactTransfer/info",
    "GET /dashboard/contactTransfer/unassignedList",
    "GET /dashboard/contactTransfer/room",
    "GET /dashboard/contactTransfer/log",
    "GET /dashboard/contactTransfer/saveUnassignedList",
    "POST /dashboard/contactTransfer/index",
    "POST /dashboard/contactTransfer/room",
    "GET /dashboard/workRoomAutoPull/index",
    "GET /dashboard/workRoomAutoPull/show",
    "POST /dashboard/workRoomAutoPull/store",
    "PUT /dashboard/workRoomAutoPull/update",
    "GET /dashboard/roomTagPull/index",
    "GET /dashboard/roomTagPull/show",
    "GET /dashboard/roomTagPull/showContact",
    "GET /dashboard/roomTagPull/roomList",
    "GET /dashboard/roomTagPull/chooseContact",
    "POST /dashboard/roomTagPull/store",
    "POST /dashboard/roomTagPull/filterContact",
    "GET /dashboard/roomTagPull/remindSend",
    "DELETE /dashboard/roomTagPull/destroy",
    "GET /dashboard/contactMessageBatchSend/index",
    "GET /dashboard/contactMessageBatchSend/show",
    "GET /dashboard/contactMessageBatchSend/showRoom",
    "GET /dashboard/contactMessageBatchSend/employeeSendIndex",
    "GET /dashboard/contactMessageBatchSend/contactReceiveIndex",
    "POST /dashboard/contactMessageBatchSend/store",
    "POST /dashboard/contactMessageBatchSend/remind",
    "DELETE /dashboard/contactMessageBatchSend/destroy",
    "GET /dashboard/roomMessageBatchSend/index",
    "GET /dashboard/roomMessageBatchSend/show",
    "GET /dashboard/roomMessageBatchSend/roomOwnerSendIndex",
    "GET /dashboard/roomMessageBatchSend/roomReceiveIndex",
    "POST /dashboard/roomMessageBatchSend/store",
    "GET /dashboard/roomMessageBatchSend/remind",
    "DELETE /dashboard/roomMessageBatchSend/destroy",
    "GET /dashboard/officialAccount/index",
    "GET /dashboard/officialAccount/set",
    "GET /dashboard/officialAccount/getPreAuthUrl",
    "GET /dashboard/officialAccount/authRedirect/",
    "POST /dashboard/officialAccount/authRedirect/",
    "GET /dashboard/officialAccount/authEventCallback",
    "POST /dashboard/officialAccount/authEventCallback",
    "GET /dashboard/{appId}/officialAccount/messageEventCallback",
    "POST /dashboard/{appId}/officialAccount/messageEventCallback",
    "GET /load/{params?}",
    "POST /load/{params?}",
    "GET /dashboard/workFission/index",
    "GET /dashboard/workFission/show",
    "GET /dashboard/workFission/info",
    "GET /dashboard/workFission/statistics",
    "GET /dashboard/workFission/chooseContact",
    "POST /dashboard/workFission/store",
    "PUT /dashboard/workFission/update",
    "POST /dashboard/workFission/invite",
    "GET /dashboard/workFission/inviteData",
    "GET /dashboard/workFission/inviteDetail",
    "DELETE /dashboard/workFission/destroy",
    "GET /operation/workFission/inviteFriends",
    "GET /operation/workFission/poster",
    "GET /operation/workFission/taskData",
    "PUT /operation/workFission/receive",
    "GET /operation/auth/workFission",
    "POST /operation/auth/workFission",
    "GET /operation/openUserInfo/workFission",
    "GET /sidebar/workContactTag/allTag",
    "GET /sidebar/workContact/detail",
    "GET /sidebar/workContact/show",
    "GET /sidebar/workContact/track",
    "PUT /sidebar/workContact/update",
    "GET /sidebar/contactProcessStatus/index",
    "PUT /sidebar/contactProcessStatus/update",
    "GET /sidebar/medium/mediaIdUpdate",
    "GET /dashboard/contactField/index",
    "GET /dashboard/contactField/show",
    "GET /dashboard/contactField/portrait",
    "GET /dashboard/contactFieldPivot/index",
    "PUT /dashboard/contactFieldPivot/update",
    "GET /sidebar/contactFieldPivot/index",
    "PUT /sidebar/contactFieldPivot/update",
    "GET /dashboard/channelCode/index",
    "GET /dashboard/channelCode/show",
    "GET /dashboard/channelCode/contact",
    "GET /dashboard/channelCode/statistics",
    "GET /dashboard/channelCode/statisticsIndex",
    "POST /dashboard/channelCode/store",
    "PUT /dashboard/channelCode/update",
    "GET /dashboard/channelCodeGroup/index",
    "GET /dashboard/channelCodeGroup/detail",
    "POST /dashboard/channelCodeGroup/store",
    "PUT /dashboard/channelCodeGroup/update",
    "PUT /dashboard/channelCodeGroup/move",
    "GET /sidebar/agent/auth",
    "POST /sidebar/agent/auth",
    "GET /sidebar/agent/oauth",
    "GET /sidebar/agent/jssdkConfig",
    "GET /sidebar/wxJsSdk/config",
}
routes = set(status.get("migrated_routes") or [])
missing = sorted(expected - routes)
if missing:
    raise SystemExit("missing migrated routes: " + ", ".join(missing))
if status.get("migrated_route_count") != 165:
    raise SystemExit(f"unexpected migrated_route_count: {status.get('migrated_route_count')}")
if not status.get("source_root_exists") or not status.get("manifest_exists"):
    raise SystemExit("source root or manifest is missing")
PY

txt_verify_body="$(curl -sS "http://$GO_ADDR/WW_verify_ABCDEF1234567890.txt")"
if [ "$txt_verify_body" != "ABCDEF1234567890" ]; then
  echo "unexpected txt verify body: $txt_verify_body" >&2
  exit 1
fi

printf '%s' 'ABCDEF1234567890' >"$WORK_DIR/verify.txt"
curl -sS -f -F 'file=@'"$WORK_DIR/verify.txt"';type=text/plain;filename=WW_verify_ABCDEF1234567890.txt' \
  "http://$GO_ADDR/dashboard/agent/txtVerifyUpload" >"$WORK_DIR/txt_upload.json"
python3 - "$WORK_DIR/txt_upload.json" "$WORK_DIR/upload/wx_txt_verify/WW_verify_ABCDEF1234567890.txt" <<'PY'
import json
import pathlib
import sys

body = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
if body.get("code") != 200:
    raise SystemExit(body)
content = pathlib.Path(sys.argv[2]).read_text(encoding="utf-8")
if content != "ABCDEF1234567890":
    raise SystemExit(f"unexpected uploaded content: {content!r}")
PY

echo "local stack check passed"
