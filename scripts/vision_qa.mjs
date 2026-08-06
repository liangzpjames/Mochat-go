#!/usr/bin/env node
/**
 * vision_qa.mjs — 把本地截图发给视觉模型 API，返回文本分析。
 *
 * 用途：为不具备原生识图能力的代理提供“眼睛”。
 * 典型链路：浏览器截图 -> 本脚本调用视觉模型 -> 返回结构化文本 -> 代理据此做验收判断。
 *
 * 配置优先级：环境变量 > 配置文件。
 *   环境变量：VISION_API_BASE_URL / VISION_API_KEY / VISION_MODEL
 *   配置文件：JSON，路径用 --config 指定，或放在
 *     D:\workspace\mochat-go\output\vision-qa.config.json（仓库外，不上库）
 *   配置示例：
 *     { "baseUrl": "https://api.example.com/v1", "apiKey": "sk-xxx", "model": "qwen-vl-max" }
 *
 * 用法：
 *   node scripts/vision_qa.mjs shot.png
 *   node scripts/vision_qa.mjs --prompt "请检查订单页是否有报错" shot.png
 *   node scripts/vision_qa.mjs --system "你是中文验收助手" shot.png
 *   node scripts/vision_qa.mjs --check-config
 *
 * 兼容性：默认 OpenAI /chat/completions 协议（image_url + base64 data URI），
 * 覆盖大多数提供视觉模型的国产 API；若目标服务协议特殊（Azure/Anthropic），按需改 sendRequest。
 */

import { readFile } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import path from 'node:path';

const DEFAULT_SYSTEM_PROMPT =
  '你是一个严格的 UI 缺陷审查员。禁止任何夸奖、正面评价或“整体不错”之类的结论。请逐条列出截图中的所有问题与缺陷，包括但不限于：布局错位、元素未对齐、内容偏左/偏右/居中异常、间距不一致、文字重叠或截断、重复文案、空白异常、颜色/对比度问题、数据异常（如应为空却显示 0、应为有值却显示占位符）、导航或菜单高亮与页面不符、格式不统一。每条问题说明位置与原因；若确实没有问题，只回答“未发现问题”，不要加任何赞美。只描述截图中的事实，不要推测或编造。';

const DEFAULT_PROMPT = '请逐项描述这张截图，并指出任何异常。';

const CONFIG_CANDIDATES = [
  process.env.VISION_QA_CONFIG,
  'D:\\workspace\\mochat-go\\output\\vision-qa.config.json',
];

function usage() {
  console.log(`用法:
  node scripts/vision_qa.mjs [options] <image...>

选项:
  --prompt <text>    对图片的提问（默认："${DEFAULT_PROMPT}"）
  --system <text>    系统提示词（默认内置中文验收助手）
  --config <path>    配置文件路径（默认找 D:\\workspace\\mochat-go\\output\\vision-qa.config.json）
  --check-config     只校验配置是否齐全，不调用 API
  --retries <n>      失败重试次数（默认 3）

环境变量: VISION_API_BASE_URL / VISION_API_KEY / VISION_MODEL`);
}

async function loadConfig() {
  let fromFile = {};
  for (const candidate of CONFIG_CANDIDATES) {
    if (candidate && existsSync(candidate)) {
      try {
        fromFile = JSON.parse(await readFile(candidate, 'utf8'));
      } catch (error) {
        throw new Error(`配置文件解析失败 ${candidate}: ${error.message}`);
      }
      break;
    }
  }
  const fromEnv = {
    baseUrl: process.env.VISION_API_BASE_URL,
    apiKey: process.env.VISION_API_KEY,
    model: process.env.VISION_MODEL,
  };
  const merged = { ...fromFile, ...Object.fromEntries(Object.entries(fromEnv).filter(([, value]) => value)) };
  const missing = ['baseUrl', 'apiKey', 'model'].filter((key) => !merged[key]);
  if (missing.length) {
    throw new Error(
      `视觉模型配置缺失: ${missing.join(', ')}。请设置环境变量 VISION_API_BASE_URL/VISION_API_KEY/VISION_MODEL，` +
        '或创建 D:\\workspace\\mochat-go\\output\\vision-qa.config.json（仓库外）。',
    );
  }
  return merged;
}

