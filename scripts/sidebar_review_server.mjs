import { readFile, realpath, stat } from 'node:fs/promises';
import { createServer } from 'node:http';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const REQUEST_ID = 'sidebar-local-review';
const LOOPBACK_HOSTS = new Set(['127.0.0.1', 'localhost']);
const SIDEBAR_ROUTES = new Set([
  '/',
  '/auth',
  '/codeAuth',
  '/contact',
  '/contact/editDetail',
  '/contact/remark',
  '/contact/settingTag',
  '/contactBatchAdd',
  '/contactSop',
  '/login',
  '/medium',
  '/roomSop',
]);
const READ_ENDPOINTS = new Map([
  ['/sidebar/workContact/detail', 'contactDetail'],
  ['/sidebar/workContact/show', 'contactSummary'],
  ['/sidebar/workContact/track', 'tracks'],
  ['/sidebar/contactFieldPivot/index', 'portraitFields'],
  ['/sidebar/workContactTagGroup/index', 'tagGroups'],
  ['/sidebar/workContactTag/allTag', 'tags'],
  ['/sidebar/contactSop/getSopInfo', 'contactSop'],
  ['/sidebar/contactSop/getSopTipInfo', 'contactSopTips'],
  ['/sidebar/roomSop/getSopInfo', 'roomSop'],
  ['/sidebar/mediumGroup/index', 'mediumGroups'],
  ['/sidebar/medium/index', 'mediumPage'],
]);
const WRITE_ENDPOINTS = new Set([
  '/sidebar/workContact/update',
  '/sidebar/contactFieldPivot/update',
  '/sidebar/roomSop/logState',
]);
const MIME_TYPES = new Map([
  ['.css', 'text/css; charset=utf-8'],
  ['.gif', 'image/gif'],
  ['.html', 'text/html; charset=utf-8'],
  ['.ico', 'image/x-icon'],
  ['.jpeg', 'image/jpeg'],
  ['.jpg', 'image/jpeg'],
  ['.js', 'text/javascript; charset=utf-8'],
  ['.json', 'application/json; charset=utf-8'],
  ['.map', 'application/json; charset=utf-8'],
  ['.png', 'image/png'],
  ['.svg', 'image/svg+xml; charset=utf-8'],
  ['.webp', 'image/webp'],
  ['.woff', 'font/woff'],
  ['.woff2', 'font/woff2'],
]);

function clone(value) {
  return structuredClone(value);
}

