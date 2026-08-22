import assert from 'node:assert/strict';
import { once } from 'node:events';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { after, before, test } from 'node:test';

import {
  createSidebarReviewServer,
  reviewAuthLocation,
} from './sidebar_review_server.mjs';

const fixturePath = new URL('../web/e2e/fixtures/sidebar-employee-review.json', import.meta.url);
const fixture = JSON.parse(await readFile(fixturePath, 'utf8'));

let distRoot;
let review;
let origin;
let port;

function request(pathname, { body, headers = {}, host, method = 'GET' } = {}) {
  return new Promise((resolve, reject) => {
    const request = http.request({
      hostname: '127.0.0.1',
      port,
      path: pathname,
      method,
      headers: {
        ...(host === undefined ? {} : { Host: host }),
        ...headers,
      },
    }, (response) => {
      const chunks = [];
      response.on('data', (chunk) => chunks.push(chunk));
      response.on('end', () => resolve({
        body: Buffer.concat(chunks).toString('utf8'),
        headers: response.headers,
        status: response.statusCode,
      }));
    });
    request.on('error', reject);
    if (body !== undefined) request.write(body);
    request.end();
  });
}

function authorized(pathname, options = {}) {
  return request(pathname, {
    ...options,
    headers: {
      Authorization: `Bearer ${fixture.token}`,
      ...options.headers,
    },
  });
}

function jsonBody(response) {
  return JSON.parse(response.body);
}

function jsonWrite(pathname, data) {
  return authorized(pathname, {
    body: JSON.stringify(data),
    headers: { 'Content-Type': 'application/json' },
    method: 'PUT',
  });
}

before(async () => {
  distRoot = await mkdtemp(path.join(os.tmpdir(), 'sidebar-review-'));
  await writeFile(path.join(distRoot, 'index.html'), '<!doctype html><html><body><div id="root"></div></body></html>');
  await writeFile(path.join(distRoot, 'asset.js'), 'globalThis.reviewAsset = true;');
  review = createSidebarReviewServer({ distRoot, fixture, host: '127.0.0.1', port: 0 });
  review.server.listen(0, '127.0.0.1');
  await once(review.server, 'listening');
  const address = review.server.address();
  assert(address && typeof address === 'object');
  port = address.port;
  origin = `http://127.0.0.1:${port}`;
});

after(async () => {
  if (review?.server.listening) {
    review.server.close();
    await once(review.server, 'close');
  }
  if (distRoot) await rm(distRoot, { recursive: true, force: true });
});

test('returns the fixed default review URL', () => {
  const unopened = createSidebarReviewServer({ distRoot, fixture });
  assert.equal(
    unopened.url,
    'http://127.0.0.1:28083/sidebar-app/login?agentId=7&target=%2F',
  );
  unopened.server.close();
  assert.equal(
    review.url,
    `${origin}/sidebar-app/login?agentId=7&target=%2F`,
  );
});

test('accepts only the fixed review agent and a safe Sidebar target', () => {
  assert.equal(
    reviewAuthLocation('/sidebar/agent/auth?agentId=8&target=%2F', fixture, origin),
    null,
  );
  for (const target of ['https://evil.test', '//evil.test', '/\\evil', '/outside', 'contact']) {
    const query = new URLSearchParams({ agentId: fixture.agentId, target });
    assert.equal(reviewAuthLocation(`/sidebar/agent/auth?${query}`, fixture, origin), null);
  }

  const query = new URLSearchParams({
    agentId: fixture.agentId,
    target: '/contact?wxExternalUserid=external-user-1&agentId=7',
  });
  const location = reviewAuthLocation(`/sidebar/agent/auth?${query}`, fixture, origin);
  assert(location);
  const callback = new URL(location);
  assert.equal(callback.origin, origin);
  assert.equal(callback.pathname, '/sidebar-app/auth');
  assert.equal(
    callback.searchParams.get('target'),
    `${origin}/sidebar-app/contact?wxExternalUserid=external-user-1&agentId=7`,
  );
  assert.deepEqual(
    JSON.parse(Buffer.from(callback.searchParams.get('state'), 'base64').toString('utf8')),
    { code: 200, msg: '', data: { token: fixture.token, expire: 7200 } },
  );
});

test('rejects requests with a non-review Host', async () => {
  const response = await request('/sidebar-app/', { host: `evil.test:${port}` });
  assert.equal(response.status, 403);
  assert.match(response.body, /host/i);
});

test('requires the fixed bearer token and fails unknown APIs closed', async () => {
  assert.equal((await request('/sidebar/workContact/show')).status, 401);
  assert.equal((await authorized('/sidebar/workContact/show', {
    headers: { Authorization: 'Bearer wrong-token' },
  })).status, 401);
  assert.equal((await authorized('/sidebar/unknown')).status, 404);
});

