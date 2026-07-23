#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
. ./scripts/production_url_guard.sh

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: MOCHAT_PROD_FRONTEND_BASE_URL=https://mochat-prod.your-domain.cn ./scripts/capture_prod_frontend_evidence.sh

用真实浏览器采集生产前端回归证据，生成 docs/evidence/production/prod-frontend.md。

环境变量：
  MOCHAT_PROD_FRONTEND_BASE_URL
      必填，生产站点 base URL。
  MOCHAT_PROD_FRONTEND_OUT
      输出 Markdown，默认 docs/evidence/production/prod-frontend.md。
  MOCHAT_PROD_FRONTEND_SCREENSHOT_DIR
      截图目录，默认 docs/evidence/production/frontend-screenshots。
  MOCHAT_PROD_FRONTEND_BUILD_VERSION
      构建版本；未设置时记录为 unknown。
  MOCHAT_PROD_FRONTEND_DASHBOARD_PATHS
      dashboard 主路径，空格或逗号分隔，默认 /login。
  MOCHAT_PROD_FRONTEND_SIDEBAR_PATHS
      sidebar 主路径，空格或逗号分隔，默认 /sidebar-app/。
  MOCHAT_PROD_FRONTEND_OPERATION_PATHS
      operation 主路径，空格或逗号分隔，默认 /operation-app/。
  MOCHAT_PROD_FRONTEND_ABNORMAL_PATHS
      异常路径，空格或逗号分隔，默认 /__mochat_go_prod_frontend_not_found__。
  MOCHAT_PROD_FRONTEND_EXPECTED_ABNORMAL_STATUS
      异常路径允许状态码，逗号分隔，默认 401,403,404。
  MOCHAT_PROD_FRONTEND_ALLOWED_STATUS
      主路径额外允许状态码，逗号分隔；默认空，主路径要求 <400。
  MOCHAT_PROD_FRONTEND_AUTH_STATE
      可选，Playwright storageState JSON，用于已登录生产账号回归。
  MOCHAT_PROD_FRONTEND_IGNORE_HTTPS_ERRORS
      设为 1 时忽略 HTTPS 证书错误，默认 0。
  MOCHAT_PRODUCTION_EVIDENCE_ALLOW_NON_PROD_URLS
      设为 1 时允许示例域名或本机地址，仅用于本地 fixture / smoke，默认 0。
  PLAYWRIGHT_NODE_PATH
      Node Playwright 模块目录；默认 npm root -g。

该脚本只做浏览器证据采集，不启动 24 小时持续运行。
EOF
  exit 0
fi

BASE_URL="${MOCHAT_PROD_FRONTEND_BASE_URL:-}"
if [ -z "$BASE_URL" ]; then
  echo "MOCHAT_PROD_FRONTEND_BASE_URL is required" >&2
  exit 2
fi
validate_production_base_url MOCHAT_PROD_FRONTEND_BASE_URL "$BASE_URL"

SOURCE_FINGERPRINT="${MOCHAT_EVIDENCE_SOURCE_FINGERPRINT:-$(python3 ./scripts/source_fingerprint.py | python3 -c 'import json, sys; print(json.load(sys.stdin)["fingerprint"])')}"
export MOCHAT_EVIDENCE_SOURCE_FINGERPRINT="$SOURCE_FINGERPRINT"

PLAYWRIGHT_NODE_PATH="${PLAYWRIGHT_NODE_PATH:-$(npm root -g 2>/dev/null || true)}"
if ! NODE_PATH="$PLAYWRIGHT_NODE_PATH${NODE_PATH:+:$NODE_PATH}" node -e 'require("playwright")' >/dev/null 2>&1; then
  echo "Node Playwright module is not available; set PLAYWRIGHT_NODE_PATH or install playwright" >&2
  exit 2
fi

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-prod-frontend.XXXXXX")"
cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT INT TERM

SCRIPT_PATH="$WORK_DIR/capture-prod-frontend.js"
OUT="${MOCHAT_PROD_FRONTEND_OUT:-docs/evidence/production/prod-frontend.md}"
SCREENSHOT_DIR="${MOCHAT_PROD_FRONTEND_SCREENSHOT_DIR:-docs/evidence/production/frontend-screenshots}"