function isRecord(value) {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function send(response, status, body, contentType, method = 'GET', headers = {}) {
  const bytes = Buffer.isBuffer(body) ? body : Buffer.from(body);
  response.writeHead(status, {
    'Cache-Control': 'no-store',
    'Content-Length': bytes.length,
    'Content-Type': contentType,
    'X-Content-Type-Options': 'nosniff',
    ...headers,
  });
  response.end(method === 'HEAD' ? undefined : bytes);
}

function sendText(response, status, message, method = 'GET', headers = {}) {
  send(response, status, `${message}\n`, 'text/plain; charset=utf-8', method, headers);
}

function sendJson(response, status, payload, method = 'GET', headers = {}) {
  send(
    response,
    status,
    JSON.stringify(payload),
    'application/json; charset=utf-8',
    method,
    headers,
  );
}

function sendSuccess(response, data, method = 'GET') {
  sendJson(response, 200, {
    code: 200,
    msg: 'ok',
    data,
    requestId: REQUEST_ID,
  }, method);
}

function sendError(response, status, message, method = 'GET', headers = {}) {
  sendJson(response, status, {
    code: status,
    msg: message,
    data: null,
    requestId: REQUEST_ID,
  }, method, headers);
}

function actualPort(server, configuredPort) {
  const address = server.address();
  return address && typeof address === 'object' ? address.port : configuredPort;
}

function allowedHost(rawHost, server, configuredPort) {
  if (typeof rawHost !== 'string') return false;
  const port = actualPort(server, configuredPort);
  return rawHost === `127.0.0.1:${port}` || rawHost.toLowerCase() === `localhost:${port}`;
}

function routeFromTarget(rawTarget, origin) {
  const absoluteTarget = typeof rawTarget === 'string'
    && /^[a-z][a-z\d+.-]*:\/\//i.test(rawTarget);
  if (
    typeof rawTarget !== 'string'
    || rawTarget.length === 0
    || (!rawTarget.startsWith('/') && !absoluteTarget)
    || rawTarget.includes('\\')
    || /[\u0000-\u001f\u007f]/.test(rawTarget)
    || rawTarget.startsWith('//')
  ) return null;

  let target;
  try {
    target = new URL(rawTarget, origin);
  } catch {
    return null;
  }
  if (target.origin !== origin.origin || target.username || target.password) return null;

  let sidebarPath = target.pathname;
  if (absoluteTarget && sidebarPath !== '/sidebar-app' && !sidebarPath.startsWith('/sidebar-app/')) {
    return null;
  }
  if (sidebarPath === '/sidebar-app' || sidebarPath === '/sidebar-app/') {
    sidebarPath = '/';
  } else if (sidebarPath.startsWith('/sidebar-app/')) {
    sidebarPath = sidebarPath.slice('/sidebar-app'.length);
  }
  if (!SIDEBAR_ROUTES.has(sidebarPath)) return null;
  return `${sidebarPath}${target.search}${target.hash}`;
}

export function reviewAuthLocation(requestURL, fixture, rawOrigin) {
  let origin;
  let request;
  try {
    origin = new URL(rawOrigin);
    request = new URL(requestURL, origin);
  } catch {
    return null;
  }
  if (
    origin.protocol !== 'http:'
    || !LOOPBACK_HOSTS.has(origin.hostname.toLowerCase())
    || request.origin !== origin.origin
    || request.pathname !== '/sidebar/agent/auth'
    || request.searchParams.get('agentId') !== String(fixture.agentId)
  ) return null;

  const target = routeFromTarget(request.searchParams.get('target'), origin);
  if (target === null) return null;
  const parsedTarget = new URL(target, origin);
  const sidebarTarget = new URL(
    `/sidebar-app${parsedTarget.pathname === '/' ? '/' : parsedTarget.pathname}`,
    origin,
  );
  sidebarTarget.search = parsedTarget.search;
  sidebarTarget.hash = parsedTarget.hash;

  const state = Buffer.from(JSON.stringify({
    code: 200,
    msg: '',
    data: { token: fixture.token, expire: 7200 },
  }), 'utf8').toString('base64');
  const callback = new URL('/sidebar-app/auth', origin);
  callback.searchParams.set('agentId', String(fixture.agentId));
  callback.searchParams.set('state', state);
  callback.searchParams.set('target', sidebarTarget.href);
  return callback.href;
}

async function readJsonBody(request) {
  const contentType = request.headers['content-type'];
  if (typeof contentType !== 'string' || !/^application\/json(?:\s*;|$)/i.test(contentType)) {
    return { error: [415, 'expected application/json request body'] };
  }
  const chunks = [];
  let size = 0;
  for await (const chunk of request) {
    size += chunk.length;
    if (size > 64 * 1024) return { error: [413, 'JSON request body is too large'] };
    chunks.push(chunk);
  }
  if (size === 0) return { error: [400, 'JSON request body is required'] };
  try {
    const value = JSON.parse(Buffer.concat(chunks).toString('utf8'));
    if (!isRecord(value)) return { error: [422, 'JSON request body must be an object'] };
    return { value };
  } catch {
    return { error: [400, 'invalid JSON request body'] };
  }
}

function fixedContactId(state) {
  return state.contactDetail?.id;
}

function validContactUpdate(body, state) {
  if (body.contactId !== fixedContactId(state)) return false;
  const fields = ['remark', 'description'].filter((name) => body[name] !== undefined);
  if (fields.some((name) => typeof body[name] !== 'string')) return false;
  if (body.tag !== undefined && (
    !Array.isArray(body.tag)
    || body.tag.some((tagId) => !Number.isSafeInteger(tagId) || tagId <= 0)
    || body.tag.some((tagId) => !state.tags.some((tag) => tag.id === tagId))
  )) return false;
  return fields.length > 0 || body.tag !== undefined;
}

function updateContact(body, state) {
  if (body.remark !== undefined) state.contactSummary.remark = body.remark;
  if (body.description !== undefined) state.contactSummary.description = body.description;
  if (body.tag === undefined) return;
  const existingIds = new Set(state.contactSummary.tag.map((tag) => tag.tagId));
  for (const tagId of body.tag) {
    if (existingIds.has(tagId)) continue;
    const tag = state.tags.find((candidate) => candidate.id === tagId);
    if (tag !== undefined) {
      state.contactSummary.tag.push({ tagId: tag.id, tagName: tag.name });
      existingIds.add(tagId);
    }
  }
}

function validPortrait(body, state) {
  return body.contactId === fixedContactId(state)
    && Array.isArray(body.userPortrait)
    && body.userPortrait.every((field) => (
      isRecord(field)
      && Number.isSafeInteger(field.contactFieldId)
      && field.contactFieldId > 0
      && (typeof field.value === 'string'
        || (Array.isArray(field.value) && field.value.every((value) => typeof value === 'string')))
    ));
}

function updatePortrait(body, state) {
  state.portraitFields = body.userPortrait.map((field) => {
    const current = state.portraitFields.find(
      (candidate) => candidate.contactFieldId === field.contactFieldId,
    );
    return current === undefined ? {
      contactFieldId: field.contactFieldId,
      contactFieldPivotId: field.contactFieldPivotId ?? '',
      name: typeof field.name === 'string' ? field.name : '',
      type: Number.isSafeInteger(field.type) ? field.type : 0,
      typeText: '',
      options: [],
      value: clone(field.value),
    } : {
      ...current,
      ...(field.contactFieldPivotId === undefined
        ? {}
        : { contactFieldPivotId: field.contactFieldPivotId }),
      ...(typeof field.name === 'string' ? { name: field.name } : {}),
      ...(Number.isSafeInteger(field.type) ? { type: field.type } : {}),
      value: clone(field.value),
    };
  });
}

async function handleWrite(request, response, pathname, state) {
  if (request.method !== 'PUT') {
    sendError(response, 405, 'method not allowed; use PUT', request.method, { Allow: 'PUT' });
    return;
  }
  const parsed = await readJsonBody(request);
  if (parsed.error) {
    sendError(response, parsed.error[0], parsed.error[1], request.method);
    return;
  }
  const body = parsed.value;

  if (pathname === '/sidebar/workContact/update') {
    if (!validContactUpdate(body, state)) {
      sendError(response, 422, 'invalid contact update body', request.method);
      return;
    }
    updateContact(body, state);
  } else if (pathname === '/sidebar/contactFieldPivot/update') {
    if (!validPortrait(body, state)) {
      sendError(response, 422, 'invalid userPortrait update body', request.method);
      return;
    }
    updatePortrait(body, state);
  } else if (pathname === '/sidebar/roomSop/logState') {
    if (body.id !== state.roomSop?.id) {
      sendError(response, 422, 'invalid room SOP update body', request.method);
      return;
    }
    state.roomSop.state = Number.isSafeInteger(body.state) && body.state !== 0 ? body.state : 1;
  }
  sendSuccess(response, [], request.method);
}

async function handleAPI(request, response, url, state, fixture) {
  if (request.headers.authorization !== `Bearer ${fixture.token}`) {
    sendError(response, 401, 'review bearer token required', request.method, {
      'WWW-Authenticate': 'Bearer realm="sidebar-local-review"',
    });
    return;
  }

  if (url.pathname === '/sidebar/common/upload') {
    sendError(response, 501, 'image upload is unavailable in local Sidebar review', request.method);
    return;
  }
  if (WRITE_ENDPOINTS.has(url.pathname)) {
    await handleWrite(request, response, url.pathname, state);
    return;
  }
  if (url.pathname === '/sidebar/contactBatchAdd/detail') {
    if (request.method !== 'GET') {
      sendError(response, 405, 'method not allowed; use GET', request.method, { Allow: 'GET' });
      return;
    }
    const statusValue = url.searchParams.get('status');
    const data = statusValue === null || statusValue === '1' || statusValue === '4'
      ? state.batch
      : { employeeName: state.batch.employeeName, list: [] };
    sendSuccess(response, data, request.method);
    return;
  }
  const fixtureKey = READ_ENDPOINTS.get(url.pathname);
  if (fixtureKey === undefined) {
    sendError(response, 404, 'unknown Sidebar review API', request.method);
    return;
  }
  if (request.method !== 'GET') {
    sendError(response, 405, 'method not allowed; use GET', request.method, { Allow: 'GET' });
    return;
  }
  sendSuccess(response, state[fixtureKey], request.method);
}

function escapeHTML(value) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('"', '&quot;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;');
}

