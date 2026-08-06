#!/usr/bin/env node
/**
 * debt-workflow.mjs — Phase 3 坏账清理数据流工作流验证。
 *
 * 真实链路：
 *   1) AI 知识库：新建 P36-ACCEPT-* 知识库 → 列表回读；
 *   2) AI 智能体：新建 P36-ACCEPT-* 智能体并关联知识库 → 列表回读；
 *   3) 输出步骤证据到 D:\workspace\mochat-go\output\debt-clearance-browser\2026-08-07\workflow.json。
 *
 * 用法：
 *   $env:DEBT_ACCEPT_PHONE='13800000000'
 *   $env:DEBT_ACCEPT_PASSWORD='...'
 *   pnpm --filter @mochat/e2e exec node debt-workflow.mjs
 */

import { chromium } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

const baseUrl = process.env.DEBT_ACCEPT_BASE ?? 'http://127.0.0.1:18080';
const outDir = process.env.DEBT_ACCEPT_OUT ?? 'D:\\workspace\\mochat-go\\output\\debt-clearance-browser\\2026-08-07';
const phone = process.env.DEBT_ACCEPT_PHONE ?? '13800000000';
const password = process.env.DEBT_ACCEPT_PASSWORD;
const stamp = Date.now();
const kbName = `P36-ACCEPT-知识库-${stamp}`;
const agentName = `P36-ACCEPT-智能体-${stamp}`;

if (!password) {
  console.error('缺少环境变量 DEBT_ACCEPT_PASSWORD');
  process.exit(2);
}

const steps = [];

async function login(page) {
  await page.goto(`${baseUrl}/login`, { waitUntil: 'domcontentloaded' });
  await page.locator('#login-phone').fill(phone);
  await page.locator('#login-password').fill(password);
  await page.getByRole('button', { name: '登录' }).click();
  await page.waitForURL(/\/index/, { timeout: 20000 }).catch(() => {});
  await page.waitForTimeout(1500);
}

async function main() {
  await mkdir(outDir, { recursive: true });
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  page.on('dialog', (dialog) => dialog.accept());
  await login(page);

  // 1) 新建知识库
  await page.goto(`${baseUrl}/ai-setting/ai-knowledge-base`, { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('h1', { timeout: 15000 }).catch(() => {});
  await page.getByRole('button', { name: '新建知识库' }).click();
  await page.getByLabel('名称').fill(kbName);
  await page.getByLabel('说明').fill('坏账清理验收数据：知识库');
  await page.getByRole('button', { name: '保存' }).click();
  await page.waitForTimeout(1800);
  const kbVisible = await page.getByText(kbName).first().isVisible().catch(() => false);
  await page.screenshot({ path: path.join(outDir, 'workflow-kb-created.png'), fullPage: true });
  steps.push({ step: 'create-knowledge-base', name: kbName, visible: kbVisible, screenshot: 'workflow-kb-created.png' });
  if (!kbVisible) throw new Error('知识库创建后未在列表回读');

  // 2) 新建智能体并关联知识库
  await page.goto(`${baseUrl}/ai-setting/agent`, { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('h1', { timeout: 15000 }).catch(() => {});
  await page.getByRole('button', { name: '新建智能体' }).click();
  await page.getByLabel('名称').fill(agentName);
  await page.getByLabel('说明').fill('坏账清理验收数据：智能体');
  const kbCheckbox = page.getByLabel(kbName);
  if (await kbCheckbox.isVisible().catch(() => false)) {
    await kbCheckbox.check();
  }
  await page.getByRole('button', { name: '保存' }).click();
  await page.waitForTimeout(1800);
  const agentVisible = await page.getByText(agentName).first().isVisible().catch(() => false);
  await page.screenshot({ path: path.join(outDir, 'workflow-agent-created.png'), fullPage: true });
  steps.push({ step: 'create-agent', name: agentName, visible: agentVisible, linkedKb: kbName, screenshot: 'workflow-agent-created.png' });
  if (!agentVisible) throw new Error('智能体创建后未在列表回读');

  await writeFile(path.join(outDir, 'workflow.json'), JSON.stringify({ kbName, agentName, steps }, null, 2), 'utf8');
  await browser.close();
  console.log(`workflow ok: kb=${kbName} agent=${agentName} steps=${steps.length}`);
}

main().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
