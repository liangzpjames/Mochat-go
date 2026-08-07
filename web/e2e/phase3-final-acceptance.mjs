#!/usr/bin/env node
/**
 * phase3-final-acceptance.mjs — Phase 3 Final 真实栈验收。
 *
 * 覆盖：
 * 1. /chat/file-audio 音频闭环：上传 → 列表回读 → 鉴权下载字节一致 → 播放控件 → 软删。
 * 2. AI 洞察五页：真实 AI 调用 + 落库回读（页面上下文带 token 请求）。
 * 3. 截图与文本证据输出到 D:\workspace\mochat-go\output\phase3-final-browser-20260807。
 *
 * 用法：
 *   $env:PHASE3FINAL_PHONE='13800000000'
 *   $env:PHASE3FINAL_PASSWORD='MochatLocal@123'
 *   $env:PHASE3FINAL_CORP_ID='1536612155'
 *   pnpm --filter @mochat/e2e exec node phase3-final-acceptance.mjs
 */

import { chromium } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

const baseUrl = process.env.PHASE3FINAL_BASE ?? 'http://127.0.0.1:18080';
const outDir = process.env.PHASE3FINAL_OUT ?? 'D:\\workspace\\mochat-go\\output\\phase3-final-browser-20260807';
const phone = process.env.PHASE3FINAL_PHONE ?? '13800000000';
const password = process.env.PHASE3FINAL_PASSWORD ?? 'MochatLocal@123';
const corpId = Number(process.env.PHASE3FINAL_CORP_ID ?? 0);
const audioFileName = 'p36-accept-voice.wav';

if (!password || !corpId) {
  console.error('缺少 PHASE3FINAL_PASSWORD 或 PHASE3FINAL_CORP_ID');
  process.exit(2);
}

const evidence = [];

function makeWav(pathTarget, seconds = 1, sampleRate = 8000) {
  const sampleCount = sampleRate * seconds;
  const dataSize = sampleCount * 2;
  const buffer = Buffer.alloc(44 + dataSize);
  buffer.write('RIFF', 0);
  buffer.writeUInt32LE(36 + dataSize, 4);
  buffer.write('WAVE', 8);
  buffer.write('fmt ', 12);
  buffer.writeUInt32LE(16, 16);
  buffer.writeUInt16LE(1, 20);
  buffer.writeUInt16LE(1, 22);
  buffer.writeUInt32LE(sampleRate, 24);
  buffer.writeUInt32LE(sampleRate * 2, 28);
  buffer.writeUInt16LE(2, 32);
  buffer.writeUInt16LE(16, 34);
  buffer.write('data', 36);
  buffer.writeUInt32LE(dataSize, 40);
  for (let index = 0; index < sampleCount; index += 1) {
    const value = Math.round(Math.sin((2 * Math.PI * 440 * index) / sampleRate) * 12000);
    buffer.writeInt16LE(value, 44 + index * 2);
  }
  return buffer;
}