function reviewRoutes(fixture, origin) {
  const agentId = String(fixture.agentId);
  const customer = `wxExternalUserid=external-user-1&agentId=${agentId}`;
  const state = Buffer.from(JSON.stringify({
    code: 200,
    msg: '',
    data: { token: fixture.token, expire: 7200 },
  }), 'utf8').toString('base64');
  const auth = new URL('/sidebar-app/auth', origin);
  auth.searchParams.set('agentId', agentId);
  auth.searchParams.set('state', state);
  auth.searchParams.set('target', `${origin}/sidebar-app/`);
  const callValues = Buffer.from(JSON.stringify({
    code: 200,
    data: { agentId, act: 'contact' },
  }), 'utf8').toString('base64');
  return [
    ['/', '工作台', `/sidebar-app/?agentId=${agentId}`],
    ['/auth', '授权回调', `${auth.pathname}${auth.search}`],
    ['/codeAuth', '兼容授权', `/sidebar-app/codeAuth?callValues=${encodeURIComponent(callValues)}`],
    ['/contact', '客户资料', `/sidebar-app/contact?${customer}`],
    ['/contact/editDetail', '编辑资料', `/sidebar-app/contact/editDetail?${customer}`],
    ['/contact/remark', '客户备注', `/sidebar-app/contact/remark?${customer}`],
    ['/contact/settingTag', '客户标签', `/sidebar-app/contact/settingTag?${customer}`],
    ['/contactBatchAdd', '批量加好友', `/sidebar-app/contactBatchAdd?batchId=9&agentId=${agentId}`],
    ['/contactSop', '个人 SOP', `/sidebar-app/contactSop?id=4&agentId=${agentId}`],
    ['/login', '登录', `/sidebar-app/login?agentId=${agentId}&target=%2F`],
    ['/medium', '素材库', `/sidebar-app/medium?agentId=${agentId}`],
    ['/roomSop', '群 SOP', `/sidebar-app/roomSop?id=5&agentId=${agentId}`],
  ];
}

