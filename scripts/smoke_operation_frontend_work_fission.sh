#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-operation-front-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18151}"
GO_HOST="${GO_ADDR%:*}"
GO_PORT="${GO_ADDR##*:}"
SIDEBAR_ADDR="${MOCHAT_SIDEBAR_FRONTEND_ADDR:-$GO_HOST:$((GO_PORT + 1))}"
OPERATION_ADDR="${MOCHAT_OPERATION_FRONTEND_ADDR:-$GO_HOST:$((GO_PORT + 2))}"
WECHAT_ADDR="${MOCHAT_WECHAT_ADDR:-$GO_HOST:$((GO_PORT + 3))}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13351}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26401}"
PLAYWRIGHT_NODE_PATH="${PLAYWRIGHT_NODE_PATH:-$(npm root -g 2>/dev/null || true)}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-operation-front.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECHAT_LOG="$WORK_DIR/wechat.ndjson"
PLAYWRIGHT_SCRIPT="$WORK_DIR/operation-e2e.js"
PLAYWRIGHT_RESULT="$WORK_DIR/playwright-result.json"
SCREENSHOT_PATH="../output/playwright/operation-work-fission-e2e.png"
SCREENSHOT_PATH_OUT="../output/playwright/operation-work-fission-e2e-screenshot-path.txt"
GO_PID=""
WECHAT_PID=""

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
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
  [ -f "$WECHAT_LOG" ] && tail -120 "$WECHAT_LOG" >&2 || true
  exit 1
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
  local query="$1"
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "$query" | tr -d '\r'
}

if ! NODE_PATH="$PLAYWRIGHT_NODE_PATH${NODE_PATH:+:$NODE_PATH}" node -e 'require("playwright")' >/dev/null 2>&1; then
  echo "Node Playwright module is not available; set PLAYWRIGHT_NODE_PATH or install playwright" >&2
  exit 1
fi
if [ ! -f web/apps/operation/dist/index.html ]; then
  echo "operation dist not found: web/apps/operation/dist" >&2
  exit 1
fi

mkdir -p ../output/playwright
assert_port_free "$GO_ADDR"
assert_port_free "$SIDEBAR_ADDR"
assert_port_free "$OPERATION_ADDR"
assert_port_free "$WECHAT_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"

python3 - "$WECHAT_ADDR" "$WECHAT_LOG" "$GO_ADDR" <<'PY' &
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

host, port = sys.argv[1].rsplit(":", 1)
log_path = sys.argv[2]
go_addr = sys.argv[3]

def append(event):
    with open(log_path, "a", encoding="utf-8") as fh:
        fh.write(json.dumps(event, ensure_ascii=False, sort_keys=True) + "\n")

class Handler(BaseHTTPRequestHandler):
    def _json(self, payload):
        raw = json.dumps(payload, ensure_ascii=False).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def _log(self, body=b""):
        append({"method": self.command, "path": self.path, "body": body.decode("utf-8", "ignore")})

    def log_message(self, *_):
        return

    def do_GET(self):
        self._log()
        if self.path == "/health":
            self._json({"ok": True})
        elif self.path.startswith("/sns/oauth2/component/access_token"):
            self._json({
                "access_token": "operation-web-token",
                "expires_in": 7200,
                "refresh_token": "operation-refresh-token",
                "openid": "openid-operation-user",
                "scope": "snsapi_userinfo",
                "unionid": "union-operation-user"
            })
        elif self.path.startswith("/sns/userinfo"):
            self._json({
                "openid": "openid-operation-user",
                "nickname": "Go迁移微信昵称",
                "sex": 1,
                "province": "",
                "city": "",
                "country": "",
                "headimgurl": f"http://{go_addr}/static/operation/avatar.png",
                "privilege": [],
                "unionid": "union-operation-user"
            })
        else:
            self.send_response(404)
            self.end_headers()

    def do_POST(self):
        length = int(self.headers.get("Content-Length") or 0)
        body = self.rfile.read(length)
        self._log(body)
        if self.path.startswith("/cgi-bin/component/api_component_token"):
            self._json({
                "errcode": 0,
                "errmsg": "ok",
                "component_access_token": "operation-component-token",
                "expires_in": 7200
            })
        else:
            self.send_response(404)
            self.end_headers()

