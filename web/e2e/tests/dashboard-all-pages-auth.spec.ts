import { mkdir, writeFile } from 'node:fs/promises';
import { isAbsolute, resolve } from 'node:path';

import { expect, test, type Page, type Response } from '@playwright/test';

import manifest from '../../apps/dashboard/src/benchmark/manifest.json' with { type: 'json' };
import { validateDashboardAllPagesEvidenceFiles } from '../../../scripts/validate_dashboard_all_pages_evidence.mjs';
import { dashboardPageActions } from '../fixtures/dashboard-page-actions';

type LiveFixture = {
  phone: string;
  password: string;
  userName: string;
  corpName: string;
};

type ResponseStatus = {
  method: string;
  url: string;
  status: number;
  machineCode?: string;
};

type ExpectedResponse = ResponseStatus & { route: string };

type PageEvidence = {
  route: string;
  action: string;
  titleVisible: boolean;
  responseStatuses: ResponseStatus[];
  unexpectedResponses: ResponseStatus[];
  consoleErrors: string[];
  pageErrors: string[];
  screenshot: string;
  returnedToIndex: boolean;
  userNameVisible: boolean;
  corpNameVisible: boolean;
};

const liveBase = (process.env.MOCHAT_E2E_LIVE_BASE ?? '').replace(/\/$/, '');
const liveFixture = process.env.MOCHAT_E2E_ALL_PAGES_FIXTURE_JSON
  ? JSON.parse(process.env.MOCHAT_E2E_ALL_PAGES_FIXTURE_JSON) as LiveFixture
  : undefined;
const pages = manifest.pages.map(({ path, title }) => ({ route: path, title }));
const actions = new Map(dashboardPageActions.map((action) => [action.route, action]));
const evidenceDir = process.env.MOCHAT_E2E_EVIDENCE_DIR;

// Provider limitations are represented by successful capability envelopes today.
// Add future exceptions here only as an exact page route + API path + status + machine code tuple.
const expectedProviderResponses: readonly ExpectedResponse[] = [];

function redacted(message: string): string {
  return message
    .replace(/Bearer\s+[A-Za-z0-9._~+/-]+=*/gi, 'Bearer [REDACTED]')
    .replace(/([?&](?:token|authorization)=)[^&\s]+/gi, '$1[REDACTED]');
}

function machineCode(value: unknown): string | undefined {
  if (value === null || typeof value !== 'object') return undefined;
  const code = (value as { errorCode?: unknown }).errorCode;
  return typeof code === 'string' ? code : undefined;
}

async function captureResponse(response: Response, origin: string): Promise<ResponseStatus | undefined> {
  const url = new URL(response.url());
  if (url.origin !== origin || !url.pathname.startsWith('/dashboard/')) return undefined;
  const result: ResponseStatus = { method: response.request().method(), url: url.pathname, status: response.status() };
  if (result.status >= 400 && (response.headers()['content-type'] ?? '').includes('application/json')) {
    const code = machineCode(await response.json().catch(() => undefined));
    if (code) result.machineCode = code;
  }
  return result;
}

function isExpectedProviderResponse(route: string, response: ResponseStatus): boolean {
  if (response.status === 401 || response.machineCode === 'UNAUTHORIZED'
    || response.machineCode === 'TENANT_ACCESS_DENIED'
    || response.machineCode === 'DASHBOARD_PERMISSION_DENIED') return false;
  return expectedProviderResponses.some((expected) => expected.route === route
    && expected.method === response.method
    && expected.url === response.url
    && expected.status === response.status
    && (expected.machineCode ?? null) === (response.machineCode ?? null));
}

async function login(page: Page, fixture: LiveFixture): Promise<void> {
  await page.goto(`${liveBase}/login`, { waitUntil: 'domcontentloaded' });
  await page.getByLabel('手机号', { exact: true }).fill(fixture.phone);
  await page.getByLabel('密码', { exact: true }).fill(fixture.password);
  const responsePromise = page.waitForResponse((response) => response.url().includes('/dashboard/user/auth'));
  await page.getByRole('button', { name: /登录/ }).click();
  expect((await responsePromise).status()).toBe(200);
  await page.waitForURL((url) => url.pathname === '/index');
  await assertIdentity(page, fixture);
}

async function assertIdentity(page: Page, fixture: LiveFixture): Promise<void> {
  await expect(page.locator('.dashboard-account-actions')).toContainText(fixture.userName);
  await expect(page.locator('.dashboard-corp-badge')).toContainText(fixture.corpName);
}

