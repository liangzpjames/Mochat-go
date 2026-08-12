#!/usr/bin/env node
/**
 * b-batch-acceptance.mjs - B2/B3/B4/B5 live acceptance against the local stack.
 * Defaults: http://127.0.0.1:18080, 13800000000 / MochatLocal@123.
 */
import { chromium } from '@playwright/test';

const base = process.env.MOCHAT_ACCEPT_BASE ?? 'http://127.0.0.1:18080';
const phone = process.env.MOCHAT_ACCEPT_PHONE ?? '13800000000';
const password = process.env.MOCHAT_ACCEPT_PASSWORD ?? 'MochatLocal@123';
const corpId = '1536612155';
const results = [];

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1360, height: 900 } });

const login = async () => {
  await page.goto(`${base}/login`, { waitUntil: 'domcontentloaded' });
  await page.locator('#login-phone').fill(phone);
  await page.locator('#login-password').fill(password);
  await page.getByRole('button', { name: '登录' }).click();
  await page.waitForURL(/\/index/, { timeout: 30000 });
  await page.waitForTimeout(1000);
};

await login();

// B2: five deep-link refreshes stay on the original route.
for (let i = 1; i <= 5; i += 1) {
  await page.goto(`${base}/data/customer`, { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(1800);
  results.push({ step: `B2-refresh-${i}`, onDeepLink: page.url().includes('/data/customer') });
}

// B2: login returnTo restores the deep link.
await page.goto(`${base}/login?returnTo=${encodeURIComponent('/data/employee?x=1')}`, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(600);
results.push({ step: 'B2-return-to-form', returnToInput: await page.locator('#login-phone').inputValue().catch(() => 'n/a'), urlBefore: page.url() });
await page.locator('#login-phone').fill(phone);
await page.locator('#login-password').fill(password);
await page.getByRole('button', { name: '登录' }).click();
try {
  await page.waitForURL(/\/data\/employee/, { timeout: 10000 });
} catch {
  // fall through and record the final URL
}
await page.waitForTimeout(800);
results.push({ step: 'B2-return-to', onReturnTo: page.url().includes('/data/employee') });

// B3: summary report primary metrics equal the API summary.
const auth = await fetch(`${base}/dashboard/user/auth`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ phone, password }),
}).then((r) => r.json());
const token = auth.data.token;
const startAt = '2026-01-01T00:00:00Z';
const endAt = '2026-08-08T00:00:00Z';
const report = await fetch(
  `${base}/dashboard/reports/report?corpId=${corpId}&timezone=Asia%2FShanghai&startAt=${startAt}&endAt=${endAt}`,
  { headers: { Authorization: `Bearer ${token}` } },
).then((r) => r.json());
await page.goto(`${base}/data/report`, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(1800);
const articles = await page.locator('article').allInnerTexts();
const summary = report.data.summary;
const primary = { 客户概览: 'customer', 转化漏斗: 'lead', 订单经营: 'order', 行为审计: 'behavior' };
const b3 = Object.entries(primary).map(([name, key]) => {
  const article = articles.find((text) => text.startsWith(name)) ?? '';
  const value = Number(summary[key] ?? 0);
  return { name, expected: value, ui: article.split('\n')[1] ?? '', match: article.split('\n')[1] === String(value) };
});
results.push({ step: 'B3-primary-metrics', items: b3 });

// B4: SaaS identity login is the only login entry and issues no asset 404s.
const assetRequests = [];
page.on('response', (response) => {
  if (response.status() >= 400 && (response.url().includes('/favicon.ico') || response.url().includes('/img/'))) {
    assetRequests.push(`${response.status()} ${response.url()}`);
  }
});
const saasLoginResponse = await page.goto(`${base}/saas/login`, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(500);
results.push({
  step: 'B4-identity-login',
  status: saasLoginResponse?.status() ?? 0,
  redirectedToSaaSAdmin: page.url().includes('/saas-admin/'),
  asset404s: assetRequests,
  inlineAssets: (await page.locator('html').innerHTML()).includes('data:image/svg+xml'),
});

// B5: AI insight restricted state is unified and actionable.
await page.goto(`${base}/ai-insight/communication-keyword`, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(1800);
const aiBody = await page.locator('body').innerText();
results.push({
  step: 'B5-ai-restricted',
  unifiedCopy: aiBody.includes('AI 能力未接入') && aiBody.includes('当前未接入可用的 AI 分析 Provider，暂无分析结果'),
  hasActionLink: await page.getByRole('link', { name: '前往接入 AI 能力' }).count() > 0,
});

console.log(JSON.stringify(results, null, 2));
const failed = results.some((r) =>
  (Array.isArray(r.items) ? r.items.some((i) => !i.match) : false) ||
  r.onDeepLink === false || r.onReturnTo === false ||
  (r.status !== undefined && r.status >= 400) ||
  r.redirectedToSaaSAdmin === false ||
  (r.asset404s && r.asset404s.length > 0) || r.unifiedCopy === false || r.hasActionLink === false);
console.log('HITS=', JSON.stringify(results.map((r) => ({
  step: r.step,
  itemFail: Array.isArray(r.items) ? r.items.some((i) => !i.match) : false,
  deep: r.onDeepLink === false,
  ret: r.onReturnTo === false,
  status: r.status,
  redirectedToSaaSAdmin: r.redirectedToSaaSAdmin === false,
  assets: !!(r.asset404s && r.asset404s.length > 0),
  copy: r.unifiedCopy === false,
  link: r.hasActionLink === false,
}))));
console.log('FAILED=', failed);
if (failed) process.exitCode = 1;
await browser.close();
