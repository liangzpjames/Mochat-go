import {
  existsSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
} from 'node:fs';
import { spawnSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { extname, join, relative, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { inflateSync } from 'node:zlib';

const EXPECTED_ROUTE_COUNTS = {
  Sidebar: 12,
  Operation: 10,
};

const ROOT_SCRIPT = 'node scripts/check_mobile_clients_foundation.mjs && node --test scripts/check_mobile_clients_foundation.test.mjs';
const CAPTURE_SCRIPT = 'node scripts/capture_mobile_visual_evidence.mjs';
const VISUAL_EVIDENCE_FILES = [
  'sidebar-contact-390.png',
  'sidebar-workbench-390.png',
  'sidebar-pending-390.png',
  'operation-work-fission-390.png',
  'operation-pending-390.png',
  'sidebar-contact-1280.png',
  'operation-work-fission-1280.png',
];
const E2E_SCRIPT = 'playwright test tests/mobile-clients-foundation.spec.ts --workers=1';

const MOJIBAKE_PATTERN = /(?:锛|銆|鈥|鈮|馃|椤甸潰|妯″潡|浠诲姟|绔欏唴|涓嶅瓨鍦|瀹㈡埛|绉诲姩|鐢ㄦ埛|璇锋眰|璺敱)/g;
const FAKE_OUTCOME_PATTERNS = [
  /Math\.random\s*\(/g,
  /\b(?:fixedProgress|progress|percentage|percent)\s*(?:=|:)\s*\d+(?:\.\d+)?\b/g,
  /\b(?:participants?|participantCount|inviteCount|customerCount|rewardCount|prizeCount|businessMetric|fakeCount)\s*(?:=|:)\s*\d+(?:\.\d+)?\b/gi,
  /[>='"]\s*(?:已有|累计|今日|成功|完成)?\s*\d+(?:\.\d+)?\s*(?:人|位|个|条|元|%|次|份)/g,
  /(?:操作已完成|提交成功|保存成功|领取成功|随机奖品|模拟成功|虚假进度|固定进度)/g,
];

function readJSON(root, relativePath) {
  return JSON.parse(readFileSync(join(root, relativePath), 'utf8'));
}

function normalized(relativePath) {
  return relativePath.replaceAll('\\', '/');
}

function productionSources(root, sourceRoot) {
  const absoluteRoot = join(root, sourceRoot);
  const sources = [];
  const visit = (directory) => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      if (entry.isDirectory()) {
        if (/^(?:__tests__|tests?|fixtures?|dist|build|coverage|node_modules)$/i.test(entry.name)) continue;
        visit(join(directory, entry.name));
        continue;
      }
      if (!entry.isFile() || !['.css', '.ts', '.tsx'].includes(extname(entry.name))) continue;
      if (/(?:^|\.)(?:test|spec|stories)\.[cm]?[jt]sx?$/i.test(entry.name)) continue;
      const filePath = join(directory, entry.name);
      sources.push({
        path: normalized(relative(root, filePath)),
        source: readFileSync(filePath, 'utf8'),
      });
    }
  };
  visit(absoluteRoot);
  return sources;
}

function manifestPaths(manifest, label, errors) {
  if (!Array.isArray(manifest)) {
    errors.push(`${label} manifest must be an array`);
    return [];
  }
  const paths = [];
  for (const [index, route] of manifest.entries()) {
    if (typeof route !== 'object' || route === null || typeof route.path !== 'string' || !route.path.startsWith('/')) {
      errors.push(`${label} manifest route ${index} has no valid path`);
      continue;
    }
    if (route.target !== 'react') errors.push(`${label} manifest route ${route.path} must target react`);
    paths.push(route.path);
  }
  const unique = new Set(paths);
  if (unique.size !== paths.length) errors.push(`${label} manifest contains duplicate paths`);
  if (paths.length !== EXPECTED_ROUTE_COUNTS[label]) {
    errors.push(`${label} manifest must contain ${EXPECTED_ROUTE_COUNTS[label]} routes; found ${paths.length}`);
  }
  return paths;
}

function registryPaths(source, label, errors) {
  const declaration = /const\s+routeDefinitions\s*=\s*\{/.exec(source);
  if (declaration === null) {
    errors.push(`${label} registry must declare routeDefinitions independently`);
    return [];
  }
  const start = declaration.index + declaration[0].length;
  const end = source.indexOf('} as const', start);
  if (end === -1) {
    errors.push(`${label} registry routeDefinitions must be an as const object`);
    return [];
  }
  const body = source.slice(start, end);
  const paths = [...body.matchAll(/^\s*(['"])(\/[^'"]*)\1\s*:/gm)].map((match) => match[2]);
  if (new Set(paths).size !== paths.length) errors.push(`${label} registry contains duplicate paths`);
  return paths;
}

function compareRegistryToManifest(label, registry, manifest, errors) {
  const manifestSet = new Set(manifest);
  const registrySet = new Set(registry);
  for (const path of registry) {
    if (!manifestSet.has(path)) errors.push(`${label} registry route ${path} is absent from manifest`);
  }
  for (const path of manifest) {
    if (!registrySet.has(path)) errors.push(`${label} manifest route ${path} is absent from registry`);
  }
}

function specArrayBody(source, variable, errors) {
  const declaration = new RegExp(`const\\s+${variable}\\s*=\\s*\\[`).exec(source);
  if (declaration === null) {
    errors.push(`browser spec must declare ${variable}`);
    return '';
  }
  const start = declaration.index + declaration[0].length;
  const end = source.indexOf('] as const', start);
  if (end === -1) {
    errors.push(`browser spec ${variable} must be an as const array`);
    return '';
  }
  return source.slice(start, end);
}

function specCasePaths(source, variable, errors) {
  return [...specArrayBody(source, variable, errors).matchAll(/\bpath\s*:\s*(['"])(\/[^'"]*)\1/g)]
    .map((match) => match[2]);
}

function compareBrowserCases(label, cases, manifest, errors) {
  const expected = new Set(manifest);
  const actual = new Set(cases);
  if (actual.size !== cases.length) errors.push(`${label} browser cases contain duplicate paths`);
  for (const path of cases) {
    if (!expected.has(path)) errors.push(`${label} browser case ${path} is absent from manifest`);
  }
  for (const path of manifest) {
    if (!actual.has(path)) errors.push(`${label} manifest route ${path} has no browser case`);
  }
}

function validateUnknownRoute(label, source, errors) {
  const wildcard = source.match(/\{\s*path\s*:\s*['"]\*['"][\s\S]{0,240}?\}/);
  if (wildcard === null) {
    errors.push(`${label} router must register an unknown-route wildcard`);
    return;
  }
  if (/Navigate[\s\S]*?to\s*=\s*['"]\/['"]|(?:Home|routeDefinitions\[['"]\/['"]\])/i.test(wildcard[0])) {
    errors.push(`${label} unknown route falls back to home`);
    return;
  }
  if (!/NotFound/.test(wildcard[0])) errors.push(`${label} unknown route must render a NotFound page`);
}

function countMatches(source, pattern) {
  return [...source.matchAll(pattern)].length;
}

function braceBody(source, openingPattern) {
  const opening = openingPattern.exec(source);
  if (opening === null) return null;
  const openingBrace = source.indexOf('{', opening.index);
  if (openingBrace === -1) return null;
  let depth = 0;
  for (let index = openingBrace; index < source.length; index += 1) {
    if (source[index] === '{') depth += 1;
    if (source[index] === '}') {
      depth -= 1;
      if (depth === 0) return source.slice(openingBrace + 1, index);
    }
  }
  return null;
}

function validatePackageScripts(root, errors) {
  const rootPackage = readJSON(root, 'package.json');
  const e2ePackage = readJSON(root, 'web/e2e/package.json');
  if (rootPackage.scripts?.['check:mobile-clients-foundation'] !== ROOT_SCRIPT) {
    errors.push('check:mobile-clients-foundation package script is missing or incorrect');
  }
  if (e2ePackage.scripts?.['test:mobile-clients-foundation'] !== E2E_SCRIPT) {
    errors.push('test:mobile-clients-foundation E2E package script is missing or incorrect');
  }
  if (rootPackage.scripts?.['capture:mobile-visual-evidence'] !== CAPTURE_SCRIPT) {
    errors.push('capture:mobile-visual-evidence package script is missing or incorrect');
  }
}

function validateCaptureScript(root, errors) {
  const capturePath = join(root, 'scripts/capture_mobile_visual_evidence.mjs');
  const source = readFileSync(capturePath, 'utf8');
  if (!/process\.env\.MOCHAT_MOBILE_VISUAL_OUTPUT/.test(source)) {
    errors.push('visual evidence capture must require MOCHAT_MOBILE_VISUAL_OUTPUT');
  }
  if (!/\bchromium\.launch\(/.test(source) || !/page\.screenshot\(/.test(source)) {
    errors.push('visual evidence capture must use Playwright Chromium screenshots');
  }
  if (!/createServer\(/.test(source) || !/server\.listen\(\s*0\s*,/.test(source)) {
    errors.push('visual evidence capture must serve the current build on an isolated local port');
  }
  if (/fullPage\s*:\s*true/.test(source)) {
    errors.push('visual evidence capture must preserve fixed viewport height instead of fullPage');
  }
  if (!/finally\s*\{[\s\S]*?try\s*\{[\s\S]*?browser\.close\(\)[\s\S]*?finally\s*\{[\s\S]*?server\.close\(/.test(source)) {
    errors.push('visual evidence capture must close the local server even when browser cleanup fails');
  }
  for (const filename of VISUAL_EVIDENCE_FILES) {
    if (countMatches(source, new RegExp(filename.replaceAll('.', '\\.'), 'g')) !== 1) {
      errors.push(`visual evidence capture must declare ${filename} exactly once`);
    }
  }
  if (/console\.log\([^\n]*(?:token|cookie|state)|process\.stdout\.write\([^\n]*(?:token|cookie|state)/i.test(source)) {
    errors.push('visual evidence capture must not log session secrets');
  }
  const contract = spawnSync(process.execPath, [capturePath, '--contract'], {
    cwd: root,
    encoding: 'utf8',
    env: { ...process.env, MOCHAT_MOBILE_VISUAL_OUTPUT: join(root, 'visual-contract-output') },
    windowsHide: true,
  });
  try {
    const parsed = JSON.parse(contract.stdout);
    if (
      contract.status !== 0
      || parsed.fixedViewport !== true
      || JSON.stringify(parsed.files) !== JSON.stringify(VISUAL_EVIDENCE_FILES)
    ) {
      errors.push('visual evidence capture executable contract is invalid');
    }
  } catch {
    errors.push('visual evidence capture executable contract is invalid');
  }
}

function crc32(bytes) {
  let crc = 0xffffffff;
  for (const byte of bytes) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit += 1) {
      crc = (crc >>> 1) ^ ((crc & 1) === 1 ? 0xedb88320 : 0);
    }
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function decodePng(filePath) {
  const bytes = readFileSync(filePath);
  const pngSignature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
  if (bytes.length < 57 || !bytes.subarray(0, pngSignature.length).equals(pngSignature)) {
    throw new Error(`${filePath} is not a valid PNG`);
  }

  let offset = pngSignature.length;
  let header;
  let ended = false;
  let hasPalette = false;
  const imageData = [];
  while (offset < bytes.length) {
    if (offset + 12 > bytes.length) throw new Error(`${filePath} has a truncated PNG chunk`);
    const length = bytes.readUInt32BE(offset);
    const end = offset + 12 + length;
    if (end > bytes.length) throw new Error(`${filePath} has a truncated PNG chunk`);
    const type = bytes.toString('ascii', offset + 4, offset + 8);
    const data = bytes.subarray(offset + 8, offset + 8 + length);
    const expectedCRC = bytes.readUInt32BE(offset + 8 + length);
    const actualCRC = crc32(bytes.subarray(offset + 4, offset + 8 + length));
    if (actualCRC !== expectedCRC) throw new Error(`${filePath} has an invalid ${type} CRC`);
    if (header === undefined && type !== 'IHDR') throw new Error(`${filePath} must start with IHDR`);
    if (type === 'IHDR') {
      if (header !== undefined || length !== 13) throw new Error(`${filePath} has an invalid IHDR`);
      header = {
        width: data.readUInt32BE(0),
        height: data.readUInt32BE(4),
        bitDepth: data[8],
        colorType: data[9],
        compression: data[10],
        filter: data[11],
        interlace: data[12],
      };
    } else if (type === 'IDAT') {
      imageData.push(data);
    } else if (type === 'PLTE') {
      hasPalette = true;
    } else if (type === 'IEND') {
      if (length !== 0 || end !== bytes.length) throw new Error(`${filePath} has an invalid IEND`);
      ended = true;
    }
    offset = end;
  }

  if (header === undefined || imageData.length === 0 || !ended) {
    throw new Error(`${filePath} is missing required PNG chunks`);
  }
  const encodingByColorType = new Map([
    [0, { channels: 1, bitDepths: new Set([1, 2, 4, 8, 16]) }],
    [2, { channels: 3, bitDepths: new Set([8, 16]) }],
    [3, { channels: 1, bitDepths: new Set([1, 2, 4, 8]) }],
    [4, { channels: 2, bitDepths: new Set([8, 16]) }],
    [6, { channels: 4, bitDepths: new Set([8, 16]) }],
  ]);
  const encoding = encodingByColorType.get(header.colorType);
  if (
    header.width === 0
    || header.height === 0
    || encoding === undefined
    || !encoding.bitDepths.has(header.bitDepth)
    || (header.colorType === 3 && !hasPalette)
    || header.compression !== 0
    || header.filter !== 0
    || header.interlace !== 0
  ) {
    throw new Error(`${filePath} uses an unsupported PNG encoding`);
  }
  let decoded;
  try {
    decoded = inflateSync(Buffer.concat(imageData));
  } catch {
    throw new Error(`${filePath} has invalid compressed PNG image data`);
  }
  const rowBytes = Math.ceil((header.width * encoding.channels * header.bitDepth) / 8);
  if (decoded.length !== header.height * (rowBytes + 1)) {
    throw new Error(`${filePath} has an invalid decoded PNG raster length`);
  }
  for (let row = 0; row < header.height; row += 1) {
    if (decoded[row * (rowBytes + 1)] > 4) {
      throw new Error(`${filePath} has an invalid PNG row filter`);
    }
  }
  return { width: header.width, height: header.height };
}

export function verifyCaptureEvidenceRuntime(root = process.cwd()) {
  const resolvedRoot = resolve(root);
  const capturePath = join(resolvedRoot, 'scripts/capture_mobile_visual_evidence.mjs');
  const outputDirectory = mkdtempSync(join(tmpdir(), 'mochat-mobile-visual-runtime-'));
  try {
    const capture = spawnSync(process.execPath, [capturePath], {
      cwd: resolvedRoot,
      encoding: 'utf8',
      env: { ...process.env, MOCHAT_MOBILE_VISUAL_OUTPUT: outputDirectory },
      timeout: 120_000,
      windowsHide: true,
    });
    if (capture.status !== 0) {
      const detail = (capture.stderr || capture.stdout || 'unknown capture failure').trim();
      throw new Error(`visual evidence runtime capture failed: ${detail}`);
    }

    const actualFiles = readdirSync(outputDirectory).sort();
    const expectedFiles = [...VISUAL_EVIDENCE_FILES].sort();
    if (JSON.stringify(actualFiles) !== JSON.stringify(expectedFiles)) {
      throw new Error(`visual evidence runtime inventory mismatch: ${actualFiles.join(', ')}`);
    }

    for (const filename of VISUAL_EVIDENCE_FILES) {
      const filePath = join(outputDirectory, filename);
      if (!existsSync(filePath) || statSync(filePath).size < 1_024) {
        throw new Error(`visual evidence runtime screenshot is missing or empty: ${filename}`);
      }
      const dimensions = decodePng(filePath);
      const expected = filename.includes('-390.')
        ? { width: 390, height: 844 }
        : { width: 1280, height: 900 };
      if (dimensions.width !== expected.width || dimensions.height !== expected.height) {
        throw new Error(
          `visual evidence runtime screenshot has wrong dimensions: ${filename} `
          + `${dimensions.width}x${dimensions.height}`,
        );
      }
    }
  } finally {
    rmSync(outputDirectory, { force: true, recursive: true });
  }
}

function validateSidebarNavigationCases(sidebarCasesBody, errors) {
  const entries = [...sidebarCasesBody.matchAll(/\{[^\n}]*\bpath\s*:\s*(['"])(\/[^'"]*)\1[^\n}]*\}/g)];
  for (const entry of entries) {
    const [source, , path] = entry;
    const publicRoute = ['/auth', '/codeAuth', '/login'].includes(path);
    if (publicRoute) {
      if (!/needsSession\s*:\s*false/.test(source)) {
        errors.push(`Sidebar ${path} browser case must not expect employee navigation`);
      }
      continue;
    }
    const expected = ['/contactSop', '/roomSop'].includes(path) ? '会话' : '客户';
    if (
      !/needsSession\s*:\s*true/.test(source)
      || !new RegExp(`activeNavigation\\s*:\\s*['"]${expected}['"]`).test(source)
    ) {
      errors.push(`Sidebar ${path} browser case must map active navigation to ${expected}`);
    }
  }
}

function validateVisualSource(root, sources, errors) {
  const sidebarShell = readFileSync(
    join(root, 'web/apps/sidebar/src/ui/sidebar-page-shell.tsx'),
    'utf8',
  );
  if (
    !/\bMobileBottomNavigation\b/.test(sidebarShell)
    || !/label\s*=\s*['"]员工工作台['"]/.test(sidebarShell)
    || !/label\s*:\s*['"]客户['"]/.test(sidebarShell)
    || !/label\s*:\s*['"]会话['"]/.test(sidebarShell)
    || !/label\s*:\s*['"]我的['"]/.test(sidebarShell)
  ) {
    errors.push('Sidebar production shell must render the exact three-tab employee navigation');
  }

  for (const file of sources) {
    if (
      file.path.startsWith('web/apps/operation/')
      && /\bMobileBottomNavigation\b|员工工作台/.test(file.source)
    ) {
      errors.push(`Operation must not render employee navigation in ${file.path}`);
    }
    if (/圆弧AI会话/.test(file.source)) {
      errors.push(`reference brand text found in ${file.path}`);
    }
    if (
      /<img\b[^>]*\bsrc\s*=\s*['"]https?:\/\//i.test(file.source)
      || /background(?:-image)?\s*:\s*url\(\s*['"]?https?:\/\//i.test(file.source)
      || /@import\s+(?:url\()?\s*['"]?https?:\/\//i.test(file.source)
      || /const\s+(\w+)\s*=\s*['"]https?:\/\/[^'"]+['"][\s\S]{0,1000}?<img\b[^>]*\bsrc\s*=\s*\{\s*\1\s*\}/i.test(file.source)
    ) {
      errors.push(`static external image found in ${file.path}`);
    }
  }
}

function validateSidebarSessionIsolation(root, errors) {
  const sessionSource = readFileSync(
    join(root, 'web/apps/sidebar/src/auth/sidebar-session.ts'),
    'utf8',
  );
  const sessionTestSource = readFileSync(
    join(root, 'web/apps/sidebar/src/auth/sidebar-session.test.ts'),
    'utf8',
  );
  const mainSource = readFileSync(join(root, 'web/apps/sidebar/src/main.tsx'), 'utf8');

  if (/\bPath=\/(?:;|['"])/.test(sessionSource)) {
    errors.push('Sidebar token cookie must not use Path=/');
  }
  if (
    !/function\s+createSessionStorageSidebarSessionAdapter\s*\(/.test(sessionSource)
    || !/['"]mochat_sidebar_session_v1['"]/.test(sessionSource)
    || !/storage\.getItem\(/.test(sessionSource)
    || !/storage\.setItem\(/.test(sessionSource)
    || !/storage\.removeItem\(/.test(sessionSource)
  ) {
    errors.push('independent-root Sidebar sessionStorage adapter is missing or incomplete');
  }
  if (
    !/const\s+basename\s*=\s*sidebarBasename\(window\.location\.pathname\)/.test(mainSource)
    || !/basename\s*===\s*['"]\/sidebar-app['"][\s\S]{0,300}?createCookieSidebarSessionAdapter[\s\S]{0,300}?:\s*createSessionStorageSidebarSessionAdapter\(window\.sessionStorage\)/.test(mainSource)
  ) {
    errors.push('Sidebar main must select cookie or sessionStorage from its runtime basename');
  }
  if (
    !/root(?:-mount)?[^\n]{0,200}sessionStorage/i.test(sessionTestSource)
    || !/malformed[^\n]{0,200}(?:fail|closed)/i.test(sessionTestSource)
    || !/createSessionStorageSidebarSessionAdapter\(/.test(sessionTestSource)
  ) {
    errors.push('root Sidebar sessionStorage callback and fail-closed tests are missing');
  }
}

function validateRawGoBrowserFixtures(source, errors) {
  if (!/contact\s*:\s*['"]\*\*\/sidebar\/workContact\/detail\?\*['"]/.test(source)) {
    errors.push('contact raw Go envelope fixture endpoint is missing');
  }
  if (!/taskData\s*:\s*['"]\*\*\/operation\/workFission\/taskData\?\*['"]/.test(source)) {
    errors.push('taskData raw Go envelope fixture endpoint is missing');
  }
  if (!/openUserInfo\s*:\s*['"]\*\*\/operation\/openUserInfo\/workFission\?\*['"]/.test(source)) {
    errors.push('openUserInfo raw Go envelope fixture endpoint is missing');
  }
  if (!/JSON\.stringify\(\{\s*code\s*:\s*200\s*,\s*msg\s*:\s*['"]ok['"]\s*,\s*data\b[\s\S]{0,120}?\}\)/.test(source)) {
    errors.push('raw Go envelope helper must preserve code, msg and data fields');
  }
  if (
    !/const\s+rawContactData\s*=\s*\{[\s\S]{0,400}?\bid\s*:[\s\S]{0,200}?\bname\s*:[\s\S]{0,200}?\bavatar\s*:[\s\S]{0,200}?\bcorpId\s*:/.test(source)
    || !/page\.route\(\s*browserContract\.fixtures\.contact[\s\S]{0,700}?body\s*:\s*rawGoEnvelope\(\s*rawContactData\b/.test(source)
  ) {
    errors.push('contact route must return a raw Go envelope');
  }
  if (
    !/const\s+rawTaskData\s*=\s*\{[\s\S]{0,900}?\binvite_count\s*:[\s\S]{0,240}?\bdiffer_count\s*:[\s\S]{0,240}?\bend_time\s*:[\s\S]{0,300}?\btask\s*:[\s\S]{0,500}?\breceive_status\s*:[\s\S]{0,240}?\bgift_type\s*:[\s\S]{0,240}?\bgift_url\s*:/.test(source)
    || !/page\.route\(\s*browserContract\.fixtures\.taskData[\s\S]{0,700}?body\s*:\s*rawGoEnvelope\(\s*rawTaskData\b/.test(source)
  ) {
    errors.push('taskData route must return a raw Go envelope');
  }
  if (
    !/const\s+rawWorkFissionParticipant\s*=\s*\{[\s\S]{0,500}?\bopenid\s*:[\s\S]{0,160}?\bunionid\s*:[\s\S]{0,160}?\bnickname\s*:[\s\S]{0,160}?\bheadimgurl\s*:/.test(source)
    || !/page\.route\(\s*browserContract\.fixtures\.openUserInfo[\s\S]{0,900}?body\s*:\s*rawGoEnvelope\(\s*audit\.participantData\b/.test(source)
  ) {
    errors.push('openUserInfo route must return a raw Go envelope');
  }
}

function validateBrowserAudit(source, errors) {
  const collectors = [
    [/page\.on\(\s*['"]console['"][\s\S]{0,260}?message\.type\(\)\s*===\s*['"]error['"][\s\S]{0,180}?audit\.consoleErrors\.push\(/, 'console error audit collector is missing'],
    [/page\.on\(\s*['"]pageerror['"][\s\S]{0,220}?audit\.pageErrors\.push\(/, 'pageerror audit collector is missing'],
    [/page\.on\(\s*['"]requestfailed['"][\s\S]{0,260}?audit\.requestFailures\.push\(/, 'requestfailed audit collector is missing'],
    [/page\.on\(\s*['"]response['"][\s\S]{0,260}?response\.status\(\)\s*>=\s*400[\s\S]{0,220}?audit\.unexpectedResponses\.push\(/, '400 response audit collector is missing'],
  ];
  for (const [pattern, message] of collectors) {
    if (!pattern.test(source)) errors.push(message);
  }
  if ((source.match(/audit\.unexpectedRequests\.push\(/g) ?? []).length < 2) {
    errors.push('unexpected request audit collectors are missing');
  }

  const assertions = [
    ['consoleErrors', 'console error'],
    ['pageErrors', 'pageerror'],
    ['requestFailures', 'requestfailed'],
    ['unexpectedResponses', '400 response'],
    ['unexpectedRequests', 'unexpected request'],
  ];
  for (const [field, label] of assertions) {
    const assertion = new RegExp(`expect\\(audit\\.${field}(?:\\s*,[^)]*)?\\)\\.toEqual\\(\\[\\]\\)`);
    if (!assertion.test(source)) errors.push(`${label} clean-audit final assertion is missing`);
  }

  const stableAudit = /async\s+function\s+assertStableCleanAudit\s*\([^)]*\)\s*(?::\s*Promise<\s*void\s*>\s*)?\{[\s\S]{0,500}?await\s+page\.waitForLoadState\(\s*['"]networkidle['"]\s*\)[\s\S]{0,220}?await\s+page\.waitForTimeout\(\s*0\s*\)[\s\S]{0,220}?assertCleanAudit\(\s*audit/.test(source);
  if (!stableAudit) errors.push('networkidle and event-loop stabilization must precede clean audit assertions');
  if ((source.match(/await\s+assertStableCleanAudit\(/g) ?? []).length < 4) {
    errors.push('all known and unknown route groups must execute the stable clean audit');
  }
}

function validateBrowserLayout(source, sidebarCasesBody, operationCasesBody, errors) {
  if (
    !/clientWidth\s*:\s*document\.documentElement\.clientWidth/.test(source)
    || !/scrollWidth\s*:\s*document\.documentElement\.scrollWidth/.test(source)
    || !/expect\(pageShape\.scrollWidth(?:\s*,[^)]*)?\)\.toBeLessThanOrEqual\(pageShape\.clientWidth\)/.test(source)
  ) {
    errors.push('browser layout audit must assert scrollWidth is no greater than clientWidth');
  }

  const caseCount = countMatches(sidebarCasesBody, /\bpath\s*:/g)
    + countMatches(operationCasesBody, /\bpath\s*:/g);
  const actionFlagCount = countMatches(`${sidebarCasesBody}\n${operationCasesBody}`, /\bexpectsAction\s*:\s*(?:true|false)/g);
  if (actionFlagCount !== caseCount) errors.push('every browser route case must declare expectsAction');
  if (!/path\s*:\s*['"]\/login['"][^\n]*expectsAction\s*:\s*true/.test(sidebarCasesBody)) {
    errors.push('Sidebar login browser case must declare expectsAction true');
  }
  if (!/minimumActionHeight\s*:\s*44\b/.test(source)) {
    errors.push('browser contract must declare a 44px minimum action height');
  }
  if (!/if\s*\(\s*expectsAction\s*\)[\s\S]{0,180}?expect\(await\s+controls\.count\(\)\)\.toBeGreaterThanOrEqual\(1\)/.test(source)) {
    errors.push('expectsAction routes must enforce a visible control count of at least one');
  }
  if (!/expect\(box\?\.height\s*\?\?\s*0(?:\s*,[^)]*)?\)\.toBeGreaterThanOrEqual\(browserContract\.minimumActionHeight\)/.test(source)) {
    errors.push('visible action controls must enforce the 44px browser contract');
  }
}

function validateViewportRouteLoops(source, operationCasesBody, errors) {
  const viewportBody = braceBody(source, /for\s*\(const\s+viewport\s+of\s+viewports\)\s*\{/);
  if (viewportBody === null) {
    errors.push('viewport loop must cover all viewports and both route case arrays');
    return;
  }
  const sidebarBody = braceBody(viewportBody, /for\s*\(const\s+routeCase\s+of\s+sidebarCases\)\s*\{/);
  const operationBody = braceBody(viewportBody, /for\s*\(const\s+routeCase\s+of\s+operationCases\)\s*\{/);
  if (sidebarBody === null || operationBody === null) {
    errors.push('viewport loop must cover all viewports and both route case arrays');
    return;
  }
  for (const [label, loopBody] of [['Sidebar', sidebarBody], ['Operation', operationBody]]) {
    if (!/assertVisiblePage\([^;]+routeCase\.expectsAction\s*\)/.test(loopBody)) {
      errors.push(`${label} viewport loop must pass expectsAction to the layout assertion`);
    }
    if (!/await\s+assertStableCleanAudit\(/.test(loopBody)) {
      errors.push(`${label} viewport loop must execute the stable clean audit`);
    }
  }

  if (
    !/routeCase\.needsSession/.test(sidebarBody)
    || !/employeeNavigationLabel[\s\S]{0,240}?navigation[\s\S]{0,160}?toBeVisible/.test(sidebarBody)
    || !/routeCase\.activeNavigation[\s\S]{0,400}?aria-current[\s\S]{0,120}?page/.test(sidebarBody)
  ) {
    errors.push('Sidebar viewport loop must assert visible and active Sidebar navigation');
  }
  if (!/await\s+assertContentAboveBottomNavigation\(\s*page\s*\)/.test(sidebarBody)) {
    errors.push('Sidebar bottom navigation must not cover the last content item');
  }
  if (
    !/navigation\.locator\(\s*['"]a['"]\s*\)[\s\S]{0,220}?\/sidebar-app/.test(sidebarBody)
    || !/navigation\.getByRole\(\s*['"]link['"]\s*,\s*\{\s*name\s*:\s*['"]客户['"]\s*,\s*exact\s*:\s*true\s*\}\s*\)[\s\S]{0,80}?toBeVisible/.test(sidebarBody)
    || !/navigation\.getByRole\(\s*['"]link['"]\s*,\s*\{\s*name\s*:\s*['"]会话['"]\s*,\s*exact\s*:\s*true\s*\}\s*\)[\s\S]{0,80}?toBeVisible/.test(sidebarBody)
    || !/navigation\.getByRole\(\s*['"]link['"]\s*,\s*\{\s*name\s*:\s*['"]我的['"]\s*,\s*exact\s*:\s*true\s*\}\s*\)[\s\S]{0,80}?toBeVisible/.test(sidebarBody)
  ) {
    errors.push('Sidebar viewport loop must verify scoped links and all three employee navigation tabs');
  }
  if (
    !/employeeNavigationLabel[\s\S]{0,240}?toHaveCount\(\s*0\s*\)/.test(operationBody)
  ) {
    errors.push('Operation viewport loop must assert no employee navigation');
  }

  if (
    !/async\s+function\s+assertContentAboveBottomNavigation[\s\S]{0,700}?lastContent[\s\S]{0,300}?navigationBox[\s\S]{0,300}?contentBox[\s\S]{0,300}?toBeLessThanOrEqual/.test(source)
  ) {
    errors.push('bottom navigation content-cover geometry assertion is missing');
  }

  if (!/path\s*:\s*['"]\/workFission['"][^\n]*query\s*:\s*['"]\?id=[1-9]\d*['"]/.test(operationCasesBody)) {
    errors.push('workFission browser case must use the real positive id entry');
  }
  if (/workFissionRequests|workFissionParticipantRequests|\/auth\/workFission/.test(sidebarBody)) {
    errors.push('workFission evidence must not be placed in the Sidebar route loop');
  }

  const workFissionBody = braceBody(
    operationBody,
    /if\s*\(\s*routeCase\.path\s*===\s*['"]\/workFission['"]\s*\)\s*\{/,
  );
  if (workFissionBody === null) {
    errors.push('Operation workFission evidence branch is missing');
    return;
  }
  if (
    !/workFissionRequests\[0\][\s\S]{0,220}?searchParams\.get\(\s*['"]union_id['"]\s*\)[\s\S]{0,220}?toBe\(/.test(workFissionBody)
    || !/rawWorkFissionParticipant[\s\S]{0,400}?unionid\s*:\s*['"][^'"]*session[^'"]*['"]/.test(source)
  ) {
    errors.push('taskData browser fixture must prove a session-derived unionid in the Operation workFission branch');
  }
  if (
    !/participantData\s*=\s*\[\]/.test(workFissionBody)
    || !/participantData\s*=\s*\[\][\s\S]{0,700}?\/auth\/workFission\?id=/.test(workFissionBody)
  ) {
    errors.push('empty participant browser fixture must prove the OAuth href in the Operation workFission branch');
  }
  if (
    !/workFissionEvidenceChecks\s*\+=\s*1/.test(workFissionBody)
    || !/expect\(\s*audit\.workFissionEvidenceChecks\s*\)\.toBe\(\s*1\s*\)/.test(workFissionBody)
  ) {
    errors.push('workFission evidence execution count must be exactly one inside the Operation branch');
  }
}

export function auditMobileClientsFoundation(root = process.cwd()) {
  const resolvedRoot = resolve(root);
  const errors = [];
  const sidebarManifest = manifestPaths(
    readJSON(resolvedRoot, 'web/apps/sidebar/src/migration-routes.json'),
    'Sidebar',
    errors,
  );
  const operationManifest = manifestPaths(
    readJSON(resolvedRoot, 'web/apps/operation/src/migration-routes.json'),
    'Operation',
    errors,
  );

  const sidebarRegistry = registryPaths(
    readFileSync(join(resolvedRoot, 'web/apps/sidebar/src/routes/registry.tsx'), 'utf8'),
    'Sidebar',
    errors,
  );
  const operationRegistry = registryPaths(
    readFileSync(join(resolvedRoot, 'web/apps/operation/src/routes/registry.tsx'), 'utf8'),
    'Operation',
    errors,
  );
  compareRegistryToManifest('Sidebar', sidebarRegistry, sidebarManifest, errors);
  compareRegistryToManifest('Operation', operationRegistry, operationManifest, errors);

  const sources = [
    ...productionSources(resolvedRoot, 'web/apps/sidebar/src'),
    ...productionSources(resolvedRoot, 'web/apps/operation/src'),
  ];
  validateVisualSource(resolvedRoot, sources, errors);
  let directFetch = 0;
  let dashboardSessionReferences = 0;
  let mojibakeMarkers = 0;
  let fakeBusinessOutcomes = 0;
  for (const file of sources) {
    const fetchCount = countMatches(file.source, /\bfetch\s*\(/g);
    if (fetchCount > 0) errors.push(`direct fetch found in production source ${file.path}`);
    directFetch += fetchCount;

    const dashboardCount = countMatches(file.source, /\bmochat_dashboard_[a-z\d_]+\b/gi);
    if (dashboardCount > 0) errors.push(`Dashboard session reference found in ${file.path}`);
    dashboardSessionReferences += dashboardCount;

    const mojibakeCount = countMatches(file.source, MOJIBAKE_PATTERN);
    if (mojibakeCount > 0) errors.push(`mojibake marker found in ${file.path}`);
    mojibakeMarkers += mojibakeCount;

    const fakeCount = FAKE_OUTCOME_PATTERNS.reduce(
      (count, pattern) => count + countMatches(file.source, pattern),
      0,
    );
    if (fakeCount > 0) errors.push(`fake business outcome found in ${file.path}`);
    fakeBusinessOutcomes += fakeCount;
  }

  validateUnknownRoute(
    'Sidebar',
    readFileSync(join(resolvedRoot, 'web/apps/sidebar/src/app/sidebar-router.tsx'), 'utf8'),
    errors,
  );
  validateUnknownRoute(
    'Operation',
    readFileSync(join(resolvedRoot, 'web/apps/operation/src/app/operation-router.tsx'), 'utf8'),
    errors,
  );

  const e2eSource = readFileSync(
    join(resolvedRoot, 'web/e2e/tests/mobile-clients-foundation.spec.ts'),
    'utf8',
  );
  const sidebarCasesBody = specArrayBody(e2eSource, 'sidebarCases', errors);
  const operationCasesBody = specArrayBody(e2eSource, 'operationCases', errors);
  const sidebarCases = specCasePaths(e2eSource, 'sidebarCases', errors);
  const operationCases = specCasePaths(e2eSource, 'operationCases', errors);
  compareBrowserCases('Sidebar', sidebarCases, sidebarManifest, errors);
  compareBrowserCases('Operation', operationCases, operationManifest, errors);
  validateSidebarNavigationCases(sidebarCasesBody, errors);

  const hasMobileViewport = /width\s*:\s*390\b[\s\S]{0,80}?height\s*:\s*844\b/.test(e2eSource);
  const hasDesktopViewport = /width\s*:\s*1280\b[\s\S]{0,80}?height\s*:\s*900\b/.test(e2eSource);
  if (!hasMobileViewport) errors.push('browser spec must include the 390 by 844 viewport');
  if (!hasDesktopViewport) errors.push('browser spec must include the 1280 by 900 viewport');
  validateRawGoBrowserFixtures(e2eSource, errors);
  validateSidebarSessionIsolation(resolvedRoot, errors);
  validateBrowserAudit(e2eSource, errors);
  validateBrowserLayout(e2eSource, sidebarCasesBody, operationCasesBody, errors);
  validateViewportRouteLoops(e2eSource, operationCasesBody, errors);
  if (
    !e2eSource.includes('/sidebar-app/not-a-sidebar-page')
    || !e2eSource.includes('/operation-app/not-an-operation-page')
    || !e2eSource.includes('页面不存在')
    || !e2eSource.includes('toHaveCount(0)')
  ) {
    errors.push('browser spec must prove unknown routes show 404 without home content');
  }

  validatePackageScripts(resolvedRoot, errors);
  validateCaptureScript(resolvedRoot, errors);
  if (errors.length > 0) {
    throw new Error(`mobile clients foundation gate failed:\n- ${errors.join('\n- ')}`);
  }

  return {
    sidebarRoutes: sidebarManifest.length,
    operationRoutes: operationManifest.length,
    directFetch,
    dashboardSessionReferences,
    mojibakeMarkers,
    fakeBusinessOutcomes,
    mobileViewportCases: hasMobileViewport ? sidebarCases.length + operationCases.length : 0,
  };
}

export function formatMobileClientsFoundationSummary(result) {
  return [
    `sidebar routes=${result.sidebarRoutes}`,
    `operation routes=${result.operationRoutes}`,
    `direct fetch=${result.directFetch}`,
    `dashboard session references=${result.dashboardSessionReferences}`,
    `mojibake markers=${result.mojibakeMarkers}`,
    `fake business outcomes=${result.fakeBusinessOutcomes}`,
    `mobile viewport cases>=${result.mobileViewportCases}`,
  ].join('\n');
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const result = auditMobileClientsFoundation();
  verifyCaptureEvidenceRuntime();
  console.log(formatMobileClientsFoundationSummary(result));
}