test('all 53 manifest routes have one explicit clickable action contract', () => {
  expect(machineCode({ code: 403, errorCode: 'TENANT_ACCESS_DENIED' })).toBe('TENANT_ACCESS_DENIED');
  const manifestRoutes = pages.map(({ route }) => route);
  const registryRoutes = dashboardPageActions.map(({ route }) => route);
  expect(manifestRoutes).toHaveLength(53);
  expect(new Set(manifestRoutes).size).toBe(53);
  expect(registryRoutes).toHaveLength(53);
  expect(new Set(registryRoutes).size).toBe(53);
  expect([...registryRoutes].sort()).toEqual([...manifestRoutes].sort());
  for (const action of dashboardPageActions) {
    expect(action.action, `${action.route} must locate a real clickable control`).toMatch(/^(?:button|link):\S/);
    expect(typeof action.run).toBe('function');
  }
});

test('real login safely clicks every manifest page without auth regressions @live', async ({ page }) => {
  test.setTimeout(600_000);
  test.skip(!liveBase || !liveFixture || !evidenceDir, 'SKIP: set MOCHAT_E2E_LIVE_BASE, MOCHAT_E2E_ALL_PAGES_FIXTURE_JSON and MOCHAT_E2E_EVIDENCE_DIR for live 53-page acceptance');
  expect(isAbsolute(evidenceDir!), 'MOCHAT_E2E_EVIDENCE_DIR must be an absolute path').toBe(true);
  const fixture = liveFixture!;
  const origin = new URL(liveBase).origin;
  await mkdir(evidenceDir!, { recursive: true });
  await login(page, fixture);

  const responseTasks: Array<Promise<ResponseStatus | undefined>> = [];
  const consoleErrors: string[] = [];
  const pageErrors: string[] = [];
  page.on('response', (response) => responseTasks.push(captureResponse(response, origin)));
  page.on('console', (message) => { if (message.type() === 'error') consoleErrors.push(redacted(message.text())); });
  page.on('pageerror', (error) => pageErrors.push(redacted(error.message)));

  const evidence: PageEvidence[] = [];
  for (const current of pages) {
    const action = actions.get(current.route);
    expect(action, `${current.route} is missing an action`).toBeDefined();
    const responseOffset = responseTasks.length;
    const consoleOffset = consoleErrors.length;
    const pageErrorOffset = pageErrors.length;

    // Collection starts before navigation and remains active through the registered safe click.
    await page.goto(`${liveBase}${current.route}`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('main h1').first()).toContainText(current.title, { timeout: 15_000 });
    await expect(page.locator('.dashboard-content > *').first()).toBeVisible();
    await page.waitForLoadState('networkidle');
    await action!.run(page);
    await page.waitForLoadState('networkidle');

    const responseStatuses = (await Promise.all(responseTasks.slice(responseOffset)))
      .filter((response): response is ResponseStatus => response !== undefined);
    const unexpectedResponses = responseStatuses.filter((response) => response.status >= 400
      && !isExpectedProviderResponse(current.route, response));
    const routeConsoleErrors = consoleErrors.slice(consoleOffset);
    const routePageErrors = pageErrors.slice(pageErrorOffset);
    const screenshot = `${current.route === '/index' ? 'index' : current.route.slice(1).replaceAll('/', '-')}.png`;
    await page.screenshot({ path: resolve(evidenceDir!, screenshot), fullPage: true });

    await page.goto(`${liveBase}/index`, { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(new RegExp(`${liveBase.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}/index(?:[?#]|$)`));
    await assertIdentity(page, fixture);
    await page.waitForLoadState('networkidle');
    expect(unexpectedResponses, `${current.route} returned an unexpected HTTP response`).toEqual([]);
    expect(routeConsoleErrors, `${current.route} emitted console errors`).toEqual([]);
    expect(routePageErrors, `${current.route} emitted page errors`).toEqual([]);

    evidence.push({
      route: current.route,
      action: action!.action,
      titleVisible: true,
      responseStatuses,
      unexpectedResponses,
      consoleErrors: routeConsoleErrors,
      pageErrors: routePageErrors,
      screenshot,
      returnedToIndex: true,
      userNameVisible: true,
      corpNameVisible: true,
    });
  }

  expect(evidence).toHaveLength(53);
  await validateDashboardAllPagesEvidenceFiles({
    manifestRoutes: pages.map(({ route }) => route),
    actionRegistry: dashboardPageActions.map(({ route, action }) => ({ route, action })),
    evidence,
    expectedResponses: expectedProviderResponses,
  }, { evidenceRoot: evidenceDir! });
  await writeFile(resolve(evidenceDir!, 'evidence.json'), `${JSON.stringify({
    generatedAt: new Date().toISOString(),
    baseOrigin: origin,
    pages: evidence,
  }, null, 2)}\n`, 'utf8');
});
