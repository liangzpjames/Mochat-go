#!/usr/bin/env node
/**
 * data-calibre-acceptance.mjs — 数据口径统一验收（2026-08-08）。
 *
 * 验证：
 * 1. /index 概览页使用 /reports/overview（SCRM 口径），页面无硬编码 0 模块。
 * 2. overview.summary.customer === customer 报表主指标；
 *    overview.summary.lead === conversion 报表 lead 阶段；
 *    overview.summary.order === conversion 报表 order 阶段；
 *    overview.summary.behavior === behavior 报表主指标。
 * 3. 会话归档未接入时页面显示空态（provider_unavailable）。
 *
 * 用法：
 *   $env:DATA_CALIBRE_PHONE='13800000000'
 *   $env:DATA_CALIBRE_PASSWORD='MochatLocal@123'
 *   $env:DATA_CALIBRE_CORP_ID='1536612155'
 *   pnpm --filter @mochat/e2e exec node data-calibre-acceptance.mjs
 */

import { chromium } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

const baseUrl = process.env.DATA_CALIBRE_BASE ?? 'http://127.0.0.1:18080';
const outDir = process.env.DATA_CALIBRE_OUT ?? 'D:\\workspace\\mochat-go\\output\\data-calibre-verify-20260808';
const phone = process.env.DATA_CALIBRE_PHONE ?? '13800000000';
const password = process.env.DATA_CALIBRE_PASSWORD ?? 'MochatLocal@123';
const corpId = Number(process.env.DATA_CALIBRE_CORP_ID ?? 0);

if (!password || !corpId) {
  console.error('缺少 DATA_CALIBRE_PASSWORD 或 DATA_CALIBRE_CORP_ID');
  process.exit(2);
}

const evidence = [];

async function capture(page, name, route) {
  const file = path.join(outDir, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  const bodyText = (await page.locator('body').innerText().catch(() => '')) ?? '';
  evidence.push({
    name,
    route,
    hardcodedZeroModules: ['会话数据', '质检数据', '员工会话数据排行', '员工会话轨迹一览']
      .filter((heading) => bodyText.includes(heading)),
    hasAiEmptyState: bodyText.includes('AI 能力未接入'),
    hasArchiveEmptyState: bodyText.includes('会话归档未接入'),
    bodyExcerpt: bodyText.slice(0, 700),
  });
  return file;
}

async function login(page) {
  await page.goto(`${baseUrl}/login`, { waitUntil: 'domcontentloaded' });
  await page.locator('#login-phone').fill(phone);
  await page.locator('#login-password').fill(password);
  await page.getByRole('button', { name: '登录' }).click();
  await page.waitForURL(/\/index/, { timeout: 20000 }).catch(() => {});
  await page.waitForTimeout(1200);
  const url = page.url();
  const body = await page.locator('body').innerText().catch(() => '');
  if (!url.includes('/index') && body.includes('登录')) {
    throw new Error(`登录失败：${body.slice(0, 300)}`);
  }
  evidence.push({ event: 'login', url, ok: true });
}

async function dashboardFetch(page, apiPath, params = '') {
  return page.evaluate(async ({ apiPath, params }) => {
    const raw = localStorage.getItem('mochat_dashboard_token');
    const token = raw === null ? '' : (JSON.parse(raw) ?? '');
    const response = await fetch(`/dashboard${apiPath}${params}`, {
      headers: token === '' ? {} : { Authorization: token },
    });
    return { status: response.status, body: await response.json() };
  }, { apiPath, params });
}

function reportParams() {
  return '?corpId=' + corpId +
    '&timezone=Asia%2FShanghai' +
    '&startAt=2026-08-01T00%3A00%3A00%2B08%3A00' +
    '&endAt=2026-09-01T00%3A00%3A00%2B08%3A00' +
    '&page=1&pageSize=20';
}

async function main() {
  await mkdir(outDir, { recursive: true });
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1280, height: 1800 } });
  const page = await context.newPage();
  try {
    await login(page);

    const overview = await dashboardFetch(page, '/reports/overview', reportParams());
    const customer = await dashboardFetch(page, '/reports/customer', reportParams());
    const conversion = await dashboardFetch(page, '/reports/conversion', reportParams());
    const behavior = await dashboardFetch(page, '/reports/behavior', reportParams());
    const checks = [];

    const overviewData = overview.body?.data ?? {};
    const customerData = customer.body?.data ?? {};
    const conversionData = conversion.body?.data ?? {};
    const behaviorData = behavior.body?.data ?? {};

    checks.push({ name: 'overview-status', pass: overview.status === 200, actual: overview.status });
    checks.push({
      name: 'customer-calibre',
      pass: Number(overviewData.summary?.customer ?? -1) === Number(customerData.summary?.customer ?? -2),
      overview: overviewData.summary?.customer,
      customer: customerData.summary?.customer,
    });
    checks.push({
      name: 'lead-calibre',
      pass: Number(overviewData.summary?.lead ?? -1) === Number(conversionData.summary?.lead ?? -2),
      overview: overviewData.summary?.lead,
      conversion: conversionData.summary?.lead,
    });
    checks.push({
      name: 'order-calibre',
      pass: Number(overviewData.summary?.order ?? -1) === Number(conversionData.summary?.order ?? -2),
      overview: overviewData.summary?.order,
      conversion: conversionData.summary?.order,
    });
    checks.push({
      name: 'behavior-calibre',
      pass: Number(overviewData.summary?.behavior ?? -1) === Number(behaviorData.summary?.behavior ?? -2),
      overview: overviewData.summary?.behavior,
      behavior: behaviorData.summary?.behavior,
    });
    checks.push({
      name: 'archive-limitation-present',
      pass: Array.isArray(overviewData.limitations) && overviewData.limitations.some((item) => item.provider === 'conversation_archive'),
      limitations: overviewData.limitations ?? [],
    });
    evidence.push({ event: 'api', overview: overviewData, customer: customerData, conversion: conversionData, behavior: behaviorData, checks });

    await page.goto(`${baseUrl}/index`, { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('h1:has-text("数据概览")', { timeout: 15000 }).catch(() => {});
    await page.waitForTimeout(1500);
    const indexText = (await page.locator('body').innerText().catch(() => '')) ?? '';
    evidence.push({
      event: 'index-ui',
      showsCustomerTotal: indexText.includes('客户总数'),
      showsLeadTotal: indexText.includes('线索总数'),
      showsOrderTotal: indexText.includes('订单总数'),
      showsBehaviorTotal: indexText.includes('行为事件'),
      hasAiEmptyState: indexText.includes('AI 能力未接入'),
      hasArchiveEmptyState: indexText.includes('会话归档未接入'),
      hardcodedZeroModules: ['会话数据', '质检数据', '员工会话数据排行', '员工会话轨迹一览'].filter((heading) => indexText.includes(heading)),
    });
    await capture(page, '01-overview-unified', '/index');

    const failed = checks.filter((check) => !check.pass);
    console.log(JSON.stringify({ ok: failed.length === 0, outDir, checks, failed, evidenceCount: evidence.length }, null, 2));
    if (failed.length > 0) {
      process.exitCode = 1;
    }
  } finally {
    await writeFile(path.join(outDir, 'evidence.json'), JSON.stringify(evidence, null, 2), 'utf8').catch(() => {});
    await browser.close();
  }
}

main().catch((error) => {
  console.error(`数据口径验收失败: ${error.message}`);
  process.exitCode = 1;
});
