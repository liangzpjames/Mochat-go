#!/usr/bin/env node
/**
 * c-batch-acceptance.mjs - C-batch live acceptance against the local stack.
 */
import { chromium } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import path from 'node:path';

const base = process.env.MOCHAT_ACCEPT_BASE ?? 'http://127.0.0.1:18080';
const phone = process.env.MOCHAT_ACCEPT_PHONE ?? '13800000000';
const password = process.env.MOCHAT_ACCEPT_PASSWORD ?? 'MochatLocal@123';
const outDir = 'D:\\workspace\\mochat-go\\output\\c-batch-ui-verify-20260807';
await mkdir(outDir, { recursive: true });

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1360, height: 900 } });
const results = [];
const shot = async (name) => page.screenshot({ path: path.join(outDir, `${name}.png`) });

await page.goto(`${base}/login`, { waitUntil: 'domcontentloaded' });
await page.locator('#login-phone').fill(phone);
await page.locator('#login-password').fill(password);
await page.getByRole('button', { name: '登录' }).click();
await page.waitForURL(/\/index/, { timeout: 30000 });
await page.waitForTimeout(1500);

const headerText = await page.locator('.dashboard-account-actions').innerText();
results.push({
  step: 'C1-header-name',
  showsUserName: headerText.includes('超级管理员'),
  showsRawAccount: headerText.includes('账号 1'),
});
await shot('01-header-name');

await page.goto(`${base}/data/report`, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(1800);
results.push({
  step: 'C6-kpi-legend',
  hasLegend: await page.locator('.phase35-kpi-legend').count() > 0,
});
await shot('02-report-legend');
const reportUrl = page.url();
await page.getByRole('link', { name: '查看详情' }).first().click();
await page.waitForTimeout(1200);
results.push({
  step: 'C4-spa-link',
  urlChanged: !page.url().includes(reportUrl),
  onCustomer: page.url().includes('/data/customer'),
});

await page.goto(`${base}/data/customer`, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(1800);
const employeeOptions = await page.locator('select[aria-label="员工"] option').allTextContents();
results.push({
  step: 'C5-employee-dropdown',
  optionCount: employeeOptions.length,
  hasPlaceholder: employeeOptions.includes('全部员工'),
});

await page.goto(`${base}/data/behavior`, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(1800);
results.push({
  step: 'C3-behavior-pagination',
  hasPrev: await page.getByRole('button', { name: '上一页' }).count() > 0,
  hasNext: await page.getByRole('button', { name: '下一页' }).count() > 0,
});
await shot('03-behavior-pagination');

await page.goto(`${base}/customer/order`, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(1800);
results.push({
  step: 'C2-order-status',
  statusBadges: await page.locator('.order-status').count(),
});
await shot('04-order-status');

await page.goto(`${base}/customer/contact`, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(1800);
const contactBody = await page.locator('body').innerText();
results.push({
  step: 'C7-contact-wording',
  usesLink: contactBody.includes('均保存在链接'),
  usesUrl: contactBody.includes('均保存在 URL'),
});

await page.goto(`${base}/ai-setting/ai-knowledge-base`, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(1800);
const deleteButtons = page.getByRole('button', { name: '删除' });
let popconfirm = false;
if ((await deleteButtons.count()) > 0) {
  await deleteButtons.first().click();
  await page.waitForTimeout(600);
  popconfirm = (await page.locator('.ant-popconfirm-title').count()) > 0;
  const cancel = page.getByRole('button', { name: '取消' }).last();
  if (await cancel.count()) await cancel.click();
}
results.push({
  step: 'C8-popconfirm',
  hasDeleteButton: (await deleteButtons.count()) > 0,
  showsPopconfirm: popconfirm,
});
await shot('05-knowledge-base');

console.log(JSON.stringify(results, null, 2));
const failed = results.some((r) =>
  r.showsUserName === false || r.showsRawAccount === true ||
  r.hasLegend === false || r.onCustomer === false ||
  (r.optionCount !== undefined && r.optionCount < 1) || r.hasPlaceholder === false ||
  r.hasPrev === false || r.hasNext === false ||
  (r.statusBadges !== undefined && r.statusBadges < 1) || r.usesLink === false || r.usesUrl === true ||
  (r.hasDeleteButton === true && r.showsPopconfirm === false));
console.log('HITS=', JSON.stringify(results.map((r) => ({
  step: r.step,
  name: r.showsUserName === false,
  raw: r.showsRawAccount === true,
  legend: r.hasLegend === false,
  onCustomer: r.onCustomer === false,
  optionCount: (r.optionCount ?? 0) < 1,
  placeholder: r.hasPlaceholder === false,
  prev: r.hasPrev === false,
  next: r.hasNext === false,
  status: (r.statusBadges ?? 0) < 1,
  link: r.usesLink === false,
  url: r.usesUrl === true,
  popconfirm: r.hasDeleteButton === true && r.showsPopconfirm === false,
}))));
console.log('FAILED=', failed);
if (failed) process.exitCode = 1;
await browser.close();
