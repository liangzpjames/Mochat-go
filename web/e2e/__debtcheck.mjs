import { chromium } from '@playwright/test';

const base = process.env.DEBT_ACCEPT_BASE ?? 'http://127.0.0.1:18080';
const phone = process.env.DEBT_ACCEPT_PHONE ?? '13800000000';
const password = process.env.DEBT_ACCEPT_PASSWORD ?? 'MochatLocal@123';

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 1280, height: 1600 } });
await page.goto(`${base}/login`, { waitUntil: 'domcontentloaded' });
await page.locator('#login-phone').fill(phone);
await page.locator('#login-password').fill(password);
await page.getByRole('button', { name: '登录' }).click();
await page.waitForURL(/\/index/, { timeout: 20000 }).catch(() => {});
await page.waitForTimeout(1000);

const routes = [
  '/ai-setting/ai-knowledge-base',
  '/ai-setting/agent',
  '/setting/role',
  '/setting/authorization',
  '/company-setting/staff',
  '/company-setting/website',
  '/ai-insight/session-analysis',
  '/ai-insight/smart-analysis',
  '/ai-insight/v2/message-intercept',
  '/chat/resign-staff',
  '/customer/inheritance',
];

const results = [];
for (const route of routes) {
  await page.goto(`${base}${route}`, { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(1800);
  const h1 = await page.locator('h1').first().textContent().catch(() => '');
  const body = await page.locator('body').innerText().catch(() => '');
  const error = /加载失败|出错了|Cannot read|undefined is not|Failed to/.test(body);
  const placeholder = /TODO|占位|演示数据|Demo|placeholder/i.test(body);
  const activeMenu = await page.evaluate(() => document.querySelector('.dashboard-menu-link-active')?.textContent?.trim() ?? '').catch(() => '');
  results.push({ route, h1: h1?.trim(), error, placeholder, activeMenu });
}
console.log(JSON.stringify(results, null, 2));
await browser.close();
