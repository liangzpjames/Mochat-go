#!/usr/bin/env node
/**
 * debt-acceptance.mjs — Phase 3 坏账清理浏览器验收（26 页）。
 *
 * 用法：
 *   $env:DEBT_ACCEPT_PHONE='13800000000'
 *   $env:DEBT_ACCEPT_PASSWORD='...'
 *   pnpm --filter @mochat/e2e exec node debt-acceptance.mjs
 *
 * 输出：截图到 D:\workspace\mochat-go\output\debt-clearance-browser\2026-08-07\，
 * 页面文本证据写入同目录 evidence.json。
 */

import { chromium } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

const baseUrl = process.env.DEBT_ACCEPT_BASE ?? 'http://127.0.0.1:18080';
const outDir = process.env.DEBT_ACCEPT_OUT ?? 'D:\\workspace\\mochat-go\\output\\debt-clearance-browser\\2026-08-07';
const phone = process.env.DEBT_ACCEPT_PHONE ?? '13800000000';
const password = process.env.DEBT_ACCEPT_PASSWORD;

if (!password) {
  console.error('缺少环境变量 DEBT_ACCEPT_PASSWORD');
  process.exit(2);
}

const pages = [
  // 会话（8）
  { name: '01-chat-v2-staff', route: '/chat/v2-staff' },
  { name: '02-chat-v2-customer', route: '/chat/v2-customer' },
  { name: '03-chat-v2-group', route: '/chat/v2-group' },
  { name: '04-chat-trajectory', route: '/chat/trajectory' },
  { name: '05-chat-export', route: '/chat/export' },
  { name: '06-chat-resign-staff', route: '/chat/resign-staff' },
  { name: '07-chat-refuse-archive', route: '/chat/refuse-archive' },
  { name: '08-customer-inheritance', route: '/customer/inheritance' },
  // 风险预警（6）
  { name: '09-risk-behavior', route: '/ai-insight/v2/risk' },
  { name: '10-risk-timeout', route: '/ai-insight/v2/timeout' },
  { name: '11-risk-customer-loss', route: '/ai-insight/v2/customer-loss' },
  { name: '12-risk-message-intercept', route: '/ai-insight/v2/message-intercept' },
  { name: '13-risk-keyword-library', route: '/ai-insight/v2/keyword-library' },
  { name: '14-risk-silent-customer', route: '/ai-insight/v2/silent-customer' },
  // AI 洞察（5）
  { name: '15-ai-session-analysis', route: '/ai-insight/session-analysis' },
  { name: '16-ai-smart-analysis', route: '/ai-insight/smart-analysis' },
  { name: '17-ai-emotion', route: '/ai-insight/emotion' },
  { name: '18-ai-employee-score', route: '/ai-insight/employee-score' },
  { name: '19-ai-communication-keyword', route: '/ai-insight/communication-keyword' },
  // AI 设置（2）
  { name: '20-ai-knowledge-base', route: '/ai-setting/ai-knowledge-base' },
  { name: '21-ai-agent', route: '/ai-setting/agent' },
  // 企业设置（5）
  { name: '22-company-website', route: '/company-setting/website' },
  { name: '23-company-staff', route: '/company-setting/staff' },
  { name: '24-setting-role', route: '/setting/role' },
  { name: '25-setting-additional', route: '/setting/additional' },
  { name: '26-setting-authorization', route: '/setting/authorization' },
];

const evidence = [];

async function capture(page, name, route) {
  await page.setViewportSize({ width: 1280, height: 2000 });
  await page.goto(`${baseUrl}${route}`, { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('h1', { timeout: 15000 }).catch(() => {});
  await page.waitForTimeout(1500);
  const file = path.join(outDir, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  const h1 = await page.locator('h1').first().textContent().catch(() => '');
  const bodyText = (await page.locator('body').innerText().catch(() => '')) ?? '';
  const activeMenu = await page.evaluate(() => document.querySelector('.dashboard-menu-link-active')?.textContent?.trim() ?? '').catch(() => '');
  const markers = {
    error: /失败|错误|Exception|Cannot read|TypeError|未定义|加载失败|出错了|500/.test(bodyText),
    empty: bodyText.includes('暂无数据') || bodyText.includes('暂无'),
    limited: bodyText.includes('数据限制') || bodyText.includes('不可用') || bodyText.includes('未接入'),
    loading: bodyText.includes('正在加载'),
    placeholder: bodyText.includes('占位') || bodyText.includes('功能建设中') || bodyText.includes('演示数据'),
  };
  evidence.push({ name, route, h1: h1?.trim(), activeMenu, markers, bodyExcerpt: bodyText.slice(0, 600) });
  console.log(`${name}\t${route}\th1=${h1?.trim()}\tactive=${activeMenu}\tmarkers=${JSON.stringify(markers)}`);
  return file;
}

async function login(page) {
  await page.goto(`${baseUrl}/login`, { waitUntil: 'domcontentloaded' });
  await page.locator('#login-phone').fill(phone);
  await page.locator('#login-password').fill(password);
  await page.getByRole('button', { name: '登录' }).click();
  await page.waitForURL(/\/index/, { timeout: 20000 }).catch(() => {});
  await page.waitForTimeout(1500);
  const body = await page.locator('body').innerText().catch(() => '');
  if (body.includes('登录') && !body.includes('数据概览') && !(await page.url()).includes('/index')) {
    throw new Error(`登录失败：${body.slice(0, 300)}`);
  }
  evidence.push({ event: 'login', url: page.url(), ok: true });
}

async function main() {
  await mkdir(outDir, { recursive: true });
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext();
  const page = await context.newPage();
  await login(page);
  for (const p of pages) {
    await capture(page, p.name, p.route).catch((e) => {
      evidence.push({ name: p.name, route: p.route, captureError: String(e) });
      console.error(`${p.name} capture failed: ${e.message}`);
    });
  }
  await writeFile(path.join(outDir, 'evidence.json'), JSON.stringify(evidence, null, 2), 'utf8');
  await browser.close();
  const failed = evidence.filter((e) => e.markers?.error || e.captureError);
  const placeholders = evidence.filter((e) => e.markers?.placeholder);
  console.log(`captured=${pages.length} evidenceError=${failed.length} placeholder=${placeholders.length}`);
  if (failed.length) {
    console.error('error markers:', failed.map((f) => f.route).join(', '));
  }
  if (placeholders.length) {
    console.warn('placeholder markers:', placeholders.map((f) => f.route).join(', '));
  }
}

main().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
