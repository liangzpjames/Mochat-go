#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
. ./scripts/production_url_guard.sh

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: MOCHAT_REAL_SAAS_BASE_URL=https://mochat-prod.your-domain.cn ./scripts/capture_real_saas_tenants_evidence.sh

用生产 API 采集真实 SaaS 多租户数据回归证据，生成 docs/phases/phase-pre0-standalone/evidence/production/real-saas-tenants.md。

必填环境变量：
  MOCHAT_REAL_SAAS_BASE_URL
      生产站点 base URL。
  MOCHAT_REAL_SAAS_TENANT_A_TOKEN
  MOCHAT_REAL_SAAS_TENANT_B_TOKEN
      两个真实租户的 dashboard JWT 或等价 Bearer token。
  MOCHAT_REAL_SAAS_TENANT_A_MARKER
  MOCHAT_REAL_SAAS_TENANT_B_MARKER
      各自响应中必须出现的租户/企业/用户标识，用于证明企业归属和菜单权限未串租户。
  MOCHAT_REAL_SAAS_TENANT_A_FORBIDDEN_PATHS
  MOCHAT_REAL_SAAS_TENANT_B_FORBIDDEN_PATHS
      用 A token 访问 B 资源、用 B token 访问 A 资源时应被拒绝的路径；空格或逗号分隔。
  MOCHAT_REAL_SAAS_QUOTA_EVIDENCE
  MOCHAT_REAL_SAAS_UPLOAD_LEDGER_EVIDENCE
  MOCHAT_REAL_SAAS_ASYNC_EVIDENCE
  MOCHAT_REAL_SAAS_ALERT_EVIDENCE
      真实生产上的额度拦截、上传账本、异步任务和告警证据摘要或 @文件路径。

可选环境变量：
  MOCHAT_REAL_SAAS_OUT
      输出 Markdown，默认 docs/phases/phase-pre0-standalone/evidence/production/real-saas-tenants.md。
  MOCHAT_REAL_SAAS_TENANT_A_NAME
  MOCHAT_REAL_SAAS_TENANT_B_NAME
      证据中展示的租户名称，默认 tenant-a / tenant-b。
  MOCHAT_REAL_SAAS_READ_PATHS
      两租户都要访问的读路径，默认：
      /dashboard/auth/session /dashboard/corpData/index /dashboard/role/index /dashboard/workEmployee/searchCondition
  MOCHAT_REAL_SAAS_AUTH_HEADER
      认证头名，默认 Authorization。
  MOCHAT_REAL_SAAS_AUTH_PREFIX
      认证值前缀，默认 Bearer。
  MOCHAT_REAL_SAAS_ALLOWED_FORBIDDEN_STATUS
      跨租户禁止访问允许状态码，默认 400,401,403,404。
  MOCHAT_REAL_SAAS_EXPECTED_READ_STATUS
      正常读路径期望状态码，默认 200。
  MOCHAT_REAL_SAAS_IGNORE_HTTPS_ERRORS
      设为 1 时忽略 Node TLS 校验，默认 0。
  MOCHAT_PRODUCTION_EVIDENCE_ALLOW_NON_PROD_URLS
      设为 1 时允许示例域名或本机地址，仅用于本地 fixture / smoke，默认 0。

该脚本只采集生产证据，不启动 24 小时持续运行。
EOF
  exit 0
fi

BASE_URL="${MOCHAT_REAL_SAAS_BASE_URL:-}"
if [ -z "$BASE_URL" ]; then
  echo "MOCHAT_REAL_SAAS_BASE_URL is required" >&2
  exit 2
fi
validate_production_base_url MOCHAT_REAL_SAAS_BASE_URL "$BASE_URL"

SOURCE_FINGERPRINT="${MOCHAT_EVIDENCE_SOURCE_FINGERPRINT:-$(python3 ./scripts/source_fingerprint.py | python3 -c 'import json, sys; print(json.load(sys.stdin)["fingerprint"])')}"
export MOCHAT_EVIDENCE_SOURCE_FINGERPRINT="$SOURCE_FINGERPRINT"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-real-saas.XXXXXX")"
cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT INT TERM

SCRIPT_PATH="$WORK_DIR/capture-real-saas.js"

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

function buildURL(baseURL, requestPath) {
  return new URL(requestPath, baseURL).toString();
}

