#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-official-account-ticket-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13343}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26393}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18140}"
WECHAT_ADDR="${MOCHAT_WECHAT_ADDR:-127.0.0.1:19083}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-official-account-ticket.XXXXXX")"
PROBE_DIR="$PWD/.tmp-wechat-open-credential-probe-$$"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
MAINTENANCE_BIN="$WORK_DIR/mochat-saas-maintenance"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECHAT_LOG="$WORK_DIR/wechat.log"
GO_PID=""
WECHAT_PID=""

SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-official-account-ticket-secret}"
COMPONENT_TOKEN="component-token"
COMPONENT_AES_KEY="abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
WECHAT_OPEN_CREDENTIAL_KEY="8181818181818181818181818181818181818181818181818181818181818181"
WECHAT_OPEN_CREDENTIAL_KEY_ID="wechat-open-smoke-v1"
WECHAT_OPEN_CREDENTIAL_KEY_V2="8282828282828282828282828282828282828282828282828282828282828282"
WECHAT_OPEN_CREDENTIAL_KEY_ID_V2="wechat-open-smoke-v2"
TENANT_ID=701
PHONE=13800000701
PASSWORD=secret701
CORP_ID=701001
EMPLOYEE_ID=701001
DEPARTMENT_ID=701001

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" \
    MOCHAT_REDIS_PORT="$REDIS_PORT" \
    docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ -n "$WECHAT_PID" ] && kill -0 "$WECHAT_PID" 2>/dev/null; then
    kill "$WECHAT_PID" 2>/dev/null || true
    wait "$WECHAT_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR" "$PROBE_DIR"
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap 'echo "official account ticket smoke failed at line $LINENO" >&2; tail -200 "$GO_LOG" >&2 || true' ERR
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
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $url to return $expected" >&2
  [ -f "$GO_LOG" ] && tail -100 "$GO_LOG" >&2 || true
  [ -f "$WECHAT_LOG" ] && cat "$WECHAT_LOG" >&2 || true
  exit 1
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  local query="$1"
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat_official_account_ticket_check -e "$query" | tr -d '\r'
}

auth_token() {
  local out="$1"
  curl -sS -f \
    -H "Content-Type: application/json" \
    -d "{\"phone\":\"$PHONE\",\"password\":\"$PASSWORD\"}" \
    "http://$GO_ADDR/dashboard/user/auth" >"$out"
  python3 - "$out" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
print(payload["data"]["token"])
PY
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"
assert_port_free "${WECHAT_ADDR##*:}"

cat >"$WORK_DIR/fake_wechat.py" <<'PY'
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse, parse_qs

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
            self.send_json({"errcode": 0, "component_access_token": "component-token", "expires_in": 7200})
            return
        if parsed.path == "/cgi-bin/component/api_create_preauthcode":
            append({"path": parsed.path, "query": parse_qs(parsed.query), "payload": payload})
            self.send_json({"errcode": 0, "pre_auth_code": "pre-auth-code", "expires_in": 600})
            return
        if parsed.path == "/cgi-bin/component/api_query_auth":
            append({"path": parsed.path, "query": parse_qs(parsed.query), "payload": payload})
            self.send_json({
                "errcode": 0,
                "authorization_info": {
                    "authorizer_appid": "authorizer-app",
                    "authorizer_access_token": "authorizer-token-from-query",
                    "expires_in": 7200,
                    "authorizer_refresh_token": "authorizer-refresh-token",
                    "func_info": [{"funcscope_category": {"id": 1}}]
                }
            })
            return
        if parsed.path == "/cgi-bin/component/api_get_authorizer_info":
            append({"path": parsed.path, "query": parse_qs(parsed.query), "payload": payload})
            self.send_json({
                "errcode": 0,
                "authorizer_info": {
                    "nick_name": "授权公众号",
                    "head_img": "official/avatar-authorized.png",
                    "service_type_info": {"id": 2},
                    "verify_type_info": {"id": 0},
                    "user_name": "gh_authorizer_app",
                    "principal_name": "授权主体",
                    "alias": "mochat_go_auth",
                    "business_info": {
                        "open_pay": 1,
                        "open_shake": 0,
                        "open_scan": 1,
                        "open_card": 0,
                        "open_store": 0
                    },
                    "qrcode_url": "https://example.invalid/official-account-qrcode.jpg"
                }
            })
            return
        if parsed.path == "/cgi-bin/component/api_authorizer_token":
            append({"path": parsed.path, "query": parse_qs(parsed.query), "payload": payload})
            self.send_json({
                "errcode": 0,
                "authorizer_access_token": "authorizer-access-token",
                "expires_in": 7200,
                "authorizer_refresh_token": "authorizer-refresh-token-new"
            })
            return
        if parsed.path == "/cgi-bin/message/custom/send":
            append({"path": parsed.path, "query": parse_qs(parsed.query), "payload": payload})
            self.send_json({"errcode": 0, "errmsg": "ok"})
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

cat >"$WORK_DIR/encrypt_wechat_callback.go" <<'GO'
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
	"os"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) != 8 {
		_, _ = fmt.Fprintln(os.Stderr, "usage: encrypt_wechat_callback aes_key token timestamp nonce appid input_xml output_xml")
		os.Exit(2)
	}
	aesKey := strings.TrimSpace(os.Args[1])
	token := strings.TrimSpace(os.Args[2])
	timestamp := strings.TrimSpace(os.Args[3])
	nonce := strings.TrimSpace(os.Args[4])
	appID := strings.TrimSpace(os.Args[5])
	inputPath := os.Args[6]
	outputPath := os.Args[7]

	message, err := os.ReadFile(inputPath)
	if err != nil {
		panic(err)
	}
	key, err := base64.StdEncoding.DecodeString(aesKey + "=")
	if err != nil {
		panic(err)
	}
	if len(key) != 32 {
		panic("invalid aes key length")
	}
	payload := bytes.NewBuffer(nil)
	payload.Write([]byte("1234567890123456"))
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(message)))
	payload.Write(length[:])
	payload.Write(message)
	payload.WriteString(appID)
	plain := pkcs7Pad(payload.Bytes(), aes.BlockSize)
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(encrypted, plain)
	encryptedText := base64.StdEncoding.EncodeToString(encrypted)
	signature := signature(token, timestamp, nonce, encryptedText)
	wrapper := fmt.Sprintf("<xml><ToUserName><![CDATA[%s]]></ToUserName><Encrypt><![CDATA[%s]]></Encrypt></xml>", appID, encryptedText)
	if err := os.WriteFile(outputPath, []byte(wrapper), 0600); err != nil {
		panic(err)
	}
	fmt.Print(signature)
}

