import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const dockerfile = readFileSync(new URL('../Dockerfile', import.meta.url), 'utf8');
const syncScript = readFileSync(
  new URL('./sync_frontend_dist.sh', import.meta.url),
  'utf8',
);
const composeSmoke = readFileSync(
  new URL('./smoke_standalone_compose_app.sh', import.meta.url),
  'utf8',
);

test('Docker builds and copies both React applications', () => {
  assert.match(dockerfile, /pnpm --filter @mochat\/dashboard build/u);
  assert.match(dockerfile, /pnpm --filter @mochat\/saas-admin build/u);
  assert.match(
    dockerfile,
    /COPY --from=frontend-build \/src\/web\/apps\/dashboard\/dist \.\/web\/apps\/dashboard\/dist/u,
  );
  assert.match(
    dockerfile,
    /COPY --from=frontend-build \/src\/web\/apps\/saas-admin\/dist \.\/web\/apps\/saas-admin\/dist/u,
  );
});

test('the frontend sync command builds both React applications', () => {
  assert.match(syncScript, /pnpm --filter @mochat\/dashboard build/u);
  assert.match(syncScript, /pnpm --filter @mochat\/saas-admin build/u);
  assert.match(
    syncScript,
    /test -f "web\/apps\/saas-admin\/dist\/index\.html"/u,
  );
});

test('compose smoke distinguishes SaaS Admin from Dashboard fallback', () => {
  assert.match(
    composeSmoke,
    /assert dashboard_login != saas_admin/u,
  );
  assert.match(
    composeSmoke,
    /assert "\/saas-admin\/assets\/" in saas_admin/u,
  );
});