cat >"$SCRIPT_PATH" <<'JS'
const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');

function env(name, fallback = '') {
  const value = process.env[name];
  return value === undefined || value === '' ? fallback : value;
}

function parseList(value) {
  return String(value || '')
    .split(/[\s,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseStatusSet(value) {
  return new Set(parseList(value).map((item) => Number(item)).filter((item) => Number.isInteger(item)));
}

function redactSensitiveText(value) {
  return String(value ?? '')
    .replace(/\b(Authorization\s*[:=]\s*)(Bearer\s+)?[A-Za-z0-9._~+/=-]{12,}/gi, '$1$2<redacted>')
    .replace(/\b(Bearer\s+)[A-Za-z0-9._~+/=-]{20,}/gi, '$1<redacted>')
    .replace(/(["']?\b(?:access_token|refresh_token|component_access_token|authorizer_access_token|authorizer_refresh_token|pre_auth_code|verify_ticket|token|secret|corpsecret|component_appsecret|encodingaeskey|password|passwd|session_key|jwt)\b["']?\s*[:=]\s*["']?)[^"',&\s`{}]{8,}/gi, '$1<redacted>')
    .replace(/([?&](?:access_token|refresh_token|component_access_token|authorizer_access_token|authorizer_refresh_token|pre_auth_code|verify_ticket|token|secret|corpsecret|component_appsecret|encodingaeskey|password|passwd|session_key|jwt)=)[^&\s]+/gi, '$1<redacted>');
}

function markdownEscape(value) {
  return redactSensitiveText(value).replace(/\r?\n/g, ' ').trim();
}

function relPath(target) {
  const repo = process.cwd();
  const resolved = path.resolve(target);
  return path.relative(repo, resolved) || '.';
}

function safeName(value) {
  return String(value).replace(/[^A-Za-z0-9_.-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 90) || 'page';
}

function buildURL(baseURL, pagePath) {
  return new URL(pagePath, baseURL).toString();
}

function sameOrigin(baseURL, requestURL) {
  try {
    return new URL(requestURL).origin === new URL(baseURL).origin;
  } catch {
    return false;
  }
}

async function main() {
  const baseURL = env('MOCHAT_PROD_FRONTEND_BASE_URL');
  const outputPath = env('MOCHAT_PROD_FRONTEND_OUT', 'docs/evidence/production/prod-frontend.md');
  const screenshotDir = env('MOCHAT_PROD_FRONTEND_SCREENSHOT_DIR', 'docs/evidence/production/frontend-screenshots');
  const buildVersion = env('MOCHAT_PROD_FRONTEND_BUILD_VERSION', 'unknown');
  const sourceFingerprint = env('MOCHAT_EVIDENCE_SOURCE_FINGERPRINT');
  const timeoutMs = Number(env('MOCHAT_PROD_FRONTEND_TIMEOUT_MS', '30000'));
  const allowedMainStatuses = parseStatusSet(env('MOCHAT_PROD_FRONTEND_ALLOWED_STATUS', ''));
  const expectedAbnormalStatuses = parseStatusSet(env('MOCHAT_PROD_FRONTEND_EXPECTED_ABNORMAL_STATUS', '401,403,404'));
  const ignoreHTTPSErrors = env('MOCHAT_PROD_FRONTEND_IGNORE_HTTPS_ERRORS', '0') === '1';
  const authState = env('MOCHAT_PROD_FRONTEND_AUTH_STATE', '');

  const sections = [
    { key: 'dashboard', label: 'dashboard', paths: parseList(env('MOCHAT_PROD_FRONTEND_DASHBOARD_PATHS', '/login')) },
    { key: 'sidebar', label: 'sidebar', paths: parseList(env('MOCHAT_PROD_FRONTEND_SIDEBAR_PATHS', '/sidebar-app/')) },
    { key: 'operation', label: 'operation', paths: parseList(env('MOCHAT_PROD_FRONTEND_OPERATION_PATHS', '/operation-app/')) },
  ];
  const abnormalPaths = parseList(env('MOCHAT_PROD_FRONTEND_ABNORMAL_PATHS', '/__mochat_go_prod_frontend_not_found__'));

  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.mkdirSync(screenshotDir, { recursive: true });

  const browser = await chromium.launch({ args: ['--no-sandbox'] });
  const contextOptions = {
    ignoreHTTPSErrors,
    viewport: { width: 1440, height: 960 },
  };
  if (authState) {
    contextOptions.storageState = authState;
  }
  const context = await browser.newContext(contextOptions);
  const page = await context.newPage();

  const consoleErrors = [];
  const requestFailures = [];
  const badResponses = [];
  const visits = [];
  const abnormalVisits = [];
  let currentVisitContext = null;

  page.on('console', (message) => {
    if (message.type() === 'error') {
      consoleErrors.push({
        page: page.url(),
        text: message.text(),
        abnormal: Boolean(currentVisitContext?.abnormal),
      });
    }
  });
  page.on('requestfailed', (request) => {
    const url = request.url();
    if (sameOrigin(baseURL, url)) {
      requestFailures.push({
        url,
        method: request.method(),
        failure: request.failure()?.errorText || 'request failed',
        abnormal: Boolean(currentVisitContext?.abnormal),
      });
    }
  });
  page.on('response', (response) => {
    const url = response.url();
    const status = response.status();
    if (sameOrigin(baseURL, url) && status >= 400 && !allowedMainStatuses.has(status) && !expectedAbnormalStatuses.has(status)) {
      badResponses.push({ url, status });
    }
  });

  async function visit(section, pagePath, index, abnormal = false) {
    const url = buildURL(baseURL, pagePath);
    let status = 0;
    let finalURL = url;
    let title = '';
    let error = '';
    const screenshotPath = path.join(
      screenshotDir,
      `${section.key}-${String(index + 1).padStart(2, '0')}-${safeName(pagePath)}.png`,
    );
    try {
      currentVisitContext = { section: section.label, path: pagePath, abnormal };
      const response = await page.goto(url, { waitUntil: 'domcontentloaded', timeout: timeoutMs });
      status = response ? response.status() : 0;
      await page.waitForLoadState('networkidle', { timeout: Math.min(timeoutMs, 10000) }).catch(() => {});
      title = await page.title().catch(() => '');
      finalURL = page.url();
      await page.screenshot({ path: screenshotPath, fullPage: true });
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    } finally {
      currentVisitContext = null;
    }

    const record = {
      section: section.label,
      path: pagePath,
      url,
      finalURL,
      status,
      title,
      screenshot: relPath(screenshotPath),
      error,
      abnormal,
    };
    if (abnormal) {
      abnormalVisits.push(record);
    } else {
      visits.push(record);
    }
    return record;
  }

  for (const section of sections) {
    for (let index = 0; index < section.paths.length; index += 1) {
      await visit(section, section.paths[index], index, false);
    }
  }
  for (let index = 0; index < abnormalPaths.length; index += 1) {
    await visit({ key: 'abnormal', label: '异常' }, abnormalPaths[index], index, true);
  }

  await browser.close();

  const mainErrors = [];
  for (const visitRecord of visits) {
    if (visitRecord.error) {
      mainErrors.push(`${visitRecord.section} ${visitRecord.path}: ${visitRecord.error}`);
      continue;
    }
    if (visitRecord.status >= 400 && !allowedMainStatuses.has(visitRecord.status)) {
      mainErrors.push(`${visitRecord.section} ${visitRecord.path}: unexpected HTTP ${visitRecord.status}`);
    }
  }
  for (const visitRecord of abnormalVisits) {
    if (visitRecord.error) {
      mainErrors.push(`异常路径 ${visitRecord.path}: ${visitRecord.error}`);
      continue;
    }
    if (!expectedAbnormalStatuses.has(visitRecord.status)) {
      mainErrors.push(`异常路径 ${visitRecord.path}: expected ${Array.from(expectedAbnormalStatuses).join('/')} got ${visitRecord.status}`);
    }
  }
  const blockingConsoleErrors = consoleErrors.filter((item) => !item.abnormal);
  const blockingRequestFailures = requestFailures.filter((item) => !item.abnormal);

  if (blockingConsoleErrors.length > 0) {
    mainErrors.push(`console error ${blockingConsoleErrors.length} 条`);
  }
  if (blockingRequestFailures.length > 0) {
    mainErrors.push(`同源请求失败 ${blockingRequestFailures.length} 条`);
  }
  if (badResponses.length > 0) {
    mainErrors.push(`同源 4xx/5xx 响应 ${badResponses.length} 条`);
  }

  if (!sourceFingerprint) {
    mainErrors.push('缺少 MOCHAT_EVIDENCE_SOURCE_FINGERPRINT');
  }
  const passed = mainErrors.length === 0;
  const generatedAt = new Date().toLocaleString('zh-CN', { hour12: false, timeZoneName: 'short' });
  const dashboardSummary = visits.filter((item) => item.section === 'dashboard').map((item) => `${item.path} HTTP ${item.status}`).join('；') || '未配置';
  const sidebarSummary = visits.filter((item) => item.section === 'sidebar').map((item) => `${item.path} HTTP ${item.status}`).join('；') || '未配置';
  const operationSummary = visits.filter((item) => item.section === 'operation').map((item) => `${item.path} HTTP ${item.status}`).join('；') || '未配置';
  const mainPathSummary = visits.map((item) => `${item.section} ${item.path} -> HTTP ${item.status}, title=${item.title || 'unknown'}, screenshot=${item.screenshot}`).join('；');
  const abnormalSummary = abnormalVisits.map((item) => `${item.path} -> HTTP ${item.status}, screenshot=${item.screenshot}`).join('；');

  const lines = [
    '# 生产前端浏览器回归证据',
    '',
    `- 执行时间：${generatedAt}`,
    `- 环境：生产`,
    `- 生产：${baseURL}`,
    `- 构建版本：${buildVersion}`,
    `- 源码指纹：${sourceFingerprint}`,
    `- 结论：${passed ? '通过' : '失败'}`,
    '',
    '## 必填核验项',
    '',
    `- dashboard：${dashboardSummary}`,
    `- sidebar：${sidebarSummary}`,
    `- operation：${operationSummary}`,
    `- 生产：${baseURL}`,
    `- 异常：${abnormalSummary || '未配置异常路径'}`,
    '',
    '## 证据记录',
    '',
    `- 主要业务路径：${mainPathSummary || '未配置主路径'}`,
    `- 异常路径：${abnormalSummary || '未配置异常路径'}`,
    `- 控制台错误：${blockingConsoleErrors.length === 0 ? '无阻断性 console error。' : blockingConsoleErrors.map((item) => `${markdownEscape(item.page)} ${markdownEscape(item.text)}`).join('；')}`,
    `- 网络请求错误：${blockingRequestFailures.length === 0 && badResponses.length === 0 ? '无同源 requestfailed 或非预期 4xx/5xx。' : [
      ...blockingRequestFailures.map((item) => `${item.method} ${markdownEscape(item.url)} ${markdownEscape(item.failure)}`),
      ...badResponses.map((item) => `${item.status} ${markdownEscape(item.url)}`),
    ].join('；')}`,
    `- 截图 / 录像 / 报告引用：${relPath(screenshotDir)}`,
    '',
    '## 页面明细',
    '',
  ];

  for (const item of [...visits, ...abnormalVisits]) {
    lines.push(`- ${item.abnormal ? '异常路径' : item.section} \`${item.path}\`：HTTP \`${item.status}\`，最终 URL \`${markdownEscape(item.finalURL)}\`，截图 \`${item.screenshot}\`${item.error ? `，错误：${markdownEscape(item.error)}` : ''}`);
  }
  if (mainErrors.length > 0) {
    lines.push('', '## 失败原因', '');
    for (const item of mainErrors) {
      lines.push(`- ${markdownEscape(item)}`);
    }
  }

  fs.writeFileSync(outputPath, `${lines.join('\n')}\n`, 'utf8');
  console.log(outputPath);
  if (!passed) {
    process.exit(1);
  }
}

main().catch((err) => {
  console.error(err instanceof Error ? err.stack : err);
  process.exit(1);
});
JS

NODE_PATH="$PLAYWRIGHT_NODE_PATH${NODE_PATH:+:$NODE_PATH}" \
  MOCHAT_PROD_FRONTEND_BASE_URL="$BASE_URL" \
  MOCHAT_PROD_FRONTEND_OUT="$OUT" \
  MOCHAT_PROD_FRONTEND_SCREENSHOT_DIR="$SCREENSHOT_DIR" \
  node "$SCRIPT_PATH"
