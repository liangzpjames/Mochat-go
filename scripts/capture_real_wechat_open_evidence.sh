#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
. ./scripts/production_url_guard.sh

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: MOCHAT_REAL_WECHAT_OPEN_BASE_URL=https://mochat-prod.your-domain.cn ./scripts/capture_real_wechat_open_evidence.sh

用生产 API 和真实联调记录采集微信开放平台联调证据，生成 docs/phases/phase-pre0-standalone/evidence/production/real-wechat-open.md。

必填环境变量：
  MOCHAT_REAL_WECHAT_OPEN_BASE_URL
      生产站点 base URL。
  MOCHAT_REAL_WECHAT_OPEN_TOKEN
      生产 dashboard JWT 或等价 Bearer token。
  MOCHAT_REAL_WECHAT_OPEN_TICKET_EVIDENCE
      第三方平台 ticket 接收、验签、落库记录摘要，或 @文件路径。
  MOCHAT_REAL_WECHAT_OPEN_AUTH_REDIRECT_EVIDENCE
      授权回跳处理和 authorizer 资料写入记录摘要，或 @文件路径。
  MOCHAT_REAL_WECHAT_OPEN_CANCEL_EVIDENCE
      取消授权回调处理记录摘要，或 @文件路径。
  MOCHAT_REAL_WECHAT_OPEN_MESSAGE_CALLBACK_EVIDENCE
      消息回调处理记录摘要，或 @文件路径。
  MOCHAT_REAL_WECHAT_OPEN_LOG_REF
      截图、日志、工单、监控或联调记录引用。

默认请求路径，可用同名环境变量覆盖，值为空格或逗号分隔：
  MOCHAT_REAL_WECHAT_OPEN_PREAUTH_PATHS
      默认 /dashboard/officialAccount/getPreAuthUrl
  MOCHAT_REAL_WECHAT_OPEN_PROFILE_PATHS
      默认 /dashboard/officialAccount/index

可选环境变量：
  MOCHAT_REAL_WECHAT_OPEN_OUT
      输出 Markdown，默认 docs/phases/phase-pre0-standalone/evidence/production/real-wechat-open.md。
  MOCHAT_REAL_WECHAT_OPEN_AUTH_HEADER
      认证头名，默认 Authorization。
  MOCHAT_REAL_WECHAT_OPEN_AUTH_PREFIX
      认证值前缀，默认 Bearer。
  MOCHAT_REAL_WECHAT_OPEN_EXPECTED_STATUS
      正常请求期望状态码，默认 200。
  MOCHAT_REAL_WECHAT_OPEN_MARKER
      所有请求响应中必须出现的标识；未设置时不检查。
  MOCHAT_REAL_WECHAT_OPEN_IGNORE_HTTPS_ERRORS
      设为 1 时忽略 Node TLS 校验，默认 0。
  MOCHAT_PRODUCTION_EVIDENCE_ALLOW_NON_PROD_URLS
      设为 1 时允许示例域名或本机地址，仅用于本地 fixture / smoke，默认 0。

该脚本只采集生产证据，不启动 24 小时持续运行。
EOF
  exit 0
fi

if [ -z "${MOCHAT_REAL_WECHAT_OPEN_BASE_URL:-}" ]; then
  echo "MOCHAT_REAL_WECHAT_OPEN_BASE_URL is required" >&2
  exit 2
fi
validate_production_base_url MOCHAT_REAL_WECHAT_OPEN_BASE_URL "$MOCHAT_REAL_WECHAT_OPEN_BASE_URL"

SOURCE_FINGERPRINT="${MOCHAT_EVIDENCE_SOURCE_FINGERPRINT:-$(python3 ./scripts/source_fingerprint.py | python3 -c 'import json, sys; print(json.load(sys.stdin)["fingerprint"])')}"
export MOCHAT_EVIDENCE_SOURCE_FINGERPRINT="$SOURCE_FINGERPRINT"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-real-wechat-open.XXXXXX")"
cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT INT TERM

SCRIPT_PATH="$WORK_DIR/capture-real-wechat-open.js"

cat >"$SCRIPT_PATH" <<'JS'
const fs = require('fs');
const path = require('path');

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

function excerpt(text, limit = 1000) {
  const normalized = redactSensitiveText(text).replace(/\s+/g, ' ').trim();
  return normalized.length > limit ? `${normalized.slice(0, limit)}...` : normalized;
}

