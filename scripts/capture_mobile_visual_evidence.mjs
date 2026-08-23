import { createServer } from 'node:http';
import { existsSync, mkdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, extname, isAbsolute, join, normalize, resolve, sep } from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const repositoryRoot = resolve(scriptDirectory, '..');
const capturePlan = [
  { viewport: 'mobile', url: '/sidebar-app/contact?wxExternalUserid=visual-contact&agentId=7', filename: 'sidebar-contact-390.png', readyText: '林小青' },
  { viewport: 'mobile', url: '/sidebar-app/', filename: 'sidebar-workbench-390.png', readyText: '客户经营工作台' },
  { viewport: 'mobile', url: '/sidebar-app/roomSop?id=5', filename: 'sidebar-pending-390.png', readyText: '视觉验收客户群' },
  { viewport: 'mobile', url: '/operation-app/workFission?id=17', filename: 'operation-work-fission-390.png', readyText: '已邀请 2 位好友' },
  { viewport: 'mobile', url: '/operation-app/lottery', filename: 'operation-pending-390.png', readyText: '抽奖活动模块待迁移' },
  { viewport: 'desktop', url: '/sidebar-app/contact?wxExternalUserid=visual-contact&agentId=7', filename: 'sidebar-contact-1280.png', readyText: '林小青' },
  { viewport: 'desktop', url: '/operation-app/workFission?id=17', filename: 'operation-work-fission-1280.png', readyText: '已邀请 2 位好友' },
];

if (process.argv.includes('--contract')) {
  process.stdout.write(JSON.stringify({
    files: capturePlan.map(({ filename }) => filename),
    fixedViewport: true,
  }));
  process.exit(0);
}

const outputValue = process.env.MOCHAT_MOBILE_VISUAL_OUTPUT?.trim() ?? '';
if (outputValue.length === 0) {
  throw new Error('MOCHAT_MOBILE_VISUAL_OUTPUT is required for mobile visual evidence.');
}
const outputDirectory = isAbsolute(outputValue)
  ? outputValue
  : resolve(repositoryRoot, outputValue);
mkdirSync(outputDirectory, { recursive: true });

const require = createRequire(import.meta.url);
const { chromium } = require(join(
  repositoryRoot,
  'web/e2e/node_modules/@playwright/test',
));

const contentTypes = new Map([
  ['.css', 'text/css; charset=utf-8'],
  ['.html', 'text/html; charset=utf-8'],
  ['.js', 'text/javascript; charset=utf-8'],
  ['.json', 'application/json; charset=utf-8'],
  ['.svg', 'image/svg+xml'],
]);

function mountedFile(urlPath) {
  if (urlPath.startsWith('/assets/')) {
    for (const dist of [
      join(repositoryRoot, 'web/apps/sidebar/dist'),
      join(repositoryRoot, 'web/apps/operation/dist'),
    ]) {
      const candidate = normalize(join(dist, urlPath.slice(1)));
      const safePrefix = `${normalize(dist)}${sep}`;
      if (candidate.startsWith(safePrefix) && existsSync(candidate) && statSync(candidate).isFile()) {
        return candidate;
      }
    }
    return null;
  }
  for (const [mount, dist] of [
    ['/sidebar-app', join(repositoryRoot, 'web/apps/sidebar/dist')],
    ['/operation-app', join(repositoryRoot, 'web/apps/operation/dist')],
  ]) {
    if (urlPath !== mount && !urlPath.startsWith(`${mount}/`)) continue;
    const relativePath = urlPath.slice(mount.length).replace(/^\/+/, '');
    const candidate = normalize(join(dist, relativePath));
    const safePrefix = `${normalize(dist)}${sep}`;
    if (candidate !== normalize(dist) && !candidate.startsWith(safePrefix)) return null;
    if (relativePath.length > 0 && existsSync(candidate) && statSync(candidate).isFile()) {
      return candidate;
    }
    return join(dist, 'index.html');
  }
  return null;
}

const server = createServer((request, response) => {
  const url = new URL(request.url ?? '/', 'http://127.0.0.1');
  const filePath = mountedFile(url.pathname);
  if (filePath === null || !existsSync(filePath)) {
    response.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
    response.end('not found');
    return;
  }
  response.writeHead(200, {
    'Cache-Control': 'no-store',
    'Content-Type': contentTypes.get(extname(filePath)) ?? 'application/octet-stream',
  });
  response.end(readFileSync(filePath));
});

