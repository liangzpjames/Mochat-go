import assert from 'node:assert/strict';
import { test } from 'node:test';
import http from 'node:http';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const script = fileURLToPath(new URL('./phase35_acceptance_data.ps1', import.meta.url));

function runPowerShell(args) {
  return new Promise((resolve, reject) => {
    const child = spawn('powershell', ['-NoProfile', '-File', script, ...args], { stdio: ['ignore', 'pipe', 'pipe'] });
    let stdout = '', stderr = '';
    child.stdout.on('data', (value) => { stdout += value; }); child.stderr.on('data', (value) => { stderr += value; });
    child.on('error', reject); child.on('exit', (code) => code === 0 ? resolve(stdout) : reject(new Error(stderr || stdout)));
  });
}

test('create verify cleanup use only the isolated scoped lifecycle API', async () => {
  const requests = [];
  const server = http.createServer((request, response) => {
    let body = ''; request.on('data', (chunk) => { body += chunk; }); request.on('end', () => {
      requests.push({ method: request.method, url: request.url, environment: request.headers['x-phase35-acceptance-environment'], body });
      response.writeHead(200, { 'content-type': 'application/json' }); response.end(JSON.stringify({ prefix: 'P35-ACCEPT-', count: 3 }));
    });
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address();
  try {
    for (const action of ['create', 'verify', 'cleanup']) await runPowerShell(['-Action', action, '-BaseUrl', `http://127.0.0.1:${port}`, '-EnvironmentId', 'P35-ACCEPT-MOCK', '-CorpId', '42', '-AllowIsolatedEnvironment']);
  } finally { await new Promise((resolve) => server.close(resolve)); }
  assert.deepEqual(requests.map((item) => item.method), ['POST', 'GET', 'DELETE']);
  for (const request of requests) { assert.match(request.url, /corpId=42&prefix=P35-ACCEPT-/); assert.equal(request.environment, 'P35-ACCEPT-MOCK'); }
  assert.match(requests[0].body, /P35-ACCEPT-MOCK/); assert.match(requests[2].body, /P35-ACCEPT-/);
});