test('serves every explicit fixture read API in the success envelope', async () => {
  review.reset();
  const endpoints = new Map([
    ['/sidebar/workContact/detail?wxExternalUserid=external-user-1', fixture.contactDetail],
    ['/sidebar/workContact/show?contactId=23', fixture.contactSummary],
    ['/sidebar/workContact/track?contactId=23', fixture.tracks],
    ['/sidebar/contactFieldPivot/index?contactId=23', fixture.portraitFields],
    ['/sidebar/workContactTagGroup/index', fixture.tagGroups],
    ['/sidebar/workContactTag/allTag?groupId=3', fixture.tags],
    ['/sidebar/contactBatchAdd/detail?batchId=9&status=1', fixture.batch],
    ['/sidebar/contactSop/getSopInfo?id=4', fixture.contactSop],
    ['/sidebar/contactSop/getSopTipInfo?contactId=23', fixture.contactSopTips],
    ['/sidebar/roomSop/getSopInfo?id=5', fixture.roomSop],
    ['/sidebar/mediumGroup/index', fixture.mediumGroups],
    ['/sidebar/medium/index?page=1', fixture.mediumPage],
  ]);
  for (const [pathname, data] of endpoints) {
    const response = await authorized(pathname);
    assert.equal(response.status, 200, pathname);
    assert.deepEqual(jsonBody(response), {
      code: 200,
      msg: 'ok',
      data,
      requestId: 'sidebar-local-review',
    }, pathname);
  }
  const filtered = jsonBody(await authorized('/sidebar/contactBatchAdd/detail?batchId=9&status=3'));
  assert.deepEqual(filtered.data, { employeeName: fixture.batch.employeeName, list: [] });
});

test('returns explicit 4xx/501 responses for invalid methods, bodies, and unsupported upload', async () => {
  assert.equal((await authorized('/sidebar/workContact/show', { method: 'POST' })).status, 405);
  assert.equal((await authorized('/sidebar/workContact/update', {
    body: '{',
    headers: { 'Content-Type': 'application/json' },
    method: 'PUT',
  })).status, 400);
  assert.equal((await authorized('/sidebar/workContact/update', {
    body: JSON.stringify({ contactId: 23, remark: 'new' }),
    headers: { 'Content-Type': 'text/plain' },
    method: 'PUT',
  })).status, 415);
  assert.equal((await authorized('/sidebar/workContact/update', {
    body: JSON.stringify({ contactId: 23 }),
    headers: { 'Content-Type': 'application/json' },
    method: 'PUT',
  })).status, 422);
  assert.equal((await authorized('/sidebar/workContact/update', {
    body: JSON.stringify({ contactId: 23, tag: [999] }),
    headers: { 'Content-Type': 'application/json' },
    method: 'PUT',
  })).status, 422);
  assert.equal((await authorized('/sidebar/common/upload', { method: 'POST' })).status, 501);
});

test('injects the review marker and links for all 12 Sidebar routes without rewriting the build', async () => {
  const before = await readFile(path.join(distRoot, 'index.html'), 'utf8');
  const response = await request('/sidebar-app/contact?wxExternalUserid=external-user-1&agentId=7');
  assert.equal(response.status, 200);
  assert.match(response.body, /本地验收数据 · 重启后重置/);
  assert.equal((response.body.match(/data-review-route=/g) ?? []).length, 12);
  assert.match(response.body, /wxExternalUserid=external-user-1&amp;agentId=7/);
  assert.match(response.body, /batchId=9&amp;agentId=7/);
  assert.match(response.body, /id=4&amp;agentId=7/);
  assert.match(response.body, /id=5&amp;agentId=7/);
  assert.equal(await readFile(path.join(distRoot, 'index.html'), 'utf8'), before);
});

test('persists remark, description, and appended tags until reset', async () => {
  review.reset();
  assert.equal((await jsonWrite('/sidebar/workContact/update', {
    contactId: 23,
    remark: '新备注',
    description: '新描述',
    tag: [9],
  })).status, 200);
  const current = jsonBody(await authorized('/sidebar/workContact/show?contactId=23')).data;
  assert.equal(current.remark, '新备注');
  assert.equal(current.description, '新描述');
  assert.deepEqual(current.tag, [
    { tagId: 7, tagName: '高意向' },
    { tagId: 9, tagName: '待回访' },
  ]);

  review.reset();
  assert.deepEqual(
    jsonBody(await authorized('/sidebar/workContact/show?contactId=23')).data,
    fixture.contactSummary,
  );
});

test('persists portrait userPortrait values until reset', async () => {
  review.reset();
  const userPortrait = fixture.portraitFields.map((field) => ({
    contactFieldPivotId: field.contactFieldPivotId,
    contactFieldId: field.contactFieldId,
    name: field.name,
    type: field.type,
    value: '已更新',
  }));
  assert.equal((await jsonWrite('/sidebar/contactFieldPivot/update', {
    contactId: 23,
    userPortrait,
  })).status, 200);
  assert.equal(
    jsonBody(await authorized('/sidebar/contactFieldPivot/index?contactId=23')).data[0].value,
    '已更新',
  );

  review.reset();
  assert.deepEqual(
    jsonBody(await authorized('/sidebar/contactFieldPivot/index?contactId=23')).data,
    fixture.portraitFields,
  );
});

test('persists a nonzero room SOP completion state until reset', async () => {
  review.reset();
  assert.equal((await jsonWrite('/sidebar/roomSop/logState', { id: 5 })).status, 200);
  assert.notEqual(
    jsonBody(await authorized('/sidebar/roomSop/getSopInfo?id=5')).data.state,
    0,
  );

  review.reset();
  assert.deepEqual(
    jsonBody(await authorized('/sidebar/roomSop/getSopInfo?id=5')).data,
    fixture.roomSop,
  );
});

test('serves static assets, provides Sidebar SPA fallback, and exposes local readiness', async () => {
  const asset = await request('/asset.js');
  assert.equal(asset.status, 200);
  assert.equal(asset.body, 'globalThis.reviewAsset = true;');

  const fallback = await request('/sidebar-app/roomSop?id=5&agentId=7');
  assert.equal(fallback.status, 200);
  assert.match(fallback.body, /<div id="root"><\/div>/);
  assert.match(fallback.body, /本地验收数据/);

  const ready = await request('/readyz');
  assert.equal(ready.status, 200);
  assert.match(ready.body, /sidebar-local-review/);
});
