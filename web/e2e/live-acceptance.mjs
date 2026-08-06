#!/usr/bin/env node
/**
 * live-acceptance.mjs — 对真实 mochat-go-desktop 栈做 Phase 3.5 浏览器验收。
 *
 * 用法：
 *   $env:PHASE35_ACCEPT_PHONE='18635080635'
 *   $env:PHASE35_ACCEPT_PASSWORD='...'
 *   pnpm --filter @mochat/e2e exec node live-acceptance.mjs
 *
 * 输出：截图到 D:\workspace\mochat-go\output\phase35-browser-20260806\，
 * 页面文本证据写入同目录 evidence.json。
 */

import { chromium } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

const baseUrl = process.env.PHASE35_ACCEPT_BASE ?? 'http://127.0.0.1:18080';
const outDir = process.env.PHASE35_ACCEPT_OUT ?? 'D:\\workspace\\mochat-go\\output\\phase35-browser-20260806';
const phone = process.env.PHASE35_ACCEPT_PHONE ?? '18635080635';
const password = process.env.PHASE35_ACCEPT_PASSWORD;

if (!password) {
  console.error('缺少环境变量 PHASE35_ACCEPT_PASSWORD');
  process.exit(2);
}

const pages = [
  { name: '01-friends', route: '/customer/friends' },
  { name: '02-group', route: '/customer/group' },
  { name: '03-order', route: '/customer/order' },
  { name: '04-settings', route: '/customer/settings' },
  { name: '05-customer-report', route: '/data/customer' },
  { name: '06-employee-report', route: '/data/employee' },
  { name: '07-conversion-report', route: '/data/conversion' },
  { name: '08-behavior-report', route: '/data/behavior' },
  { name: '09-summary-report', route: '/data/report' },
];

const evidence = [];