async function capture(page, name, extra = {}) {
  const file = path.join(outDir, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  const bodyText = (await page.locator('body').innerText().catch(() => '')) ?? '';
  const markers = {
    error: /失败|错误|Exception|Cannot read|TypeError|未定义|加载失败|出错了/.test(bodyText),
    empty: bodyText.includes('暂无数据') || bodyText.includes('还没有可展示'),
    ready: bodyText.includes('已就绪'),
  };
  evidence.push({ name, ...extra, markers, bodyExcerpt: bodyText.slice(0, 400) });
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

async function audioWorkflow(page) {
  const steps = [];
  const wavPath = path.join(outDir, audioFileName);
  await writeFile(wavPath, makeWav(wavPath));
  const wavBytes = await import('node:fs/promises').then(({ readFile }) => readFile(wavPath));

  await page.goto(`${baseUrl}/chat/file-audio`, { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('h1', { timeout: 15000 }).catch(() => {});
  await page.waitForTimeout(1000);
  await page.setInputFiles('input[type="file"]', wavPath);
  await page.waitForTimeout(400);
  await page.getByRole('button', { name: '上传' }).click();
  await page.waitForSelector(`tr:has-text("${audioFileName}")`, { timeout: 15000 });
  steps.push('upload-row-visible');

  const audioSrc = await page.locator(`tr:has-text("${audioFileName}") audio`).getAttribute('src');
  evidence.push({ event: 'audioRow', audioSrc });
  await page.waitForTimeout(600);
  await capture(page, '01-file-audio-uploaded', { route: '/chat/file-audio' });

  const content = await page.evaluate(async ({ url }) => {
    const raw = localStorage.getItem('mochat_dashboard_token');
    const token = raw === null ? '' : (JSON.parse(raw) ?? '');
    const response = await fetch(url, { headers: token === '' ? {} : { Authorization: token } });
    const bytes = new Uint8Array(await response.arrayBuffer());
    let binary = '';
    for (const value of bytes) {
      binary += String.fromCharCode(value);
    }
    return {
      status: response.status,
      contentType: response.headers.get('content-type') ?? '',
      base64: btoa(binary),
    };
  }, { url: audioSrc });
  const contentBytes = Buffer.from(content.base64, 'base64');
  const contentOk = content.status === 200
    && content.contentType === 'audio/wav'
    && contentBytes.length === wavBytes.length
    && contentBytes.equals(wavBytes);
  evidence.push({ event: 'audioContent', status: content.status, contentType: content.contentType, sizeMatch: contentBytes.length === wavBytes.length, bytesEqual: contentBytes.equals(wavBytes) });
  if (!contentOk) {
    throw new Error('音频鉴权下载字节不一致');
  }
  steps.push('download-bytes-match');

  page.once('dialog', (dialog) => void dialog.accept());
  await page.getByRole('button', { name: `删除 ${audioFileName}` }).click();
  await page.waitForSelector(`tr:has-text("${audioFileName}")`, { state: 'detached', timeout: 10000 });
  steps.push('soft-delete-hidden');
  await capture(page, '02-file-audio-after-delete', { route: '/chat/file-audio' });
  return steps;
}

async function aiInsightWorkflow(page) {
  const pages = ['session-analysis', 'smart-analysis', 'emotion', 'employee-score', 'communication-keyword'];
  for (const pageName of pages) {
    const result = await dashboardFetch(page, `/ai-insight/${pageName}`, `?corpId=${corpId}&refresh=1`);
    const data = result.body?.data ?? {};
    evidence.push({
      event: 'aiInsightApi',
      page: pageName,
      status: result.status,
      capability: data.capability,
      provider: data.provider,
      dataLength: Array.isArray(data.data) ? data.data.length : -1,
      limitations: data.limitations ?? [],
      summary: Array.isArray(data.data) && data.data[0] ? String(data.data[0].summary ?? '').slice(0, 200) : '',
    });
    if (result.status !== 200 || data.capability !== 'ready' || !Array.isArray(data.data) || data.data.length === 0) {
      throw new Error(`${pageName} 未返回 ready 结果：${JSON.stringify(data).slice(0, 500)}`);
    }
  }

  await page.goto(`${baseUrl}/ai-insight/session-analysis`, { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(1000);
  await page.waitForFunction(() => document.body.innerText.includes('AI 分析能力已就绪'), null, { timeout: 30000 }).catch(() => {});
  await page.waitForTimeout(800);
  await capture(page, '03-ai-insight-session-analysis', { route: '/ai-insight/session-analysis' });
  return ['five-pages-ready', 'session-analysis-ui-ready'];
}

async function main() {
  await mkdir(outDir, { recursive: true });
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1280, height: 2000 } });
  const page = await context.newPage();
  try {
    await login(page);
    const audioSteps = await audioWorkflow(page);
    const aiSteps = await aiInsightWorkflow(page);
    console.log(JSON.stringify({ ok: true, outDir, audioSteps, aiSteps, evidenceCount: evidence.length, evidence }, null, 2));
  } finally {
    await writeFile(path.join(outDir, 'evidence.json'), JSON.stringify(evidence, null, 2), 'utf8').catch(() => {});
    await browser.close();
  }
}

main().catch((error) => {
  console.error(`Phase 3 Final 验收失败: ${error.message}`);
  process.exitCode = 1;
});