ThreadingHTTPServer((host, int(port)), Handler).serve_forever()
PY
WECHAT_PID="$!"
wait_url "http://$WECHAT_ADDR/health" 200

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis
compose exec -T redis redis-cli FLUSHDB >/dev/null

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
SET NAMES utf8mb4;
INSERT INTO mc_tenant (id, name, status, logo, login_background, url, copyright, created_at, updated_at)
VALUES (1, 'operation联调租户', 1, '', '', '', '', NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), status=VALUES(status), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_corp (id, name, wx_corpid, social_code, employee_secret, event_callback, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at)
VALUES (1, 'operation联调企业', 'wx-operation-corp', '', 'employee-secret', '', 'contact-secret', '', '', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), wx_corpid=VALUES(wx_corpid), employee_secret=VALUES(employee_secret), contact_secret=VALUES(contact_secret), tenant_id=VALUES(tenant_id), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_official_account (id, app_type, appid, authorized_status, authorizer_appid, nickname, avatar, secret, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (91, 'service', 'component-operation-smoke', 1, 'authorizer-work-fission-smoke', 'Go迁移公众号', 'operation/avatar.png', 'component-secret', 1, 1, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE appid=VALUES(appid), authorized_status=VALUES(authorized_status), authorizer_appid=VALUES(authorizer_appid), nickname=VALUES(nickname), avatar=VALUES(avatar), secret=VALUES(secret), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_official_account_set (id, official_account_id, type, tenant_id, corp_id, create_user_id, created_at, updated_at)
VALUES (91, 91, 7, 1, 1, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE official_account_id=VALUES(official_account_id), type=VALUES(type), tenant_id=VALUES(tenant_id), corp_id=VALUES(corp_id), create_user_id=VALUES(create_user_id), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_work_fission
  (id, corp_id, active_name, service_employees, auto_pass, auto_add_tag, contact_tags, end_time, qr_code_invalid, tasks, new_friend, delete_invalid, receive_prize, receive_prize_employees, receive_links, receive_qrcode, create_user_id, created_at, updated_at)
VALUES
  (900001, 1, 'Go迁移任务宝', '[\"operation-user\"]', 1, 0, '[]', DATE_ADD(NOW(), INTERVAL 7 DAY), 7, '[{\"count\":1},{\"count\":3}]', 0, 1, 0, '[]', '[]', '{\"url\":\"operation/prize.png\"}', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), active_name=VALUES(active_name), service_employees=VALUES(service_employees), auto_pass=VALUES(auto_pass), tasks=VALUES(tasks), end_time=VALUES(end_time), delete_invalid=VALUES(delete_invalid), receive_prize=VALUES(receive_prize), receive_qrcode=VALUES(receive_qrcode), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_work_fission_poster
  (id, fission_id, poster_type, cover_pic, wx_cover_pic, foward_text, avatar_show, nickname_show, nickname_color, card_corp_image_name, card_corp_name, card_corp_logo, qrcode_w, qrcode_h, qrcode_x, qrcode_y, qrcode_id, qrcode_url, created_at, updated_at)
VALUES
  (900001, 900001, 0, 'operation/cover.png', '', 'Go迁移转发话术', 1, 1, '#123456', 'Go迁移企业形象', 'Go迁移企业', 'operation/avatar.png', '72', '72', '120', '260', 'poster-qrcode-id', 'operation/qrcode.png', NOW(), NOW())
ON DUPLICATE KEY UPDATE poster_type=VALUES(poster_type), cover_pic=VALUES(cover_pic), foward_text=VALUES(foward_text), avatar_show=VALUES(avatar_show), nickname_show=VALUES(nickname_show), nickname_color=VALUES(nickname_color), qrcode_w=VALUES(qrcode_w), qrcode_h=VALUES(qrcode_h), qrcode_x=VALUES(qrcode_x), qrcode_y=VALUES(qrcode_y), qrcode_id=VALUES(qrcode_id), qrcode_url=VALUES(qrcode_url), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_work_fission_contact
  (id, fission_id, union_id, nickname, avatar, contact_superior_user_parent, level, employee, invite_count, loss, status, receive_level, is_new, external_user_id, qrcode_id, qrcode_url, created_at, updated_at)
VALUES
  (900001, 900001, 'union-operation-user', 'Go迁移微信昵称', 'operation/avatar.png', 0, 1, 'operation-user', 0, 0, 0, 0, 1, 'external-operation-user', 'existing-qrcode-id', 'operation/qrcode.png', NOW(), NOW()),
  (900002, 900001, 'union-operation-child-a', 'Go迁移助力好友A', 'operation/avatar.png', 900001, 2, 'operation-user', 0, 0, 1, 0, 1, 'external-operation-child-a', '', '', NOW(), NOW()),
  (900003, 900001, 'union-operation-child-b', 'Go迁移助力好友B', 'operation/avatar.png', 900001, 2, 'operation-user', 0, 0, 1, 0, 1, 'external-operation-child-b', '', '', NOW(), NOW())
ON DUPLICATE KEY UPDATE fission_id=VALUES(fission_id), union_id=VALUES(union_id), nickname=VALUES(nickname), avatar=VALUES(avatar), contact_superior_user_parent=VALUES(contact_superior_user_parent), level=VALUES(level), employee=VALUES(employee), invite_count=VALUES(invite_count), loss=VALUES(loss), status=VALUES(status), receive_level=VALUES(receive_level), is_new=VALUES(is_new), external_user_id=VALUES(external_user_id), qrcode_id=VALUES(qrcode_id), qrcode_url=VALUES(qrcode_url), updated_at=NOW(), deleted_at=NULL;
SQL

mkdir -p "$WORK_DIR/upload/operation"
python3 - "$WORK_DIR/upload/operation/cover.png" "$WORK_DIR/upload/operation/avatar.png" "$WORK_DIR/upload/operation/qrcode.png" "$WORK_DIR/upload/operation/prize.png" <<'PY'
import base64
import pathlib
import sys

raw = base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
for target in sys.argv[1:]:
    pathlib.Path(target).write_bytes(raw)
PY

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ENABLE_FRONTEND_SERVERS=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_SIDEBAR_FRONTEND_ADDR="$SIDEBAR_ADDR" \
  MOCHAT_OPERATION_FRONTEND_ADDR="$OPERATION_ADDR" \
  MOCHAT_PHP_UPSTREAM="not-a-valid-upstream-url" \
  MOCHAT_SOURCE_ROOT="$WORK_DIR/should-not-read-php-source" \
  MOCHAT_COMPAT_MANIFEST="$WORK_DIR/should-not-read-manifest.json" \
  MOCHAT_API_BASE_URL="http://$GO_ADDR" \
  MOCHAT_OPERATION_BASE_URL="http://$OPERATION_ADDR" \
  MOCHAT_FILE_STORAGE_ROOT="$WORK_DIR/upload" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_WECHAT_API_BASE_URL="http://$WECHAT_ADDR" \
  MOCHAT_WECHAT_OPEN_PLATFORM_APP_ID="component-operation-smoke" \
  MOCHAT_WECHAT_OPEN_PLATFORM_SECRET="component-secret" \
  MOCHAT_WECHAT_COMPONENT_VERIFY_TICKET="operation-component-ticket" \
  MOCHAT_SIMPLE_JWT_SECRET="operation-front-secret" \
  MOCHAT_SIMPLE_JWT_PREFIX=mc_jwt_ \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
wait_url "http://$OPERATION_ADDR/workFission?id=900001" 200

LOAD_COOKIE="$WORK_DIR/load.cookie"
load_get_code="$(curl -sS -D "$WORK_DIR/load-get.headers" -o "$WORK_DIR/load-get.body" -w '%{http_code}' -c "$LOAD_COOKIE" "http://$GO_ADDR/load/legacy-load?official_account_id=91")"
load_post_code="$(curl -sS -D "$WORK_DIR/load-post.headers" -o "$WORK_DIR/load-post.body" -w '%{http_code}' -b "$LOAD_COOKIE" -c "$LOAD_COOKIE" -H "Content-Type: application/x-www-form-urlencoded" -d "code=legacy-load-code&appid=authorizer-work-fission-smoke&official_account_id=91" "http://$GO_ADDR/load/legacy-load")"
python3 - "$WORK_DIR/load-get.headers" "$WORK_DIR/load-post.headers" "$load_get_code" "$load_post_code" "$OPERATION_ADDR" <<'PY'
import sys
from urllib.parse import parse_qs, unquote, urlparse

get_headers, post_headers, get_code, post_code, operation_addr = sys.argv[1:]

def location(path):
    last = ""
    for line in open(path, encoding="utf-8", errors="ignore"):
        if line.lower().startswith("location:"):
            last = line.split(":", 1)[1].strip()
    return last

get_location = location(get_headers)
post_location = location(post_headers)
if get_code != "302" or post_code != "302":
    raise SystemExit(f"unexpected /load/{{params?}} status: {get_code}, {post_code}")
parsed = urlparse(get_location)
query = parse_qs(parsed.query)
if parsed.netloc != "open.weixin.qq.com" or parsed.path != "/connect/oauth2/authorize":
    raise SystemExit(f"unexpected /load oauth location: {get_location}")
expected_target = f"http://{operation_addr}/legacy-load"
if query.get("appid") != ["authorizer-work-fission-smoke"]:
    raise SystemExit(f"unexpected /load appid: {query}")
if query.get("component_appid") != ["component-operation-smoke"]:
    raise SystemExit(f"unexpected /load component_appid: {query}")
if query.get("response_type") != ["code"] or query.get("scope") != ["snsapi_userinfo"]:
    raise SystemExit(f"unexpected /load oauth parameters: {query}")
redirect_uri = query.get("redirect_uri", [""])[0]
if not redirect_uri.startswith(f"http://{operation_addr}/load?"):
    raise SystemExit(f"unexpected /load redirect_uri: {redirect_uri}")
redirect_query = parse_qs(urlparse(redirect_uri).query)
if redirect_query.get("target") != [expected_target] or redirect_query.get("official_account_id") != ["91"]:
    raise SystemExit(f"unexpected /load redirect query: {redirect_query}")
if unquote(post_location) != expected_target:
    raise SystemExit(f"unexpected /load callback target: {post_location}")
PY

cat >"$PLAYWRIGHT_SCRIPT" <<'JS'
const fs = require("fs");
const { chromium } = require("playwright");

const [operationAddr, resultPath, screenshotPath] = process.argv.slice(2);
const origin = `http://${operationAddr}`;
const targetURL = `${origin}/workFission?id=900001`;
const authTarget = "/workFission?id=900001";
const authURL = `${origin}/auth/workFission?id=900001&target=${encodeURIComponent(authTarget)}`;
const authCallbackURL = `${origin}/auth/workFission?id=900001&target=${encodeURIComponent(authTarget)}&code=operation-oauth-code&appid=authorizer-work-fission-smoke`;
const speedURL = `${origin}/speed?fission_id=900001&union_id=union-operation-user`;

function normalizeOperationPath(url) {
  const parsed = new URL(url);
  let path = parsed.pathname;
  if (path.startsWith("/undefined/operation/")) {
    path = path.replace("/undefined", "");
  }
  return path;
}

function waitForEndpoint(endpoints, path, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  return new Promise((resolve, reject) => {
    const tick = () => {
      if (endpoints[path]) {
        resolve(endpoints[path]);
        return;
      }
      if (Date.now() > deadline) {
        reject(new Error(`missing endpoint ${path}`));
        return;
      }
      setTimeout(tick, 100);
    };
    tick();
  });
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext();
  await context.route(/hm\.baidu\.com|open\.weixin\.qq\.com|open\.work\.weixin\.qq\.com|res\.wx\.qq\.com/, route => {
    route.fulfill({ status: 204, body: "" });
  });

  const page = await context.newPage();
  const endpoints = {};
  const failures = [];
  const localRequestErrors = [];
  let authRedirectURL = "";
  const authChecks = {
    status: false,
    oauthHost: false,
    appid: false,
    componentAppID: false,
    redirectURI: false,
    responseType: false,
    scope: false,
  };
  page.on("response", async response => {
    const url = response.url();
    if (!url.startsWith(origin)) {
      return;
    }
    const path = normalizeOperationPath(url);
    if (response.status() >= 400) {
      localRequestErrors.push(`${response.status()} ${url}`);
    }
    if (path === "/auth/workFission" || path === "/operation/auth/workFission") {
      endpoints[path] = { status: response.status(), originalURL: url, location: response.headers().location || "" };
      return;
    }
    if (![
      "/openUserInfo/workFission",
      "/operation/workFission/poster",
      "/operation/workFission/taskData",
      "/operation/workFission/inviteFriends",
      "/operation/workFission/receive",
    ].includes(path)) {
      return;
    }
    let payload = null;
    try {
      payload = await response.json();
    } catch (_) {
      payload = null;
    }
    endpoints[path] = { status: response.status(), originalURL: url, payload };
  });
  page.on("requestfailed", request => {
    const url = request.url();
    if (url.startsWith(origin)) {
      localRequestErrors.push(`${url} ${request.failure()?.errorText || ""}`);
    }
  });

  try {
    const authResponse = await fetch(authURL, { redirect: "manual" });
    authRedirectURL = authResponse.headers.get("location") || "";
    authChecks.status = authResponse.status === 302;
    const redirect = new URL(authRedirectURL);
    authChecks.oauthHost = redirect.origin === "https://open.weixin.qq.com" && redirect.pathname === "/connect/oauth2/authorize";
    authChecks.appid = redirect.searchParams.get("appid") === "authorizer-work-fission-smoke";
    authChecks.componentAppID = redirect.searchParams.get("component_appid") === "component-operation-smoke";
    authChecks.redirectURI = redirect.searchParams.get("redirect_uri") === `${origin}/auth/workFission?target=${encodeURIComponent(targetURL)}`;
    authChecks.responseType = redirect.searchParams.get("response_type") === "code";
    authChecks.scope = redirect.searchParams.get("scope") === "snsapi_userinfo";
  } catch (error) {
    failures.push(`auth redirect failed: ${error.message}`);
  }

  await page.goto(authCallbackURL, { waitUntil: "domcontentloaded", timeout: 30000 });
  if (page.url() !== targetURL) {
    failures.push(`auth callback target = ${page.url()}, want ${targetURL}`);
  }
  await Promise.all([
    waitForEndpoint(endpoints, "/openUserInfo/workFission"),
    waitForEndpoint(endpoints, "/operation/workFission/poster"),
  ]);
  await page.waitForTimeout(1000);
  const posterText = await page.locator("body").innerText();

  await page.getByText("查看助力进度").click({ timeout: 8000 }).catch(async error => {
    failures.push(`click 查看助力进度 failed: ${error.message}`);
    await page.goto(speedURL, { waitUntil: "domcontentloaded", timeout: 30000 });
  });
  await Promise.all([
    waitForEndpoint(endpoints, "/operation/workFission/taskData"),
    waitForEndpoint(endpoints, "/operation/workFission/inviteFriends"),
  ]);
  await page.waitForTimeout(1000);
  const speedTextBeforeReceive = await page.locator("body").innerText();

  await page.getByText("+1人").first().click({ timeout: 8000 }).catch(error => {
    failures.push(`click gift failed: ${error.message}`);
  });
  await page.getByText("前往领取").click({ timeout: 8000 }).catch(error => {
    failures.push(`click receive failed: ${error.message}`);
  });
  await waitForEndpoint(endpoints, "/operation/workFission/receive", 10000).catch(error => {
    failures.push(error.message);
  });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: screenshotPath, fullPage: true });
  const speedTextAfterReceive = await page.locator("body").innerText();
  const sessionCookie = (await context.cookies(origin)).find(cookie => cookie.name === "MOCHAT_SESSION_ID");
  await browser.close();

  const required = [
    "/openUserInfo/workFission",
    "/operation/workFission/poster",
    "/operation/workFission/taskData",
    "/operation/workFission/inviteFriends",
    "/operation/workFission/receive",
  ];
  const missing = required.filter(path => !endpoints[path]);
  const bad = Object.entries(endpoints)
    .filter(([path, entry]) => !path.includes("/auth/") && (entry.status >= 400 || !entry.payload || entry.payload.code !== 200))
    .map(([path, entry]) => `${path} status=${entry.status} code=${entry.payload && entry.payload.code}`);
  const result = {
    ok: missing.length === 0 && bad.length === 0 && failures.length === 0 && localRequestErrors.length === 0 && Object.values(authChecks).every(Boolean) && Boolean(sessionCookie),
    targetURL,
    authURL,
    authCallbackURL,
    authRedirectURL,
    speedURL,
    screenshotPath,
    endpoints,
    missing,
    bad,
    failures,
    localRequestErrors,
    authChecks,
    sessionCookieSet: Boolean(sessionCookie),
    textChecks: {
      posterCopy: posterText.includes("Go迁移转发话术"),
      nickname: posterText.includes("Go迁移微信昵称"),
      speedTitle: speedTextBeforeReceive.includes("我的助力好友"),
      helperA: speedTextBeforeReceive.includes("Go迁移助力好友A"),
      helperB: speedTextBeforeReceive.includes("Go迁移助力好友B"),
      receiveMask: speedTextAfterReceive.includes("领取奖品") || speedTextAfterReceive.includes("已领取"),
    },
  };
  fs.writeFileSync(resultPath, JSON.stringify(result, null, 2));
  if (!result.ok) {
    throw new Error(JSON.stringify(result, null, 2));
  }
  console.log(JSON.stringify(result, null, 2));
})();
JS