function buildURL(baseURL, requestPath) {
  return new URL(requestPath, baseURL).toString();
}

function loadEvidenceValue(raw) {
  if (!raw) {
    return '';
  }
  if (raw.startsWith('@')) {
    return fs.readFileSync(raw.slice(1), 'utf8');
  }
  if (fs.existsSync(raw) && fs.statSync(raw).isFile()) {
    return fs.readFileSync(raw, 'utf8');
  }
  return raw;
}

function tokenHeader(token, headerName, prefix) {
  if (!prefix) {
    return [headerName, token];
  }
  return [headerName, `${prefix} ${token}`];
}

async function request(baseURL, requestPath, token, label, options) {
  const url = buildURL(baseURL, requestPath);
  const headers = { Accept: 'application/json,text/plain,*/*' };
  const [headerName, headerValue] = tokenHeader(token, options.authHeader, options.authPrefix);
  headers[headerName] = headerValue;
  const started = new Date().toISOString();
  try {
    const response = await fetch(url, {
      method: 'GET',
      headers,
      redirect: 'manual',
      signal: AbortSignal.timeout(options.timeoutMs),
    });
    const text = await response.text();
    return {
      label,
      path: requestPath,
      url,
      status: response.status,
      body: text,
      bodyExcerpt: excerpt(text),
      started,
      error: '',
    };
  } catch (err) {
    return {
      label,
      path: requestPath,
      url,
      status: 0,
      body: '',
      bodyExcerpt: '',
      started,
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

async function main() {
  if (env('MOCHAT_REAL_WECHAT_OPEN_IGNORE_HTTPS_ERRORS', '0') === '1') {
    process.env.NODE_TLS_REJECT_UNAUTHORIZED = '0';
  }

  const baseURL = env('MOCHAT_REAL_WECHAT_OPEN_BASE_URL');
  const outputPath = env('MOCHAT_REAL_WECHAT_OPEN_OUT', 'docs/phases/phase-pre0-standalone/evidence/production/real-wechat-open.md');
  const token = env('MOCHAT_REAL_WECHAT_OPEN_TOKEN');
  const timeoutMs = Number(env('MOCHAT_REAL_WECHAT_OPEN_TIMEOUT_MS', '20000'));
  const expectedStatuses = parseStatusSet(env('MOCHAT_REAL_WECHAT_OPEN_EXPECTED_STATUS', '200'));
  const marker = env('MOCHAT_REAL_WECHAT_OPEN_MARKER');
  const sourceFingerprint = env('MOCHAT_EVIDENCE_SOURCE_FINGERPRINT');
  const logRef = env('MOCHAT_REAL_WECHAT_OPEN_LOG_REF');
  const ticketEvidence = loadEvidenceValue(env('MOCHAT_REAL_WECHAT_OPEN_TICKET_EVIDENCE'));
  const authRedirectEvidence = loadEvidenceValue(env('MOCHAT_REAL_WECHAT_OPEN_AUTH_REDIRECT_EVIDENCE'));
  const cancelEvidence = loadEvidenceValue(env('MOCHAT_REAL_WECHAT_OPEN_CANCEL_EVIDENCE'));
  const messageCallbackEvidence = loadEvidenceValue(env('MOCHAT_REAL_WECHAT_OPEN_MESSAGE_CALLBACK_EVIDENCE'));
  const options = {
    timeoutMs,
    authHeader: env('MOCHAT_REAL_WECHAT_OPEN_AUTH_HEADER', 'Authorization'),
    authPrefix: env('MOCHAT_REAL_WECHAT_OPEN_AUTH_PREFIX', 'Bearer'),
  };

  const groups = [
    {
      label: '预授权',
      paths: parseList(env('MOCHAT_REAL_WECHAT_OPEN_PREAUTH_PATHS', '/dashboard/officialAccount/getPreAuthUrl')),
    },
    {
      label: '资料回填',
      paths: parseList(env('MOCHAT_REAL_WECHAT_OPEN_PROFILE_PATHS', '/dashboard/officialAccount/index')),
    },
  ];

  const failures = [];
  if (!token) {
    failures.push('缺少 MOCHAT_REAL_WECHAT_OPEN_TOKEN');
  }
  for (const [name, value] of [
    ['MOCHAT_REAL_WECHAT_OPEN_TICKET_EVIDENCE', ticketEvidence],
    ['MOCHAT_REAL_WECHAT_OPEN_AUTH_REDIRECT_EVIDENCE', authRedirectEvidence],
    ['MOCHAT_REAL_WECHAT_OPEN_CANCEL_EVIDENCE', cancelEvidence],
    ['MOCHAT_REAL_WECHAT_OPEN_MESSAGE_CALLBACK_EVIDENCE', messageCallbackEvidence],
  ]) {
    if (!String(value || '').trim()) {
      failures.push(`缺少 ${name}`);
    }
  }
  if (!logRef) {
    failures.push('缺少 MOCHAT_REAL_WECHAT_OPEN_LOG_REF');
  }
  if (!sourceFingerprint) {
    failures.push('缺少 MOCHAT_EVIDENCE_SOURCE_FINGERPRINT');
  }
  for (const group of groups) {
    if (group.paths.length === 0) {
      failures.push(`${group.label} 缺少请求路径`);
    }
  }

  const requestResults = [];
  if (failures.length === 0) {
    for (const group of groups) {
      for (const requestPath of group.paths) {
        const item = await request(baseURL, requestPath, token, group.label, options);
        requestResults.push(item);
        if (!expectedStatuses.has(item.status)) {
          failures.push(`${group.label} 路径 ${requestPath} 返回 ${item.status}`);
        }
        if (marker && !item.body.includes(marker)) {
          failures.push(`${group.label} 路径 ${requestPath} 未包含期望标识 ${marker}`);
        }
      }
    }
  }

  const passed = failures.length === 0;
  const generatedAt = new Date().toISOString();
  const pathsSummary = requestResults
    .map((item) => `${item.label}: GET ${item.path} -> ${item.status}`)
    .join('；');
  const responseSummary = requestResults
    .map((item) => `${item.label}: ${item.error ? `ERROR ${item.error}` : item.bodyExcerpt}`)
    .join('；');

  const lines = [
    '# 真实微信开放平台联调证据',
    '',
    `- 执行时间：${generatedAt}`,
    `- 生产站点：${baseURL}`,
    `- 源码指纹：${sourceFingerprint}`,
    `- 结论：${passed ? '通过' : '失败'}`,
    '',
    `${passed ? 'ticket、预授权、授权回跳、资料回填、取消授权、消息回调均已完成真实平台验收。' : 'ticket、预授权、授权回跳、资料回填、取消授权、消息回调仍存在未满足项。'}`,
    '',
    '## 证据记录',
    '',
    `- 请求或操作路径：${markdownEscape(pathsSummary || '无')}`,
    `- 关键响应摘要：${markdownEscape(responseSummary || '无')}`,
    `- ticket 证据：${markdownEscape(excerpt(ticketEvidence, 1200))}`,
    `- 授权回跳证据：${markdownEscape(excerpt(authRedirectEvidence, 1200))}`,
    `- 取消授权证据：${markdownEscape(excerpt(cancelEvidence, 1200))}`,
    `- 消息回调证据：${markdownEscape(excerpt(messageCallbackEvidence, 1200))}`,
    `- 失败重试或异常路径：重复 ticket、重复授权回调、取消授权后隐藏和消息回调幂等按生产联调记录确认。`,
    `- 截图 / 日志 / 工单引用：${markdownEscape(logRef || '缺失')}`,
    '',
  ];

  if (requestResults.length > 0) {
    lines.push('## 请求明细', '');
    lines.push('| 模块 | 路径 | 状态 | 摘要 |');
    lines.push('| --- | --- | ---: | --- |');
    for (const item of requestResults) {
      lines.push(`| ${item.label} | \`${item.path}\` | ${item.status} | ${markdownEscape(item.error || item.bodyExcerpt)} |`);
    }
    lines.push('');
  }

  if (failures.length > 0) {
    lines.push('## 未通过项', '');
    for (const failure of failures) {
      lines.push(`- ${failure}`);
    }
    lines.push('');
  }

  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, `${lines.join('\n')}\n`, 'utf8');

  if (!passed) {
    console.error(`真实微信开放平台联调证据未通过，已写入 ${outputPath}`);
    for (const failure of failures) {
      console.error(`- ${failure}`);
    }
    process.exit(1);
  }

  console.log(outputPath);
}

main().catch((err) => {
  console.error(err instanceof Error ? err.stack : String(err));
  process.exit(1);
});
JS

node "$SCRIPT_PATH"