function parseArgs(argv) {
  const options = { prompt: DEFAULT_PROMPT, system: DEFAULT_SYSTEM_PROMPT, retries: 3, images: [] };
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === '--help' || arg === '-h') {
      usage();
      process.exit(0);
    }
    if (arg === '--check-config') {
      options.checkConfig = true;
      continue;
    }
    if (arg === '--prompt' || arg === '--system' || arg === '--config' || arg === '--retries') {
      const value = argv[index + 1];
      if (value === undefined) throw new Error(`${arg} 需要一个值`);
      if (arg === '--prompt') options.prompt = value;
      if (arg === '--system') options.system = value;
      if (arg === '--config') CONFIG_CANDIDATES.unshift(value);
      if (arg === '--retries') options.retries = Number(value);
      index += 1;
      continue;
    }
    options.images.push(arg);
  }
  return options;
}

async function callVision(config, imagePath, prompt, system, retries) {
  const body = await readFile(imagePath);
  const extension = (path.extname(imagePath).slice(1) || 'png').toLowerCase();
  const mime = extension === 'jpg' ? 'jpeg' : extension;
  const base64 = body.toString('base64');
  const endpoint = `${config.baseUrl.replace(/\/+$/, '')}/chat/completions`;
  const payload = {
    model: config.model,
    messages: [
      ...(system ? [{ role: 'system', content: system }] : []),
      {
        role: 'user',
        content: [
          { type: 'text', text: prompt },
          { type: 'image_url', image_url: { url: `data:image/${mime};base64,${base64}` } },
        ],
      },
    ],
    temperature: config.temperature ?? 0.2,
    max_tokens: config.maxTokens ?? 1200,
  };
  const timeoutMs = config.timeoutMs ?? 120000;

  for (let attempt = 1; attempt <= retries; attempt += 1) {
    let response;
    try {
      response = await fetch(endpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${config.apiKey}`,
        },
        body: JSON.stringify(payload),
        signal: AbortSignal.timeout(timeoutMs),
      });
    } catch (error) {
      if (attempt < retries) {
        const waitMs = attempt * 2000;
        process.stderr.write(`网络请求失败(${error.message})，${waitMs}ms 后重试…\n`);
        await new Promise((resolve) => setTimeout(resolve, waitMs));
        continue;
      }
      throw new Error(`视觉模型网络请求失败: ${error.message}`);
    }
    if (response.ok) {
      const data = await response.json();
      const content = data?.choices?.[0]?.message?.content;
      if (content) return content;
      return JSON.stringify(data);
    }
    const detail = (await response.text()).slice(0, 500);
    if (response.status === 401 || response.status === 403) {
      throw new Error(`视觉模型鉴权失败 (${response.status}): ${detail}`);
    }
    if ((response.status === 429 || response.status >= 500) && attempt < retries) {
      const waitMs = attempt * 2000;
      process.stderr.write(`请求失败(${response.status})，${waitMs}ms 后重试…\n`);
      await new Promise((resolve) => setTimeout(resolve, waitMs));
      continue;
    }
    throw new Error(`视觉模型请求失败 (${response.status}): ${detail}`);
  }
  throw new Error('视觉模型请求失败：重试次数用尽');
}

async function main() {
  const options = parseArgs(process.argv.slice(2));
  if (options.checkConfig) {
    const config = await loadConfig();
    console.log(`配置检查通过：baseUrl=${config.baseUrl} model=${config.model} apiKey=${config.apiKey ? '已设置(长度 ' + config.apiKey.length + ')' : '未设置'}`);
    return;
  }
  if (options.images.length === 0) {
    usage();
    process.exitCode = 1;
    return;
  }
  const config = await loadConfig();
  for (const image of options.images) {
    if (!existsSync(image)) throw new Error(`图片不存在: ${image}`);
    const result = await callVision(config, image, options.prompt, options.system, options.retries);
    process.stdout.write(`===== ${image} =====\n${result}\n\n`);
  }
}

main().catch((error) => {
  console.error(`vision_qa 失败: ${error.message}`);
  process.exitCode = 1;
});