NODE_PATH="$PLAYWRIGHT_NODE_PATH${NODE_PATH:+:$NODE_PATH}" node "$PLAYWRIGHT_SCRIPT" "$OPERATION_ADDR" "$PLAYWRIGHT_RESULT" "$SCREENSHOT_PATH"
printf '%s\n' "$SCREENSHOT_PATH" >"$SCREENSHOT_PATH_OUT"

python3 - "$PLAYWRIGHT_RESULT" <<'PY'
import json
import sys

result = json.load(open(sys.argv[1], encoding="utf-8"))
if not result.get("ok"):
    raise SystemExit(json.dumps(result, ensure_ascii=False, indent=2))
for key, value in result.get("textChecks", {}).items():
    if not value:
        raise SystemExit(f"text check failed: {key}")
for key, value in result.get("authChecks", {}).items():
    if not value:
        raise SystemExit(f"auth check failed: {key}")
if not result.get("sessionCookieSet"):
    raise SystemExit("operation session cookie was not set by auth callback")
print("operation workFission frontend smoke passed")
PY

test "$(mysql_scalar "SELECT receive_level FROM mc_work_fission_contact WHERE id = 900001")" = "1"

python3 - "$WECHAT_LOG" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
if not any(event["path"].startswith("/cgi-bin/component/api_component_token") and "operation-component-ticket" in event.get("body", "") for event in events):
    raise SystemExit("missing component token request")
if not any(event["path"].startswith("/sns/oauth2/component/access_token") and "operation-oauth-code" in event["path"] for event in events):
    raise SystemExit("missing oauth access token request")
if not any(event["path"].startswith("/sns/oauth2/component/access_token") and "legacy-load-code" in event["path"] for event in events):
    raise SystemExit("missing /load oauth access token request")
if not any(event["path"].startswith("/sns/userinfo") and "openid-operation-user" in event["path"] for event in events):
    raise SystemExit("missing oauth userinfo request")
PY