async function capture(page, name, description) {
  await page.setViewportSize({ width: 1280, height: 2000 });
  const file = path.join(outDir, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  const h1 = await page.locator('h1').first().textContent().catch(() => '');
  const bodyText = (await page.locator('body').innerText().catch(() => '')) ?? '';
  const activeMenu = await page.evaluate(() => document.querySelector('.dashboard-menu-link-active')?.textContent?.trim() ?? '').catch(() => '');
  const markers = {
    error: /失败|错误|Exception|Cannot read|TypeError|未定义|加载失败|出错了/.test(bodyText),
    empty: bodyText.includes('暂无数据'),
    limited: bodyText.includes('数据限制') || bodyText.includes('不可用'),
    loading: bodyText.includes('正在加载'),
  };
  evidence.push({ name, route: description?.route, h1: h1?.trim(), activeMenu, markers, bodyExcerpt: bodyText.slice(0, 500) });
  return file;
}

async function login(page) {
  await page.goto(`${baseUrl}/login`, { waitUntil: 'domcontentloaded' });
  await page.locator('#login-phone').fill(phone);
  await page.locator('#login-password').fill(password);
  await page.getByRole('button', { name: '登录' }).click();
  await page.waitForURL(/\/index/, { timeout: 20000 }).catch(() => {});
  await page.waitForTimeout(1200);
  const body = await page.locator('body').innerText().catch(() => '');
  if (body.includes('登录') && !body.includes('数据概览') && !(await page.url()).includes('/index')) {
    throw new Error(`登录失败：${body.slice(0, 300)}`);
  }
  evidence.push({ event: 'login', url: page.url(), ok: true });
}

async function orderWorkflow(page) {
  const steps = [];
  await page.goto(`${baseUrl}/customer/order`, { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('h1', { timeout: 15000 }).catch(() => {});
  await page.waitForTimeout(1200);

  const existingRow = page.locator('tr', { hasText: 'P35-ACCEPT-验收订单' }).first();
  if ((await existingRow.count()) > 0) {
    const statusCell = existingRow.locator('td').nth(4);
    const statusText = await statusCell.innerText().catch(() => '');
    await existingRow.getByRole('button', { name: '查看详情' }).click();
    await page.waitForTimeout(1200);
    if (statusText.includes('待支付')) {
      const transition = page.getByRole('button', { name: '变更为已支付' }).first();
      if (await transition.isVisible().catch(() => false)) {
        await transition.click();
        await page.waitForTimeout(1500);
        steps.push('reused-order-transitioned-to-paid');
      }
    } else {
      steps.push(`reused-order-status=${statusText}`);
    }
    await capture(page, '11-order-detail', { route: '/customer/order' });
    return steps;
  }

  const quickCreate = page.getByRole('button', { name: '创建并选中' });
  const contactSelect = page.getByRole('combobox', { name: '联系人' });
  const optionCount = await contactSelect.locator('option').count().catch(() => 0);
  if (optionCount > 1) {
    await contactSelect.selectOption({ index: 1 });
    steps.push(`contact-selected-option-${optionCount}`);
  } else if (await quickCreate.isVisible().catch(() => false)) {
    await page.getByRole('textbox', { name: '快速联系人姓名' }).fill('P35-ACCEPT-验收联系人');
    await page.getByRole('textbox', { name: '快速联系人手机' }).fill('13900001234');
    await quickCreate.click();
    await page.waitForTimeout(1500);
    steps.push('quick-contact-created');
  }

  await page.getByRole('textbox', { name: '订单标题' }).fill('P35-ACCEPT-验收订单');
  await page.getByRole('textbox', { name: '金额（元）' }).fill('128');
  await page.getByRole('textbox', { name: '备注' }).fill('浏览器验收创建');
  const submit = page.getByRole('button', { name: '创建订单' });
  await submit.waitFor({ state: 'visible', timeout: 5000 }).catch(() => {});
  await page.getByRole('button', { name: '创建订单' }).click();
  await page.waitForTimeout(2000);
  await capture(page, '10-order-created', { route: '/customer/order' });

  const row = page.locator('tr', { hasText: 'P35-ACCEPT-验收订单' });
  const created = (await row.count()) > 0;
  steps.push(`order-row-visible=${created}`);
  if (created) {
    await row.getByRole('button', { name: '查看详情' }).first().click();
    await page.waitForTimeout(1200);
    await capture(page, '11-order-detail', { route: '/customer/order' });
    const transition = page.getByRole('button', { name: /变更为/ }).first();
    if (await transition.isVisible().catch(() => false)) {
      await transition.click();
      await page.waitForTimeout(1500);
      await capture(page, '12-order-transition', { route: '/customer/order' });
      steps.push('transition-clicked');
    }
  }
  return steps;
}

async function main() {
  await mkdir(outDir, { recursive: true });
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1280, height: 720 } });
  const page = await context.newPage();
  try {
    await login(page);

    for (const item of pages) {
      await page.goto(`${baseUrl}${item.route}`, { waitUntil: 'domcontentloaded' });
      await page.waitForSelector('h1', { timeout: 15000 }).catch(() => {});
      await page.waitForTimeout(1500);
      await capture(page, item.name, { route: item.route });
    }

    const workflowSteps = await orderWorkflow(page);
    evidence.push({ event: 'orderWorkflow', steps: workflowSteps });

  await page.goto(`${baseUrl}/data/conversion`, { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(1500);
  await capture(page, '13-conversion-after-order', { route: '/data/conversion' });

  await page.goto(`${baseUrl}/data/report`, { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(1500);
  await capture(page, '14-summary-after-order', { route: '/data/report' });

    console.log(JSON.stringify({ ok: true, outDir, pages: pages.length, evidence }, null, 2));
  } finally {
    await writeFile(path.join(outDir, 'evidence.json'), JSON.stringify(evidence, null, 2), 'utf8').catch(() => {});
    await browser.close();
  }
}

main().catch((error) => {
  console.error(`验收脚本失败: ${error.message}`);
  process.exitCode = 1;
});
