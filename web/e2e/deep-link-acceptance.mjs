#!/usr/bin/env node
/**
 * deep-link-acceptance.mjs - B2 regression: five deep-link refreshes stay on
 * the original route, and a post-login returnTo restores the deep link.
 * Defaults to the local desktop stack; override with MOCHAT_ACCEPT_BASE,
 * MOCHAT_ACCEPT_PHONE and MOCHAT_ACCEPT_PASSWORD.
 */
import { chromium } from '@playwright/test';

const base = process.env.MOCHAT_ACCEPT_BASE ?? 'http://127.0.0.1:18080';
const phone = process.env.MOCHAT_ACCEPT_PHONE ?? '13800000000';
const password = process.env.MOCHAT_ACCEPT_PASSWORD ?? 'MochatLocal@123';
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1360, height: 900 } });
const results = [];

await page.goto(`${base}/login`, { waitUntil: 'domcontentloaded' });
await page.locator('#login-phone').fill(phone);
await page.locator('#login-password').fill(password);
await page.getByRole('button', { name: '登录' }).click();
await page.waitForURL(/\/index/, { timeout: 30000 });
await page.waitForTimeout(1200);

for (let i = 1; i <= 5; i += 1) {
  await page.goto(`${base}/data/customer`, { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(2200);
  results.push({
    round: i,
    url: page.url(),
    h1: (await page.locator('h1').first().innerText().catch(() => '')),
    onDeepLink: page.url().includes('/data/customer'),
  });
}

await page.goto(`${base}/login?returnTo=${encodeURIComponent('/data/employee?x=1')}`, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(800);
await page.locator('#login-phone').fill(phone);
await page.locator('#login-password').fill(password);
await page.getByRole('button', { name: '登录' }).click();
await page.waitForTimeout(2500);
results.push({
  step: 'login-return-to',
  url: page.url(),
  onReturnTo: page.url().includes('/data/employee'),
});

console.log(JSON.stringify(results, null, 2));
if (results.some((item) => item.onDeepLink === false) || results.some((item) => item.onReturnTo === false)) {
  process.exitCode = 1;
}
await browser.close();
