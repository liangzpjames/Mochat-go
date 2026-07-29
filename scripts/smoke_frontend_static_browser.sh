#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18131}"
GO_HOST="${GO_ADDR%:*}"
GO_PORT="${GO_ADDR##*:}"
SIDEBAR_ADDR="${MOCHAT_SIDEBAR_FRONTEND_ADDR:-$GO_HOST:$((GO_PORT + 1))}"
OPERATION_ADDR="${MOCHAT_OPERATION_FRONTEND_ADDR:-$GO_HOST:$((GO_PORT + 2))}"
PLAYWRIGHT_NODE_PATH="${PLAYWRIGHT_NODE_PATH:-$(npm root -g 2>/dev/null || true)}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-front-static.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""

cleanup() {
  local rc=$?
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
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
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
}

if ! NODE_PATH="$PLAYWRIGHT_NODE_PATH${NODE_PATH:+:$NODE_PATH}" node -e 'require("playwright")' >/dev/null 2>&1; then
  echo "Node Playwright module is not available; set PLAYWRIGHT_NODE_PATH or install playwright" >&2
  exit 1
fi

for dist in web/apps/dashboard/dist web/apps/sidebar/dist web/apps/operation/dist; do
  if [ ! -f "$dist/index.html" ]; then
    echo "frontend dist not found: $dist" >&2
    exit 1
  fi
done

assert_port_free "$GO_ADDR"
assert_port_free "$SIDEBAR_ADDR"
assert_port_free "$OPERATION_ADDR"

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
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
wait_url "http://$GO_ADDR/login" 200
wait_url "http://$GO_ADDR/sidebar-app/contact" 200
wait_url "http://$GO_ADDR/operation-app/workFission" 200
wait_url "http://$SIDEBAR_ADDR/contact" 200
wait_url "http://$OPERATION_ADDR/workFission" 200

NODE_PATH="$PLAYWRIGHT_NODE_PATH${NODE_PATH:+:$NODE_PATH}" node - "$GO_ADDR" "$SIDEBAR_ADDR" "$OPERATION_ADDR" <<'JS'
const { chromium } = require("playwright");

const [goAddr, sidebarAddr, operationAddr] = process.argv.slice(2);
const entries = [
  { name: "dashboard", url: `http://${goAddr}/login`, origin: `http://${goAddr}` },
  { name: "sidebar-prefixed", url: `http://${goAddr}/sidebar-app/contact`, origin: `http://${goAddr}` },
  { name: "operation-prefixed", url: `http://${goAddr}/operation-app/workFission`, origin: `http://${goAddr}` },
  { name: "sidebar", url: `http://${sidebarAddr}/contact`, origin: `http://${sidebarAddr}` },
  { name: "operation", url: `http://${operationAddr}/workFission`, origin: `http://${operationAddr}` },
];

function isLocalAsset(url, origin) {
  return url.startsWith(`${origin}/`) && /\.(js|css)(\?|$)/.test(url);
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext();
  const results = [];
  const badAssets = [];
  const failedAssets = [];

  await context.route(/hm\.baidu\.com|open\.work\.weixin\.qq\.com/, route => {
    const url = route.request().url();
    if (url.includes("jwxwork")) {
      return route.fulfill({
        status: 200,
        contentType: "application/javascript",
        body: "window.wx={config:function(){},ready:function(fn){if(fn)fn();},error:function(){}};window.jWeixin=window.wx;"
      });
    }
    return route.fulfill({ status: 204, body: "" });
  });

  for (const entry of entries) {
    const page = await context.newPage();
    const localAssets = new Set();
    page.on("response", response => {
      const url = response.url();
      if (!isLocalAsset(url, entry.origin)) {
        return;
      }
      localAssets.add(url);
      if (response.status() !== 200) {
        badAssets.push(`${entry.name}: ${response.status()} ${url}`);
      }
    });
    page.on("requestfailed", request => {
      const url = request.url();
      if (isLocalAsset(url, entry.origin)) {
        failedAssets.push(`${entry.name}: ${url} ${request.failure()?.errorText || ""}`);
      }
    });

    await page.goto(entry.url, { waitUntil: "domcontentloaded", timeout: 30000 });
    await page.waitForTimeout(1500);
    const appCount = await page.locator("#app").count();
    const html = await page.content();
    results.push({
      name: entry.name,
      url: entry.url,
      appCount,
      title: await page.title(),
      localAssetCount: localAssets.size,
      hasVueMount: html.includes('id="app"') || html.includes("id='app'"),
    });
    await page.close();
  }

  await browser.close();

  for (const result of results) {
    if (result.appCount < 1 || !result.hasVueMount || result.localAssetCount < 2) {
      throw new Error(`frontend entry did not render expected shell: ${JSON.stringify(result)}`);
    }
  }
  if (badAssets.length || failedAssets.length) {
    throw new Error(`frontend local asset failures: ${badAssets.concat(failedAssets).join("; ")}`);
  }
  console.log(JSON.stringify({ ok: true, entries: results }, null, 2));
})();
JS

echo "frontend static browser smoke passed"