func pkcs7Pad(raw []byte, blockSize int) []byte {
	padding := blockSize - len(raw)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	return append(raw, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func signature(token, timestamp, nonce, encrypted string) string {
	parts := []string{token, timestamp, nonce, encrypted}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(sum[:])
}
GO

python3 "$WORK_DIR/fake_wechat.py" "$WECHAT_ADDR" "$WECHAT_LOG" &
WECHAT_PID="$!"
wait_url "http://$WECHAT_ADDR/health" 200

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

mysql_root <<'SQL'
DROP DATABASE IF EXISTS mochat_official_account_ticket_check;
CREATE DATABASE mochat_official_account_ticket_check CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON mochat_official_account_ticket_check.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$MAINTENANCE_BIN" ./cmd/mochat-saas-maintenance
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat_official_account_ticket_check?parseTime=true&loc=Local"

"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0007_wechat_component_tickets\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0068_wechat_open_credential_encryption\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "97"

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id "$TENANT_ID" \
  -tenant-name "公众号 ticket 租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "公众号 ticket 管理员" \
  -role-name "公众号 ticket 超管" \
  -official-accounts 3 >"$WORK_DIR/bootstrap.out"

USER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE tenant_id = $TENANT_ID AND phone = '$PHONE' AND status = 1 AND isSuperAdmin = 1 AND deleted_at IS NULL")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat_official_account_ticket_check <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '公众号 ticket 企业', 'ww-official-ticket', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', $TENANT_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_department
  (id, wx_department_id, corp_id, name, parent_id, wx_parentid, \`order\`, level, path, created_at, updated_at, deleted_at)
VALUES
  ($DEPARTMENT_ID, 1, $CORP_ID, '总部', 0, 0, 1, 1, '#$DEPARTMENT_ID#', NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, gender, status, log_user_id, main_department_id, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'official-ticket-user', $CORP_ID, '公众号 ticket 员工', '$PHONE', 1, 1, $USER_ID, $DEPARTMENT_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee_department
  (employee_id, department_id, is_leader_in_dept, \`order\`, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, $DEPARTMENT_ID, 0, 1, NOW(), NOW(), NULL);

INSERT INTO mc_official_account
  (appid, authorized_status, authorizer_appid, authorization_code, pre_auth_code, encoding_aes_key, token, secret,
   nickname, tenant_id, corp_id, create_user_id, created_at, updated_at, deleted_at)
VALUES
  ('component-app', 1, 'authorizer-unauth-smoke', 'legacy-authorization-code', 'legacy-pre-auth-code',
   'legacy-component-aes-key', 'legacy-component-token', 'legacy-component-secret',
   '待取消授权公众号', $TENANT_ID, $CORP_ID, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mochat_go_wechat_component_tickets
  (component_appid, component_verify_ticket, create_time, created_at, updated_at)
VALUES
  ('component-legacy', 'legacy-component-ticket', 1783180600, NOW(), NOW());

UPDATE mochat_go_saas_usage_counters
SET used_value = 1,
    updated_by = 'seed',
    updated_at = NOW()
WHERE tenant_id = $TENANT_ID AND metric = 'official_accounts' AND deleted_at IS NULL;
SQL

test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'official_accounts' AND deleted_at IS NULL")" = "1"

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_WECHAT_API_BASE_URL="http://$WECHAT_ADDR" \
  MOCHAT_WECHAT_OPEN_PLATFORM_APP_ID="component-app" \
  MOCHAT_WECHAT_OPEN_PLATFORM_SECRET="component-secret" \
  MOCHAT_WECHAT_OPEN_PLATFORM_TOKEN="$COMPONENT_TOKEN" \
  MOCHAT_WECHAT_OPEN_PLATFORM_AES_KEY="$COMPONENT_AES_KEY" \
  MOCHAT_WECHAT_COMPONENT_VERIFY_TICKET="" \
  MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
  MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID="$TENANT_ID" \
  MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY="$WECHAT_OPEN_CREDENTIAL_KEY" \
  MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY_ID="$WECHAT_OPEN_CREDENTIAL_KEY_ID" \
  MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_REQUIRE_ENCRYPTION=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
grep -q "go migrated route enabled: GET /dashboard/officialAccount/index" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/officialAccount/set" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/officialAccount/getPreAuthUrl" "$GO_LOG"
grep -q "go migrated route enabled: GET/POST /dashboard/officialAccount/authRedirect/" "$GO_LOG"
grep -q "WeChat Open credential protection: encryption_configured=true require_encryption=true" "$GO_LOG"

printf 'echo-ok' >"$WORK_DIR/echo-plain.txt"
echo_timestamp=1783180700
echo_nonce=echo-nonce
echo_signature="$(env -u GOROOT go run "$WORK_DIR/encrypt_wechat_callback.go" "$COMPONENT_AES_KEY" "$COMPONENT_TOKEN" "$echo_timestamp" "$echo_nonce" "component-app" "$WORK_DIR/echo-plain.txt" "$WORK_DIR/echo-wrapper.xml")"
echo_encrypted="$(python3 - "$WORK_DIR/echo-wrapper.xml" <<'PY'
import pathlib
import re
import sys

raw = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
match = re.search(r"<Encrypt><!\[CDATA\[(.*?)\]\]></Encrypt>", raw)
assert match, raw
print(match.group(1))
PY
)"

echo_status="$(curl -sS -G \
  --data-urlencode "echostr=$echo_encrypted" \
  -o "$WORK_DIR/echo.out" \
  -w '%{http_code}' \
  "http://$GO_ADDR/dashboard/officialAccount/authEventCallback?encrypt_type=aes&timestamp=$echo_timestamp&nonce=$echo_nonce&msg_signature=$echo_signature")"
test "$echo_status" = "200"
grep -q '^echo-ok$' "$WORK_DIR/echo.out"

missing_echo_signature_status="$(curl -sS -G \
  --data-urlencode "echostr=$echo_encrypted" \
  -o "$WORK_DIR/echo-missing-signature.out" \
  -w '%{http_code}' \
  "http://$GO_ADDR/dashboard/officialAccount/authEventCallback?encrypt_type=aes&timestamp=$echo_timestamp&nonce=$echo_nonce")"
test "$missing_echo_signature_status" = "400"
grep -q 'msg_signature required' "$WORK_DIR/echo-missing-signature.out"

TOKEN="$(auth_token "$WORK_DIR/auth.json")"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"corpId\":$CORP_ID}" \
  "http://$GO_ADDR/dashboard/corp/bind" >"$WORK_DIR/bind.json"

curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://$GO_ADDR/dashboard/officialAccount/index" >"$WORK_DIR/official-account-before.json"
python3 - "$WORK_DIR/official-account-before.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
accounts = payload["data"]["list"] if isinstance(payload["data"], dict) and "list" in payload["data"] else payload["data"]
assert any(item.get("nickname") == "待取消授权公众号" for item in accounts), payload
PY

curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/wechatOpenCredentialProtection" >"$WORK_DIR/credential-protection-before.json"
jq -e '.data.credentialProtection.supported == true and .data.credentialProtection.encryptionConfigured == true and .data.credentialProtection.configuredCredentialCount == 2 and .data.credentialProtection.componentTicketCount == 1 and .data.credentialProtection.officialAccountCount == 1 and .data.credentialProtection.legacyPlaintextCount == 2 and .data.credentialProtection.rotationRequiredCount == 2' "$WORK_DIR/credential-protection-before.json" >/dev/null

curl -sS -f -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"tenantId":0,"limit":100,"remark":"微信开放平台旧明文轮换 smoke"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/wechatOpenCredentialRotation" >"$WORK_DIR/credential-rotation.json"
jq -e '.data.rotation.rotatedCount == 2 and .data.rotation.componentTicketRotatedCount == 1 and .data.rotation.officialAccountRotatedCount == 1 and .data.rotation.legacyCount == 2 and .data.credentialProtection.legacyPlaintextCount == 0 and .data.credentialProtection.rotationRequiredCount == 0 and .data.credentialProtection.healthy == true' "$WORK_DIR/credential-rotation.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_wechat_component_tickets WHERE component_appid = 'component-legacy' AND component_verify_ticket = '' AND component_verify_ticket_ciphertext <> '' AND credential_key_id = '$WECHAT_OPEN_CREDENTIAL_KEY_ID'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_official_account WHERE authorizer_appid = 'authorizer-unauth-smoke' AND authorization_code = '' AND pre_auth_code = '' AND encoding_aes_key = '' AND token = '' AND secret = '' AND wechat_credentials_ciphertext <> '' AND wechat_credentials_key_id = '$WECHAT_OPEN_CREDENTIAL_KEY_ID'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.wechat_open_credential.rotate' AND target_type = 'wechat_open_credential'")" = "1"

cat >"$WORK_DIR/unauthorized-plain.xml" <<XML
<xml>
  <AppId><![CDATA[component-app]]></AppId>
  <CreateTime>1783180750</CreateTime>
  <InfoType><![CDATA[unauthorized]]></InfoType>
  <AuthorizerAppid><![CDATA[authorizer-unauth-smoke]]></AuthorizerAppid>
</xml>
XML

unauth_timestamp=1783180750
unauth_nonce=unauth-nonce
unauth_signature="$(env -u GOROOT go run "$WORK_DIR/encrypt_wechat_callback.go" "$COMPONENT_AES_KEY" "$COMPONENT_TOKEN" "$unauth_timestamp" "$unauth_nonce" "component-app" "$WORK_DIR/unauthorized-plain.xml" "$WORK_DIR/unauthorized.xml")"

curl -sS -f \
  -H "Content-Type: application/xml" \
  --data-binary @"$WORK_DIR/unauthorized.xml" \
  "http://$GO_ADDR/dashboard/officialAccount/authEventCallback?encrypt_type=aes&timestamp=$unauth_timestamp&nonce=$unauth_nonce&msg_signature=$unauth_signature" >"$WORK_DIR/unauthorized.out"
grep -q '^success$' "$WORK_DIR/unauthorized.out"
test "$(mysql_scalar "SELECT authorized_status FROM mc_official_account WHERE authorizer_appid = 'authorizer-unauth-smoke' AND deleted_at IS NULL")" = "3"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'official_accounts' AND deleted_at IS NULL")" = "0"

curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://$GO_ADDR/dashboard/officialAccount/index" >"$WORK_DIR/official-account-after.json"
python3 - "$WORK_DIR/official-account-after.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
accounts = payload["data"]["list"] if isinstance(payload["data"], dict) and "list" in payload["data"] else payload["data"]
assert not any(item.get("nickname") == "待取消授权公众号" for item in accounts), payload
PY

cat >"$WORK_DIR/component-ticket-plain.xml" <<XML
<xml>
  <AppId><![CDATA[component-app]]></AppId>
  <CreateTime>1783180800</CreateTime>
  <InfoType><![CDATA[component_verify_ticket]]></InfoType>
  <ComponentVerifyTicket><![CDATA[ticket-from-callback]]></ComponentVerifyTicket>
</xml>
XML

ticket_timestamp=1783180800
ticket_nonce=ticket-nonce
ticket_signature="$(env -u GOROOT go run "$WORK_DIR/encrypt_wechat_callback.go" "$COMPONENT_AES_KEY" "$COMPONENT_TOKEN" "$ticket_timestamp" "$ticket_nonce" "component-app" "$WORK_DIR/component-ticket-plain.xml" "$WORK_DIR/component-ticket.xml")"

curl -sS -f \
  -H "Content-Type: application/xml" \
  --data-binary @"$WORK_DIR/component-ticket.xml" \
  "http://$GO_ADDR/dashboard/officialAccount/authEventCallback?encrypt_type=aes&timestamp=$ticket_timestamp&nonce=$ticket_nonce&msg_signature=$ticket_signature" >"$WORK_DIR/callback.out"
grep -q '^success$' "$WORK_DIR/callback.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_wechat_component_tickets WHERE component_appid = 'component-app' AND component_verify_ticket = '' AND component_verify_ticket_ciphertext <> '' AND credential_key_id = '$WECHAT_OPEN_CREDENTIAL_KEY_ID'")" = "1"

missing_ticket_signature_status="$(curl -sS \
  -X POST \
  -H "Content-Type: application/xml" \
  --data-binary @"$WORK_DIR/component-ticket.xml" \
  -o "$WORK_DIR/callback-missing-signature.out" \
  -w '%{http_code}' \
  "http://$GO_ADDR/dashboard/officialAccount/authEventCallback?encrypt_type=aes&timestamp=$ticket_timestamp&nonce=$ticket_nonce")"
test "$missing_ticket_signature_status" = "400"
grep -q 'msg_signature required' "$WORK_DIR/callback-missing-signature.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_wechat_component_tickets WHERE component_appid = 'component-app' AND component_verify_ticket = '' AND component_verify_ticket_ciphertext <> '' AND credential_key_id = '$WECHAT_OPEN_CREDENTIAL_KEY_ID'")" = "1"

bad_ticket_signature_status="$(curl -sS \
  -X POST \
  -H "Content-Type: application/xml" \
  --data-binary @"$WORK_DIR/component-ticket.xml" \
  -o "$WORK_DIR/callback-bad-signature.out" \
  -w '%{http_code}' \
  "http://$GO_ADDR/dashboard/officialAccount/authEventCallback?encrypt_type=aes&timestamp=$ticket_timestamp&nonce=$ticket_nonce&msg_signature=bad")"
test "$bad_ticket_signature_status" = "400"
grep -q 'invalid msg_signature' "$WORK_DIR/callback-bad-signature.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_wechat_component_tickets WHERE component_appid = 'component-app' AND component_verify_ticket = '' AND component_verify_ticket_ciphertext <> '' AND credential_key_id = '$WECHAT_OPEN_CREDENTIAL_KEY_ID'")" = "1"

curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://$GO_ADDR/dashboard/officialAccount/getPreAuthUrl" >"$WORK_DIR/preauth.json"

auth_redirect_status="$(curl -sS \
  -o "$WORK_DIR/auth-redirect.out" \
  -D "$WORK_DIR/auth-redirect.headers" \
  -w '%{http_code}' \
  "http://$GO_ADDR/dashboard/officialAccount/authRedirect/?auth_code=dashboard-auth-code&corp_id=$CORP_ID")"
test "$auth_redirect_status" = "302"
grep -qi '^Location: .*/officialAccount/index' "$WORK_DIR/auth-redirect.headers"

AUTHORIZED_ACCOUNT_ID="$(mysql_scalar "SELECT id FROM mc_official_account WHERE authorizer_appid = 'authorizer-app' AND corp_id = $CORP_ID AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$AUTHORIZED_ACCOUNT_ID"
test "$(mysql_scalar "SELECT CONCAT(COALESCE(tenant_id, 0), ':', COALESCE(authorized_status, 0), ':', COALESCE(nickname, ''), ':', COALESCE(avatar, ''), ':', COALESCE(service_type_info, 0), ':', COALESCE(verify_type_info, 0), ':', COALESCE(user_name, ''), ':', COALESCE(principal_name, '')) FROM mc_official_account WHERE id = $AUTHORIZED_ACCOUNT_ID")" = "$TENANT_ID:1:授权公众号:official/avatar-authorized.png:2:0:gh_authorizer_app:授权主体"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_official_account WHERE id = $AUTHORIZED_ACCOUNT_ID AND authorization_code = '' AND pre_auth_code = '' AND encoding_aes_key = '' AND token = '' AND secret = '' AND wechat_credentials_ciphertext <> '' AND wechat_credentials_key_id = '$WECHAT_OPEN_CREDENTIAL_KEY_ID'")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_ID AND metric = 'official_accounts' AND deleted_at IS NULL")" = "1"

mkdir -p "$PROBE_DIR"
cat >"$PROBE_DIR/main.go" <<'GO'
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"jiyi/mochat-go/internal/mysqlconn"
	"jiyi/mochat-go/internal/store"
	"jiyi/mochat-go/internal/wechatopencredentials"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: probe official-account-id")
	}
	id, err := strconv.Atoi(os.Args[1])
	if err != nil {
		panic(err)
	}
	manager, err := wechatopencredentials.NewManager(wechatopencredentials.Config{
		EncryptionKey: os.Getenv("MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY"),
		EncryptionKeyID: os.Getenv("MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY_ID"),
		RequireEncryption: true,
	})
	if err != nil {
		panic(err)
	}
	db, err := mysqlconn.Open(os.Getenv("MOCHAT_MYSQL_DSN"))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	info, found, err := store.NewMySQLStore(db).WithWeChatOpenCredentialCipher(manager).OfficialAccountOAuthInfoByID(context.Background(), id)
	if err != nil {
		panic(err)
	}
	if !found || info.ComponentAppID != "component-app" || info.AuthorizerAppID != "authorizer-app" ||
		info.ComponentSecret != "component-secret" || info.ComponentToken != "component-token" ||
		info.ComponentAESKey != "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG" ||
		info.AuthorizationCode != "dashboard-auth-code" || info.AuthorizerRefreshToken != "authorizer-refresh-token" {
		panic("decrypted official account credential mismatch")
	}
	fmt.Println("wechat_open_credential_probe\tpassed")
}
GO
MOCHAT_MYSQL_DSN="$DSN" \
MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY="$WECHAT_OPEN_CREDENTIAL_KEY" \
MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY_ID="$WECHAT_OPEN_CREDENTIAL_KEY_ID" \
env -u GOROOT go run "$PROBE_DIR/main.go" "$AUTHORIZED_ACCOUNT_ID" >"$WORK_DIR/credential-probe.out"
grep -q $'wechat_open_credential_probe\tpassed' "$WORK_DIR/credential-probe.out"

curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://$GO_ADDR/dashboard/officialAccount/index" >"$WORK_DIR/official-account-authorized.json"
python3 - "$WORK_DIR/official-account-authorized.json" "$AUTHORIZED_ACCOUNT_ID" "$GO_ADDR" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
account_id = int(sys.argv[2])
go_addr = sys.argv[3]
assert payload["code"] == 200, payload
accounts = payload["data"]["list"] if isinstance(payload["data"], dict) and "list" in payload["data"] else payload["data"]
matched = [item for item in accounts if item.get("id") == account_id]
assert matched, payload
account = matched[0]
assert account["nickname"] == "授权公众号", account
assert account["avatar"] == f"http://{go_addr}/static/official/avatar-authorized.png", account
PY

curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://$GO_ADDR/dashboard/officialAccount/index?type=2" >"$WORK_DIR/official-account-type.json"
python3 - "$WORK_DIR/official-account-type.json" "$AUTHORIZED_ACCOUNT_ID" "$GO_ADDR" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
account_id = int(sys.argv[2])
go_addr = sys.argv[3]
assert payload["code"] == 200, payload
account = payload["data"]
assert account["id"] == account_id, payload
assert account["nickname"] == "授权公众号", account
assert account["avatar"] == f"http://{go_addr}/static/official/avatar-authorized.png", account
PY
test "$(mysql_scalar "SELECT CONCAT(COALESCE(official_account_id, 0), ':', COALESCE(type, 0), ':', COALESCE(corp_id, 0), ':', COALESCE(create_user_id, 0)) FROM mc_official_account_set WHERE corp_id = $CORP_ID AND type = 2 AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")" = "$AUTHORIZED_ACCOUNT_ID:2:$CORP_ID:$USER_ID"

curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://$GO_ADDR/dashboard/officialAccount/set?type=3&official_account_id=$AUTHORIZED_ACCOUNT_ID" >"$WORK_DIR/official-account-set.json"
python3 - "$WORK_DIR/official-account-set.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
assert payload["data"] == [], payload
PY
test "$(mysql_scalar "SELECT CONCAT(COALESCE(official_account_id, 0), ':', COALESCE(type, 0), ':', COALESCE(corp_id, 0), ':', COALESCE(create_user_id, 0)) FROM mc_official_account_set WHERE corp_id = $CORP_ID AND type = 3 AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")" = "$AUTHORIZED_ACCOUNT_ID:3:$CORP_ID:$USER_ID"

cat >"$WORK_DIR/query-auth-code-plain.xml" <<XML
<xml>
  <ToUserName><![CDATA[gh_3c884a361561]]></ToUserName>
  <FromUserName><![CDATA[from-openid-query]]></FromUserName>
  <CreateTime>1783180900</CreateTime>
  <MsgType><![CDATA[text]]></MsgType>
  <Content><![CDATA[QUERY_AUTH_CODE:query-code-from-smoke]]></Content>
  <MsgId>7</MsgId>
</xml>
XML

query_timestamp=1783180900
query_nonce=query-nonce
query_signature="$(env -u GOROOT go run "$WORK_DIR/encrypt_wechat_callback.go" "$COMPONENT_AES_KEY" "$COMPONENT_TOKEN" "$query_timestamp" "$query_nonce" "component-app" "$WORK_DIR/query-auth-code-plain.xml" "$WORK_DIR/query-auth-code.xml")"

bad_query_signature_status="$(curl -sS \
  -X POST \
  -H "Content-Type: text/xml" \
  --data-binary @"$WORK_DIR/query-auth-code.xml" \
  -o "$WORK_DIR/query-auth-code-bad-signature.out" \
  -w '%{http_code}' \
  "http://$GO_ADDR/dashboard/component-app/officialAccount/messageEventCallback?encrypt_type=aes&timestamp=$query_timestamp&nonce=$query_nonce&msg_signature=bad")"
test "$bad_query_signature_status" = "400"
grep -q 'invalid msg_signature' "$WORK_DIR/query-auth-code-bad-signature.out"

query_auth_status="$(curl -sS \
  -X POST \
  -H "Content-Type: text/xml" \
  --data-binary @"$WORK_DIR/query-auth-code.xml" \
  -o "$WORK_DIR/query-auth-code.out" \
  -w '%{http_code}' \
  "http://$GO_ADDR/dashboard/component-app/officialAccount/messageEventCallback?encrypt_type=aes&timestamp=$query_timestamp&nonce=$query_nonce&msg_signature=$query_signature")"

python3 - "$WORK_DIR/preauth.json" "$WECHAT_LOG" "$query_auth_status" "$WORK_DIR/query-auth-code.out" <<'PY'
import json
import pathlib
import sys
from urllib.parse import urlparse, parse_qs

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
query_auth_status = sys.argv[3]
query_auth_body = pathlib.Path(sys.argv[4]).read_text(encoding="utf-8")
assert payload["code"] == 200, payload
assert query_auth_status == "200", query_auth_status
assert query_auth_body == "", query_auth_body
auth_url = payload["data"]["url"]
query = parse_qs(urlparse(auth_url).query)
assert query["component_appid"] == ["component-app"], auth_url
assert query["pre_auth_code"] == ["pre-auth-code"], auth_url

events = [json.loads(line) for line in pathlib.Path(sys.argv[2]).read_text(encoding="utf-8").splitlines() if line.strip()]
token_events = [event for event in events if event["path"] == "/cgi-bin/component/api_component_token"]
preauth_events = [event for event in events if event["path"] == "/cgi-bin/component/api_create_preauthcode"]
query_auth_events = [event for event in events if event["path"] == "/cgi-bin/component/api_query_auth"]
authorizer_info_events = [event for event in events if event["path"] == "/cgi-bin/component/api_get_authorizer_info"]
authorizer_token_events = [event for event in events if event["path"] == "/cgi-bin/component/api_authorizer_token"]
custom_send_events = [event for event in events if event["path"] == "/cgi-bin/message/custom/send"]
assert token_events, events
assert preauth_events, events
assert query_auth_events, events
assert authorizer_info_events, events
assert authorizer_token_events, events
assert custom_send_events, events
assert token_events[-1]["payload"]["component_verify_ticket"] == "ticket-from-callback", token_events[-1]
assert preauth_events[-1]["query"]["component_access_token"] == ["component-token"], preauth_events[-1]
assert any(event["payload"]["authorization_code"] == "dashboard-auth-code" for event in query_auth_events), query_auth_events
assert authorizer_info_events[-1]["query"]["component_access_token"] == ["component-token"], authorizer_info_events[-1]
assert authorizer_info_events[-1]["payload"]["component_appid"] == "component-app", authorizer_info_events[-1]
assert authorizer_info_events[-1]["payload"]["authorizer_appid"] == "authorizer-app", authorizer_info_events[-1]
assert query_auth_events[-1]["query"]["component_access_token"] == ["component-token"], query_auth_events[-1]
assert query_auth_events[-1]["payload"]["authorization_code"] == "query-code-from-smoke", query_auth_events[-1]
assert query_auth_events[-1]["payload"]["component_appid"] == "component-app", query_auth_events[-1]
assert authorizer_token_events[-1]["query"]["component_access_token"] == ["component-token"], authorizer_token_events[-1]
assert authorizer_token_events[-1]["payload"]["component_appid"] == "component-app", authorizer_token_events[-1]
assert authorizer_token_events[-1]["payload"]["authorizer_appid"] == "authorizer-app", authorizer_token_events[-1]
assert authorizer_token_events[-1]["payload"]["authorizer_refresh_token"] == "authorizer-refresh-token", authorizer_token_events[-1]
assert custom_send_events[-1]["query"]["access_token"] == ["authorizer-access-token"], custom_send_events[-1]
assert custom_send_events[-1]["payload"]["touser"] == "from-openid-query", custom_send_events[-1]
assert custom_send_events[-1]["payload"]["msgtype"] == "text", custom_send_events[-1]
assert custom_send_events[-1]["payload"]["text"]["content"] == "query-code-from-smoke_from_api", custom_send_events[-1]
print("official account ticket smoke passed")
PY

curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/wechatOpenCredentialProtection" >"$WORK_DIR/credential-protection-after.json"
jq -e '.data.credentialProtection.supported == true and .data.credentialProtection.configuredCredentialCount == 4 and .data.credentialProtection.componentTicketCount == 2 and .data.credentialProtection.officialAccountCount == 2 and .data.credentialProtection.encryptedCredentialCount == 4 and .data.credentialProtection.activeKeyCredentialCount == 4 and .data.credentialProtection.legacyPlaintextCount == 0 and .data.credentialProtection.rotationRequiredCount == 0 and .data.credentialProtection.unavailableKeyCount == 0 and .data.credentialProtection.decryptFailureCount == 0 and .data.credentialProtection.healthy == true' "$WORK_DIR/credential-protection-after.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_wechat_component_tickets WHERE component_verify_ticket <> ''")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_official_account WHERE authorization_code <> '' OR pre_auth_code <> '' OR encoding_aes_key <> '' OR token <> '' OR secret <> ''")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_wechat_component_tickets WHERE component_verify_ticket_ciphertext LIKE '%ticket-from-callback%' OR component_verify_ticket_ciphertext LIKE '%legacy-component-ticket%'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_official_account WHERE wechat_credentials_ciphertext LIKE '%component-secret%' OR wechat_credentials_ciphertext LIKE '%authorizer-refresh-token%'")" = "0"

if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
  kill "$GO_PID" 2>/dev/null || true
  wait "$GO_PID" 2>/dev/null || true
fi
GO_PID=""

if MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY="$WECHAT_OPEN_CREDENTIAL_KEY_V2" \
  MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY_ID="$WECHAT_OPEN_CREDENTIAL_KEY_ID_V2" \
  "$MAINTENANCE_BIN" \
    -dsn "$DSN" \
    -action rotate-wechat-open-credentials \
    -tenant-id 0 \
    -wechat-open-credential-rotation-limit 100 >"$WORK_DIR/missing-history.out" 2>&1; then
  echo "rotation without historical key unexpectedly succeeded" >&2
  exit 1
fi
grep -q "credential encryption key \"$WECHAT_OPEN_CREDENTIAL_KEY_ID\" is unavailable" "$WORK_DIR/missing-history.out"

WECHAT_OPEN_CREDENTIAL_KEYS="{\"$WECHAT_OPEN_CREDENTIAL_KEY_ID\":\"$WECHAT_OPEN_CREDENTIAL_KEY\",\"$WECHAT_OPEN_CREDENTIAL_KEY_ID_V2\":\"$WECHAT_OPEN_CREDENTIAL_KEY_V2\"}"
MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEYS="$WECHAT_OPEN_CREDENTIAL_KEYS" \
MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY_ID="$WECHAT_OPEN_CREDENTIAL_KEY_ID_V2" \
MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_REQUIRE_ENCRYPTION=1 \
"$MAINTENANCE_BIN" \
  -dsn "$DSN" \
  -action rotate-wechat-open-credentials \
  -tenant-id 0 \
  -wechat-open-credential-rotation-limit 100 >"$WORK_DIR/keyring-rotation.out"
grep -q $'rotated\t4' "$WORK_DIR/keyring-rotation.out"
grep -q $'component_ticket_rotated\t2' "$WORK_DIR/keyring-rotation.out"
grep -q $'official_account_rotated\t2' "$WORK_DIR/keyring-rotation.out"
grep -q $'legacy_plaintext\t0' "$WORK_DIR/keyring-rotation.out"
grep -q $'reencrypted\t4' "$WORK_DIR/keyring-rotation.out"
grep -q $'active_key_id\twechat-open-smoke-v2' "$WORK_DIR/keyring-rotation.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_wechat_component_tickets WHERE credential_key_id = '$WECHAT_OPEN_CREDENTIAL_KEY_ID_V2' AND component_verify_ticket = '' AND component_verify_ticket_ciphertext <> ''")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_official_account WHERE wechat_credentials_key_id = '$WECHAT_OPEN_CREDENTIAL_KEY_ID_V2' AND authorization_code = '' AND pre_auth_code = '' AND encoding_aes_key = '' AND token = '' AND secret = '' AND wechat_credentials_ciphertext <> ''")" = "2"

MOCHAT_MYSQL_DSN="$DSN" \
MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY="$WECHAT_OPEN_CREDENTIAL_KEY_V2" \
MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY_ID="$WECHAT_OPEN_CREDENTIAL_KEY_ID_V2" \
env -u GOROOT go run "$PROBE_DIR/main.go" "$AUTHORIZED_ACCOUNT_ID" >"$WORK_DIR/credential-probe-v2.out"
grep -q $'wechat_open_credential_probe\tpassed' "$WORK_DIR/credential-probe-v2.out"
echo "wechat open credential encryption smoke passed"