function injectReviewBar(html, fixture, origin) {
  const links = reviewRoutes(fixture, origin).map(([route, label, href]) => (
    `<a data-review-route="${escapeHTML(route)}" href="${escapeHTML(href)}">${escapeHTML(label)}</a>`
  )).join('');
  const bar = `<aside id="sidebar-local-review" aria-label="本地验收路由"><style>
#sidebar-local-review{box-sizing:border-box;position:relative;z-index:0;width:100%;padding:8px 12px;background:#fff7db;border-bottom:1px solid #e6cf84;color:#5c4710;font:12px/1.4 system-ui,sans-serif}
#sidebar-local-review strong{display:block;margin-bottom:6px;font-size:12px}
#sidebar-local-review nav{display:flex;gap:6px;overflow-x:auto;padding-bottom:2px;scrollbar-width:thin}
#sidebar-local-review a{flex:0 0 auto;padding:4px 7px;border:1px solid #d8bd65;border-radius:999px;color:#5c4710;text-decoration:none;background:#fffdf6}
</style><strong>本地验收数据 · 重启后重置</strong><nav>${links}</nav></aside>`;
  if (/<body(?:\s[^>]*)?>/i.test(html)) {
    return html.replace(/<body(\s[^>]*)?>/i, (body) => `${body}${bar}`);
  }
  return `${bar}${html}`;
}

async function fileWithinRoot(distRoot, relativePath) {
  let decoded;
  try {
    decoded = decodeURIComponent(relativePath);
  } catch {
    return null;
  }
  if (decoded.includes('\\') || decoded.includes('\0')) return null;
  const segments = decoded.split('/').filter(Boolean);
  if (segments.some((segment) => segment === '..' || segment === '.')) return null;
  const candidate = path.resolve(distRoot, ...segments);
  let rootReal;
  let candidateReal;
  try {
    [rootReal, candidateReal] = await Promise.all([realpath(distRoot), realpath(candidate)]);
  } catch {
    return null;
  }
  const relative = path.relative(rootReal, candidateReal);
  if (relative.startsWith('..') || path.isAbsolute(relative)) return null;
  const details = await stat(candidateReal);
  return details.isFile() ? candidateReal : null;
}