await new Promise((resolveListen, rejectListen) => {
  server.once('error', rejectListen);
  server.listen(0, '127.0.0.1', resolveListen);
});
const address = server.address();
if (address === null || typeof address === 'string') throw new Error('visual server address unavailable.');
const baseURL = `http://127.0.0.1:${address.port}`;

function envelope(data, requestId) {
  return JSON.stringify({ code: 200, msg: 'ok', data, requestId });
}

async function installFixtures(page) {
  await page.route('**/sidebar/**', (route) => route.fulfill({
    status: 404,
    contentType: 'application/json',
    body: JSON.stringify({ code: 404, msg: 'unexpected fixture request', data: null }),
  }));
  await page.route('**/operation/**', (route) => route.fulfill({
    status: 404,
    contentType: 'application/json',
    body: JSON.stringify({ code: 404, msg: 'unexpected fixture request', data: null }),
  }));
  await page.route('**/sidebar/workContact/detail?*', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: envelope({ id: 23, name: '林小青', avatar: null, corpId: 9 }, 'visual-contact'),
  }));
  await page.route('**/sidebar/workbench/summary', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: envelope({
      employee: { id: 7, name: '员工甲', avatar: null, departmentNames: ['客户成功部'], corpName: '视觉验收企业' },
      customers: { total: 126, addedToday: 8, taggedTotal: 93, ownedRoomTotal: 12 },
      tasks: { contactSopRecords: 3, roomSopPending: 2, batchAddPending: 1 },
    }, 'visual-workbench-summary'),
  }));
  await page.route('**/sidebar/workContact/show?*', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: envelope({
      name: '林小青', avatar: null, gender: 2, genderText: '女', businessNo: 'C-23',
      remark: '重点客户', description: '偏好下午沟通', tag: [{ tagId: 7, tagName: '高意向' }],
      roomName: ['视觉验收客户群'], employeeName: ['员工甲'],
    }, 'visual-contact-workspace'),
  }));
  await page.route('**/sidebar/workContact/track?*', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: envelope([], 'visual-contact-track'),
  }));
  await page.route('**/sidebar/contactFieldPivot/index?*', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: envelope([], 'visual-contact-portrait'),
  }));
  await page.route('**/sidebar/roomSop/getSopInfo?*', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: envelope({
      id: 5, roomSopId: 13, creator: '员工甲', time: '10:00', state: 0,
      task: { content: [{ type: 0, value: '群内发送活动提醒' }] },
      room: { id: 21, name: '视觉验收客户群' },
    }, 'visual-room-sop'),
  }));
  await page.route('**/operation/openUserInfo/workFission?*', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: envelope({
      openid: 'visual-openid',
      unionid: 'visual-unionid',
      nickname: '活动参与者',
      headimgurl: '',
    }, 'visual-participant'),
  }));
  await page.route('**/operation/workFission/taskData?*', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: envelope({
      invite_count: 2,
      differ_count: 1,
      end_time: 4_102_444_800,
      task: [
        { count: 3, status: 0, receive_status: 0, gift_type: 0, gift_url: '/operation/reward/17' },
        { count: 5, status: 1, receive_status: 1, gift_type: 1, gift_url: '' },
      ],
    }, 'visual-task-data'),
  }));
}

let browser;
try {
  browser = await chromium.launch({ headless: true });
  const context = await browser.newContext();
  await context.addCookies([
    { name: 'token', value: 'visual-fixture-token', domain: '127.0.0.1', path: '/sidebar-app' },
    { name: 'agentId', value: '7', domain: '127.0.0.1', path: '/sidebar-app' },
  ]);
  const page = await context.newPage();
  await installFixtures(page);
  const viewports = {
    mobile: { width: 390, height: 844 },
    desktop: { width: 1280, height: 900 },
  };

  for (const capture of capturePlan) {
    await page.setViewportSize(viewports[capture.viewport]);
    await page.goto(`${baseURL}${capture.url}`);
    try {
      await page.getByText(capture.readyText, { exact: true }).first().waitFor({ state: 'visible' });
    } catch (error) {
      const visibleText = (await page.locator('body').innerText()).replaceAll(/\s+/g, ' ').slice(0, 500);
      throw new Error(`capture ${capture.url} ended at ${page.url()}: ${visibleText}`, { cause: error });
    }
    const screenshotPath = join(outputDirectory, capture.filename);
    await page.screenshot({ path: screenshotPath });
    process.stdout.write(`${screenshotPath}\n`);
  }
  await context.close();
} finally {
  try {
    if (browser !== undefined) await browser.close();
  } finally {
    await new Promise((resolveClose, rejectClose) => {
      server.close((error) => (error === undefined ? resolveClose() : rejectClose(error)));
    });
  }
}