function excerpt(text, limit = 1200) {
  const normalized = redactSensitiveText(text).replace(/\s+/g, ' ').trim();
  return normalized.length > limit ? `${normalized.slice(0, limit)}...` : normalized;
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
  const headers = {
    Accept: 'application/json,text/plain,*/*',
  };
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
      tenant: label,
      path: requestPath,
      url,
      status: response.status,
      ok: response.ok,
      body: text,
      bodyExcerpt: excerpt(text),
      started,
      error: '',
    };
  } catch (err) {
    return {
      tenant: label,
      path: requestPath,
      url,
      status: 0,
      ok: false,
      body: '',
      bodyExcerpt: '',
      started,
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

async function main() {
  if (env('MOCHAT_REAL_SAAS_IGNORE_HTTPS_ERRORS', '0') === '1') {
    process.env.NODE_TLS_REJECT_UNAUTHORIZED = '0';
  }

  const baseURL = env('MOCHAT_REAL_SAAS_BASE_URL');
  const outputPath = env('MOCHAT_REAL_SAAS_OUT', 'docs/phases/phase-pre0-standalone/evidence/production/real-saas-tenants.md');
  const sourceFingerprint = env('MOCHAT_EVIDENCE_SOURCE_FINGERPRINT');
  const timeoutMs = Number(env('MOCHAT_REAL_SAAS_TIMEOUT_MS', '20000'));
  const expectedReadStatuses = parseStatusSet(env('MOCHAT_REAL_SAAS_EXPECTED_READ_STATUS', '200'));
  const allowedForbiddenStatuses = parseStatusSet(env('MOCHAT_REAL_SAAS_ALLOWED_FORBIDDEN_STATUS', '400,401,403,404'));
  const options = {
    timeoutMs,
    authHeader: env('MOCHAT_REAL_SAAS_AUTH_HEADER', 'Authorization'),
    authPrefix: env('MOCHAT_REAL_SAAS_AUTH_PREFIX', 'Bearer'),
  };

  const tenantA = {
    label: '租户 A',
    name: env('MOCHAT_REAL_SAAS_TENANT_A_NAME', 'tenant-a'),
    token: env('MOCHAT_REAL_SAAS_TENANT_A_TOKEN'),
    marker: env('MOCHAT_REAL_SAAS_TENANT_A_MARKER'),
    forbiddenPaths: parseList(env('MOCHAT_REAL_SAAS_TENANT_A_FORBIDDEN_PATHS')),
  };
  const tenantB = {
    label: '租户 B',
    name: env('MOCHAT_REAL_SAAS_TENANT_B_NAME', 'tenant-b'),
    token: env('MOCHAT_REAL_SAAS_TENANT_B_TOKEN'),
    marker: env('MOCHAT_REAL_SAAS_TENANT_B_MARKER'),
    forbiddenPaths: parseList(env('MOCHAT_REAL_SAAS_TENANT_B_FORBIDDEN_PATHS')),
  };

  const readPaths = parseList(env(
    'MOCHAT_REAL_SAAS_READ_PATHS',
    '/dashboard/auth/session /dashboard/corpData/index /dashboard/role/index /dashboard/workEmployee/searchCondition',
  ));

  const quotaEvidence = loadEvidenceValue(env('MOCHAT_REAL_SAAS_QUOTA_EVIDENCE'));
  const uploadLedgerEvidence = loadEvidenceValue(env('MOCHAT_REAL_SAAS_UPLOAD_LEDGER_EVIDENCE'));
  const asyncEvidence = loadEvidenceValue(env('MOCHAT_REAL_SAAS_ASYNC_EVIDENCE'));
  const alertEvidence = loadEvidenceValue(env('MOCHAT_REAL_SAAS_ALERT_EVIDENCE'));

  const failures = [];
  for (const item of [
    ['MOCHAT_REAL_SAAS_TENANT_A_TOKEN', tenantA.token],
    ['MOCHAT_REAL_SAAS_TENANT_B_TOKEN', tenantB.token],
    ['MOCHAT_REAL_SAAS_TENANT_A_MARKER', tenantA.marker],
    ['MOCHAT_REAL_SAAS_TENANT_B_MARKER', tenantB.marker],
  ]) {
    if (!item[1]) {
      failures.push(`缺少 ${item[0]}`);
    }
  }
  if (tenantA.forbiddenPaths.length === 0) {
    failures.push('缺少 MOCHAT_REAL_SAAS_TENANT_A_FORBIDDEN_PATHS');
  }
  if (tenantB.forbiddenPaths.length === 0) {
    failures.push('缺少 MOCHAT_REAL_SAAS_TENANT_B_FORBIDDEN_PATHS');
  }
  for (const item of [
    ['MOCHAT_REAL_SAAS_QUOTA_EVIDENCE', quotaEvidence],
    ['MOCHAT_REAL_SAAS_UPLOAD_LEDGER_EVIDENCE', uploadLedgerEvidence],
    ['MOCHAT_REAL_SAAS_ASYNC_EVIDENCE', asyncEvidence],
    ['MOCHAT_REAL_SAAS_ALERT_EVIDENCE', alertEvidence],
  ]) {
    if (!String(item[1] || '').trim()) {
      failures.push(`缺少 ${item[0]}`);
    }
  }
  if (!sourceFingerprint) {
    failures.push('缺少 MOCHAT_EVIDENCE_SOURCE_FINGERPRINT');
  }

  const readResults = [];
  const forbiddenResults = [];
  if (failures.length === 0) {
    for (const requestPath of readPaths) {
      readResults.push(await request(baseURL, requestPath, tenantA.token, tenantA.label, options));
      readResults.push(await request(baseURL, requestPath, tenantB.token, tenantB.label, options));
    }
    for (const requestPath of tenantA.forbiddenPaths) {
      forbiddenResults.push(await request(baseURL, requestPath, tenantA.token, tenantA.label, options));
    }
    for (const requestPath of tenantB.forbiddenPaths) {
      forbiddenResults.push(await request(baseURL, requestPath, tenantB.token, tenantB.label, options));
    }

    for (const item of readResults) {
      if (!expectedReadStatuses.has(item.status)) {
        failures.push(`${item.tenant} 正常读路径 ${item.path} 返回 ${item.status}`);
      }
      const marker = item.tenant === tenantA.label ? tenantA.marker : tenantB.marker;
      if (!item.body.includes(marker)) {
        failures.push(`${item.tenant} 正常读路径 ${item.path} 未包含期望标识 ${marker}`);
      }
    }
    for (const item of forbiddenResults) {
      if (!allowedForbiddenStatuses.has(item.status)) {
        failures.push(`${item.tenant} 跨租户禁止路径 ${item.path} 返回 ${item.status}，不在允许拒绝状态内`);
      }
    }
  }

  const passed = failures.length === 0;
  const generatedAt = new Date().toLocaleString('zh-CN', { hour12: false, timeZoneName: 'short' });
  const readSummary = readResults.map((item) => `${item.tenant} ${item.path} HTTP ${item.status}`).join('；') || '未执行';
  const forbiddenSummary = forbiddenResults.map((item) => `${item.tenant} ${item.path} HTTP ${item.status}`).join('；') || '未执行';

  const lines = [
    '# 真实 SaaS 多租户数据回归证据',
    '',
    `- 执行时间：${generatedAt}`,
    `- 环境：生产`,
    `- 源码指纹：${sourceFingerprint}`,
    `- 租户 A：${tenantA.name}`,
    `- 租户 B：${tenantB.name}`,
    `- 结论：${passed ? '通过' : '失败'}`,
    '',
    '## 必填核验项',
    '',
    `- 租户：${tenantA.name} / ${tenantB.name}`,
    `- 菜单权限：${readSummary}`,
    `- 企业归属：${tenantA.marker} / ${tenantB.marker}`,
    `- 资源额度：${excerpt(quotaEvidence, 600) || '缺失'}`,
    `- 上传账本：${excerpt(uploadLedgerEvidence, 600) || '缺失'}`,
    `- 异步任务：${excerpt(asyncEvidence, 600) || '缺失'}`,
    `- 告警：${excerpt(alertEvidence, 600) || '缺失'}`,
    '',
    '## 证据记录',
    '',
    `- 跨租户访问隔离：${forbiddenSummary}`,
    `- 用量刷新和额度拦截：${excerpt(quotaEvidence, 1000) || '缺失'}`,
    `- 上传账本与回收：${excerpt(uploadLedgerEvidence, 1000) || '缺失'}`,
    `- 异步任务执行记录：${excerpt(asyncEvidence, 1000) || '缺失'}`,
    `- 告警通知和处置：${excerpt(alertEvidence, 1000) || '缺失'}`,
    '',
    '## API 明细',
    '',
  ];

  for (const item of readResults) {
    lines.push(`- 正常读路径 ${item.tenant} \`${item.path}\`：HTTP \`${item.status}\`${item.error ? `，错误：${markdownEscape(item.error)}` : ''}，响应摘要：${markdownEscape(item.bodyExcerpt)}`);
  }
  for (const item of forbiddenResults) {
    lines.push(`- 跨租户禁止路径 ${item.tenant} \`${item.path}\`：HTTP \`${item.status}\`${item.error ? `，错误：${markdownEscape(item.error)}` : ''}，响应摘要：${markdownEscape(item.bodyExcerpt)}`);
  }
  if (failures.length > 0) {
    lines.push('', '## 失败原因', '');
    for (const failure of failures) {
      lines.push(`- ${markdownEscape(failure)}`);
    }
  }

  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
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

MOCHAT_REAL_SAAS_BASE_URL="$BASE_URL" node "$SCRIPT_PATH"