async function serveAsset(request, response, url, distRoot, fixture, origin) {
  if (request.method !== 'GET' && request.method !== 'HEAD') {
    sendText(response, 405, 'method not allowed', request.method, { Allow: 'GET, HEAD' });
    return;
  }
  const isSidebarPath = url.pathname === '/sidebar-app'
    || url.pathname.startsWith('/sidebar-app/');
  let relativePath = isSidebarPath
    ? url.pathname.slice('/sidebar-app'.length).replace(/^\//, '')
    : url.pathname.replace(/^\//, '');
  if (relativePath === '') relativePath = 'index.html';
  let filename = await fileWithinRoot(distRoot, relativePath);
  if (filename === null && isSidebarPath && path.extname(relativePath) === '') {
    filename = await fileWithinRoot(distRoot, 'index.html');
  }
  if (filename === null) {
    sendText(response, 404, 'review asset not found', request.method);
    return;
  }

  let body = await readFile(filename);
  const extension = path.extname(filename).toLowerCase();
  if (path.basename(filename).toLowerCase() === 'index.html') {
    body = Buffer.from(injectReviewBar(body.toString('utf8'), fixture, origin), 'utf8');
  }
  send(
    response,
    200,
    body,
    MIME_TYPES.get(extension) ?? 'application/octet-stream',
    request.method,
  );
}

export function createSidebarReviewServer({
  distRoot,
  fixture,
  host = '127.0.0.1',
  port = 28083,
}) {
  if (!LOOPBACK_HOSTS.has(host.toLowerCase())) {
    throw new TypeError('Sidebar review server host must be 127.0.0.1 or localhost');
  }
  if (!Number.isSafeInteger(port) || port < 0 || port > 65535) {
    throw new TypeError('Sidebar review server port must be an integer between 0 and 65535');
  }
  if (typeof fixture?.agentId !== 'string' || typeof fixture?.token !== 'string') {
    throw new TypeError('Sidebar review fixture must provide string agentId and token values');
  }
  const resolvedDistRoot = path.resolve(distRoot);
  let state = clone(fixture);
  let server;
  server = createServer((request, response) => {
    void (async () => {
      if (!allowedHost(request.headers.host, server, port)) {
        sendText(response, 403, 'review host rejected', request.method);
        return;
      }
      const origin = `http://${request.headers.host}`;
      let url;
      try {
        url = new URL(request.url ?? '/', origin);
      } catch {
        sendText(response, 400, 'invalid request URL', request.method);
        return;
      }

      if (url.pathname === '/readyz') {
        if (request.method !== 'GET' && request.method !== 'HEAD') {
          sendText(response, 405, 'method not allowed', request.method, { Allow: 'GET, HEAD' });
          return;
        }
        sendText(response, 200, REQUEST_ID, request.method);
        return;
      }
      if (url.pathname === '/sidebar/agent/auth') {
        if (request.method !== 'GET') {
          sendText(response, 405, 'method not allowed', request.method, { Allow: 'GET' });
          return;
        }
        const location = reviewAuthLocation(url.href, fixture, origin);
        if (location === null) {
          sendText(response, 400, 'fixed review agent and safe Sidebar target required');
          return;
        }
        sendText(response, 302, 'redirecting to Sidebar review callback', request.method, {
          Location: location,
        });
        return;
      }
      if (url.pathname.startsWith('/sidebar/')) {
        await handleAPI(request, response, url, state, fixture);
        return;
      }
      await serveAsset(request, response, url, resolvedDistRoot, fixture, origin);
    })().catch((error) => {
      if (!response.headersSent) {
        sendText(response, 500, 'Sidebar review server error', request.method);
      } else {
        response.destroy(error);
      }
    });
  });

  return {
    server,
    get url() {
      return `http://${host}:${actualPort(server, port)}/sidebar-app/login?agentId=${encodeURIComponent(fixture.agentId)}&target=%2F`;
    },
    reset() {
      state = clone(fixture);
    },
  };
}

async function runCLI() {
  const fixturePath = new URL('../web/e2e/fixtures/sidebar-employee-review.json', import.meta.url);
  const fixture = JSON.parse(await readFile(fixturePath, 'utf8'));
  const distRoot = fileURLToPath(new URL('../web/apps/sidebar/dist/', import.meta.url));
  const review = createSidebarReviewServer({ distRoot, fixture });
  review.server.on('error', (error) => {
    console.error(`Sidebar review server failed: ${error.message}`);
    process.exitCode = 1;
  });
  review.server.listen(28083, '127.0.0.1', () => {
    console.log(`Sidebar local review: ${review.url}`);
  });
}

const invokedPath = process.argv[1] ? pathToFileURL(path.resolve(process.argv[1])).href : '';
if (invokedPath === import.meta.url) {
  await runCLI();
}
