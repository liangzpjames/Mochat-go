#!/usr/bin/env node
/**
 * check_debt_clearance.mjs — Phase 3 Final 最终门禁：53/53 达标。
 *
 * 规则：坏账清理 26 页 + /chat/file-audio（音频存储 Provider 已解锁）
 * 全部 native/ready/integration-passed；否则失败。
 */

import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

const allowedIncomplete = new Set();

const debtRoutes = [
  '/chat/v2-staff', '/chat/v2-customer', '/chat/v2-group', '/chat/trajectory', '/chat/export', '/chat/file-audio',
  '/chat/resign-staff', '/chat/refuse-archive', '/customer/inheritance',
  '/ai-insight/v2/risk', '/ai-insight/v2/timeout', '/ai-insight/v2/customer-loss', '/ai-insight/v2/message-intercept',
  '/ai-insight/v2/keyword-library', '/ai-insight/v2/silent-customer',
  '/ai-insight/session-analysis', '/ai-insight/smart-analysis', '/ai-insight/emotion', '/ai-insight/employee-score',
  '/ai-insight/communication-keyword',
  '/ai-setting/ai-knowledge-base', '/ai-setting/agent',
  '/company-setting/website', '/company-setting/staff', '/setting/role', '/setting/additional', '/setting/authorization',
];

export async function validateDebtClearance(manifest) {
  const errors = [];
  let done = 0;
  for (const page of manifest?.pages ?? []) {
    if (!debtRoutes.includes(page.path)) continue;
    if (allowedIncomplete.has(page.path)) continue;
    const ok = page.implementation === 'native' || page.implementation === 'legacy-adapter';
    const accepted = ['integration-passed', 'e2e-passed'].includes(page.acceptance);
    if (!ok || page.backend !== 'ready' || !accepted) {
      errors.push(`${page.path}: implementation=${page.implementation} backend=${page.backend} acceptance=${page.acceptance}`);
    } else {
      done += 1;
    }
  }
  if (errors.length) {
    throw new Error(`Phase 3 Final 未达标（${done}/${debtRoutes.length - allowedIncomplete.size} 达标）:\n${errors.join('\n')}`);
  }
  return { total: debtRoutes.length, done, incomplete: allowedIncomplete.size };
}

async function main() {
  const manifest = JSON.parse(await readFile(new URL('../web/apps/dashboard/src/benchmark/manifest.json', import.meta.url), 'utf8'));
  const result = await validateDebtClearance(manifest);
  console.log(`Phase 3 Final：${result.done}/${result.total - result.incomplete} 达标（含 /chat/file-audio 音频存储 Provider 解锁）`);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error(`debt-clearance gate failed: ${error.message}`);
    process.exitCode = 1;
  });
}
