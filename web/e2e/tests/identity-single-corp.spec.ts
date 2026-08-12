import { expect, test, type APIRequestContext, type Page } from '@playwright/test';
import { parseStoredToken } from '../../../scripts/identity_single_corp_auth_storage.mjs';
import { resolveFixtureCredentials, validateFixture } from '../../../scripts/validate_identity_single_corp_e2e_fixture.mjs';

type LoginCredentialRef = {
  loginEnvKey: string;
  passwordEnvKey: string;
};

type ExpectedSaasGovernance = {
  dashboardProvisionOperationId: number;
  dashboardProvisionRequestId: string;
};

type IdentitySingleCorpFixture = {
  saasAdmin: LoginCredentialRef;
  dashboardSuperAdmin: LoginCredentialRef;
  dashboardOrdinary: LoginCredentialRef;
  tenantDenied: LoginCredentialRef;
  secondTenantAdmin: LoginCredentialRef;
  expectedTenantId: number;
  expectedCorpId: number;
  expectedWxCorpId: string;
  tenantCorpBindings: Array<{ tenantId: number; corpId: number; status?: string }>;
  expectedSaasGovernance: ExpectedSaasGovernance;
};

type JsonObject = Record<string, unknown>;
type NetworkAudit = {
  consoleErrors: string[];
  pageErrors: string[];
  unexpectedResponses: string[];
  pendingResponseChecks: Array<Promise<void>>;
};

const liveBase = (process.env.MOCHAT_E2E_LIVE_BASE || '').replace(/\/$/, '');
const liveFixture = readFixture();
const dashboardTokenKey = 'mochat_dashboard_token';
const saasTokenKey = 'mochat_saas_admin_token';

function readFixture(): IdentitySingleCorpFixture | undefined {
  const source = process.env.MOCHAT_E2E_IDENTITY_SINGLE_CORP_FIXTURE_JSON;
  if (!source) return undefined;
  const value: unknown = JSON.parse(source);
  validateFixture(value);
  return value as IdentitySingleCorpFixture;
}

function requireLiveFixture(): IdentitySingleCorpFixture {
  if (!liveFixture) throw new Error('identity single-corp live fixture is not configured');
  return liveFixture;
}

function credentialsFor(reference: LoginCredentialRef, accountName: string) {
  try {
    return resolveFixtureCredentials(reference, process.env);
  } catch (error) {
    throw new Error(`${accountName} credential environment keys are not set: ${error instanceof Error ? error.message : 'invalid reference'}`);
  }
}

function asObject(value: unknown): JsonObject {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? value as JsonObject
    : {};
}

function responseData(value: unknown): JsonObject {
  const body = asObject(value);
  return asObject(body.data);
}

function machineCode(value: unknown): string {
  const body = asObject(value);
  for (const key of ['machineCode', 'errorCode']) {
    if (typeof body[key] === 'string' && body[key] !== '') return body[key];
  }
  const data = asObject(body.data);
  for (const key of ['machineCode', 'errorCode']) {
    if (typeof data[key] === 'string' && data[key] !== '') return data[key];
  }
  return '';
}

function tokenHeader(token: string): string {
  return /^Bearer\s/i.test(token) ? token : `Bearer ${token}`;
}

function tokenClaims(token: string): JsonObject {
  const compact = token.replace(/^Bearer\s+/i, '');
  const payload = compact.split('.')[1];
  if (!payload) throw new Error('live login response did not contain a JWT payload');
  const normalized = payload.replaceAll('-', '+').replaceAll('_', '/').padEnd(Math.ceil(payload.length / 4) * 4, '=');
  return asObject(JSON.parse(Buffer.from(normalized, 'base64').toString('utf8')));
}

function attachNetworkAudit(page: Page): NetworkAudit {
  const audit: NetworkAudit = { consoleErrors: [], pageErrors: [], unexpectedResponses: [], pendingResponseChecks: [] };
  page.on('console', (message) => {
    if (message.type() === 'error') audit.consoleErrors.push(message.text());
  });
  page.on('pageerror', (error) => audit.pageErrors.push(error.message));
  page.on('response', (response) => {
    if (response.status() < 400) return;
    const pathname = new URL(response.url()).pathname;
    const check = (async () => {
      if (response.status() === 403 && pathname === '/dashboard/access/profile') {
        try {
          await response.finished();
          const body = await response.json() as unknown;
          if (machineCode(body) !== 'CORP_CONFIGURATION_REQUIRED') {
            audit.unexpectedResponses.push(`${response.status()} ${pathname} machineCode=${machineCode(body) || 'missing'}`);
          }
        } catch {
          audit.unexpectedResponses.push(`${response.status()} ${pathname} machineCode=unreadable`);
        }
        return;
      }
      if (response.status() === 404 && pathname === '/favicon.ico') return;
      audit.unexpectedResponses.push(`${response.status()} ${pathname}`);
    })();
    audit.pendingResponseChecks.push(check);
  });
  return audit;
}

async function assertCleanNetwork(audit: NetworkAudit, label: string) {
  await Promise.all(audit.pendingResponseChecks);
  expect(audit.consoleErrors, `${label} console errors`).toEqual([]);
  expect(audit.pageErrors, `${label} page errors`).toEqual([]);
  expect(audit.unexpectedResponses, `${label} unexpected HTTP errors`).toEqual([]);
}

async function readStorage(page: Page, key: string, realm: 'dashboard' | 'saas'): Promise<string> {
  const value = await page.evaluate((storageKey) => window.localStorage.getItem(storageKey), key);
  return tokenHeader(parseStoredToken(value, realm));
}

async function loginSaaS(page: Page, fixture: IdentitySingleCorpFixture): Promise<string> {
  const account = credentialsFor(fixture.saasAdmin, 'saasAdmin');
  await page.goto(`${liveBase}/saas/login`);
  await page.getByLabel('登录名').fill(account.login);
  await page.getByLabel('密码').fill(account.password);
  const loginResponse = page.waitForResponse((response) => (
    new URL(response.url()).pathname === '/saas/auth/login' && response.request().method() === 'POST'
  ));
  await page.getByRole('button', { name: '登录' }).click();
  expect((await loginResponse).status()).toBe(200);
  await expect.poll(() => page.url()).toMatch(/\/saas-admin\/?(?:\?.*)?$/);
  return readStorage(page, saasTokenKey, 'saas');
}

async function loginDashboard(page: Page, fixture: IdentitySingleCorpFixture, accountName: 'dashboardSuperAdmin' | 'dashboardOrdinary'): Promise<string> {
  const account = credentialsFor(fixture[accountName], accountName);
  await page.goto(`${liveBase}/login`);
  await page.getByLabel('手机号').fill(account.login);
  await page.getByLabel('密码').fill(account.password);
  const loginResponse = page.waitForResponse((response) => (
    new URL(response.url()).pathname === '/dashboard/user/auth' && response.request().method() === 'POST'
  ));
  await page.getByRole('button', { name: '登录' }).click();
  expect((await loginResponse).status()).toBe(200);
  await expect.poll(() => page.url()).not.toMatch(/\/login(?:\?|$)/);
  return readStorage(page, dashboardTokenKey, 'dashboard');
}

async function loginDashboardByAPI(request: APIRequestContext, fixture: IdentitySingleCorpFixture, accountName: 'tenantDenied' | 'secondTenantAdmin') {
  const account = credentialsFor(fixture[accountName], accountName);
  return request.post(`${liveBase}/dashboard/user/auth`, { data: { phone: account.login, password: account.password } });
}

async function getCompanyProfile(request: APIRequestContext, token: string) {
  const response = await request.get(`${liveBase}/dashboard/company/profile`, { headers: { Authorization: tokenHeader(token) } });
  const body = await response.json() as JsonObject;
  return { response, body, data: responseData(body) };
}

async function assertNoSaaSLinks(page: Page) {
  const hrefs = await page.locator('a[href]').evaluateAll((links) => links.map((link) => (link as HTMLAnchorElement).href));
  expect(hrefs.some((href) => /saas|18081|subscription|package/i.test(href))).toBe(false);
}

async function assertSingleCompanySurface(page: Page, viewportWidth: number) {
  await page.goto(`${liveBase}/company-setting/website`);
  await expect(page.getByText('唯一企业资料', { exact: true }).first()).toBeVisible();
  const bodyText = await page.locator('body').innerText();
  expect(bodyText).not.toMatch(/新建企业|新增企业|企业列表|企业选择|切换企业|删除企业|SaaS/);
  expect(await page.locator('select').count()).toBe(0);
  const profile = await page.evaluate(() => ({
    text: document.body.innerText,
    scrollWidth: document.documentElement.scrollWidth,
  }));
  expect(profile.text).toContain('唯一企业绑定');
  expect(profile.scrollWidth).toBeLessThanOrEqual(viewportWidth);
  await assertNoSaaSLinks(page);
}

async function assertCredentialConfirmationDoesNotMutate(page: Page) {
  const secretMarker = `live-e2e-${Date.now()}-${Math.random().toString(36).slice(2)}`;
  const input = page.getByLabel('员工密钥');
  await input.fill(secretMarker);
  let protectedWriteCount = 0;
  const requestListener = (request: { method(): string; url(): string }) => {
    if (request.method() !== 'GET' && request.url().includes('/dashboard/company/')) protectedWriteCount += 1;
  };
  page.on('request', requestListener);
  await page.getByRole('button', { name: '保存企业微信凭据' }).click();
  await expect(page.getByText('确认保存企业微信凭据？', { exact: true })).toBeVisible();
  expect(protectedWriteCount).toBe(0);
  await page.getByRole('button', { name: '取消' }).click();
  page.off('request', requestListener);
  await input.fill('');
  await page.reload();
  await expect(page.getByLabel('员工密钥')).toHaveValue('');
  expect(await page.locator('body').innerText()).not.toContain(secretMarker);
}

test.describe('Identity single-corp live acceptance', () => {
  test.describe.configure({ mode: 'serial' });
  test.skip(!liveBase || !liveFixture, 'SKIP: set MOCHAT_E2E_LIVE_BASE and MOCHAT_E2E_IDENTITY_SINGLE_CORP_FIXTURE_JSON for real live acceptance');

  test('real SaaS and Dashboard logins issue distinct realms and SaaS activates one Dashboard administrator', async ({ page, browser, request }) => {
    test.setTimeout(120_000);
    const fixture = requireLiveFixture();
    const saasAudit = attachNetworkAudit(page);
    const saasToken = await loginSaaS(page, fixture);
    const saasClaims = tokenClaims(saasToken);
    expect(saasClaims.realm).toBe('saas_admin');
    expect(saasClaims.aud).toBe('mochat-saas-admin');

    const dashboardContext = await browser.newContext();
    const dashboardPage = await dashboardContext.newPage();
    const dashboardAudit = attachNetworkAudit(dashboardPage);
    const dashboardToken = await loginDashboard(dashboardPage, fixture, 'dashboardSuperAdmin');
    const dashboardClaims = tokenClaims(dashboardToken);
    expect(dashboardClaims.realm).toBe('dashboard');
    expect(dashboardClaims.aud).toBe('mochat-dashboard');
    expect(saasToken === dashboardToken).toBe(false);

    const saasWithDashboard = await request.get(`${liveBase}/dashboard/saasAdmin/accessProfile`, { headers: { Authorization: dashboardToken } });
    expect(saasWithDashboard.status()).toBe(401);
    const dashboardWithSaaS = await request.get(`${liveBase}/dashboard/company/profile`, { headers: { Authorization: saasToken } });
    expect(dashboardWithSaaS.status()).toBe(401);

    const tenantDetail = await request.get(`${liveBase}/dashboard/saasAdmin/tenant?tenantId=${fixture.expectedTenantId}`, { headers: { Authorization: saasToken } });
    expect(tenantDetail.status()).toBe(200);
    const tenantDetailData = responseData(await tenantDetail.json() as JsonObject);
    const tenant = asObject(tenantDetailData.tenant);
    expect(Number(tenantDetailData.tenantId || tenant.tenantId)).toBe(fixture.expectedTenantId);

    const governanceExpectation = fixture.expectedSaasGovernance;
    const provisionAuditResponse = await request.get(
      `${liveBase}/dashboard/saasAdmin/operations?tenantId=${fixture.expectedTenantId}`
        + `&action=saas.admin.dashboard_tenant.provision&targetType=tenant`
        + `&keyword=${encodeURIComponent(governanceExpectation.dashboardProvisionRequestId)}&limit=20`,
      { headers: { Authorization: saasToken } },
    );
    expect(provisionAuditResponse.status()).toBe(200);
    const provisionAuditData = responseData(await provisionAuditResponse.json() as JsonObject);
    const provisionAudits = Array.isArray(provisionAuditData.operations) ? provisionAuditData.operations : [];
    expect(provisionAudits).toHaveLength(1);
    const provisionAudit = asObject(provisionAudits[0]);
    expect(Number(provisionAudit.id)).toBe(governanceExpectation.dashboardProvisionOperationId);
    expect(Number(provisionAudit.tenantId)).toBe(fixture.expectedTenantId);
    expect(provisionAudit.action).toBe('saas.admin.dashboard_tenant.provision');
    expect(provisionAudit.targetType).toBe('tenant');
    expect(String(provisionAudit.targetId)).toBe(String(fixture.expectedTenantId));
    const provisionAfter = asObject(provisionAudit.after);
    expect(Number(provisionAfter.tenantId)).toBe(fixture.expectedTenantId);
    expect(Number(provisionAfter.corpId)).toBe(fixture.expectedCorpId);
    expect(provisionAfter.activationIssued).toBe(true);
    expect(provisionAfter.isSuperAdmin).toBe(true);
    expect(provisionAfter.requestId).toBe(governanceExpectation.dashboardProvisionRequestId);

    const governance = await request.get(`${liveBase}/dashboard/saasAdmin/tenants/${fixture.expectedTenantId}/dashboard-admins`, { headers: { Authorization: saasToken } });
    expect(governance.status()).toBe(200);
    const governanceData = responseData(await governance.json() as JsonObject);
    const identities = Array.isArray(governanceData.identities) ? governanceData.identities : [];
    const activeSuperAdmins = identities.filter((identity) => {
      const candidate = asObject(identity);
      return candidate.isSuperAdmin === true
        && Number(candidate.userStatus) === 1
        && Number(candidate.identityStatus) === 1
        && typeof candidate.activatedAt === 'string'
        && candidate.activatedAt !== '';
    });
    expect(activeSuperAdmins).toHaveLength(1);
    expect(identities).toHaveLength(1);
    expect(Number(asObject(activeSuperAdmins[0]).id)).toBe(Number(provisionAfter.dashboardUserId));
    expect(String(asObject(activeSuperAdmins[0]).activatedAt)).not.toBe('');

    await dashboardContext.close();
    await assertCleanNetwork(saasAudit, 'SaaS login');
    await assertCleanNetwork(dashboardAudit, 'Dashboard login');
  });

  test('real Dashboard tenant isolation, pending-to-verified company flow, sync state, desktop and 390px surface', async ({ page, browser, request }) => {
    test.setTimeout(240_000);
    const fixture = requireLiveFixture();
    const audit = attachNetworkAudit(page);
    const dashboardToken = await loginDashboard(page, fixture, 'dashboardSuperAdmin');

    const deniedResponse = await loginDashboardByAPI(request, fixture, 'tenantDenied');
    expect(deniedResponse.status()).toBe(403);
    const deniedBody = asObject(await deniedResponse.json());
    expect(machineCode(deniedBody)).toBe('TENANT_ACCESS_DENIED');

    const secondTenantResponse = await loginDashboardByAPI(request, fixture, 'secondTenantAdmin');
    expect(secondTenantResponse.status()).toBe(200);
    const secondTenantData = responseData(await secondTenantResponse.json() as JsonObject);
    const secondTenantTokenValue = typeof secondTenantData.token === 'string'
      ? secondTenantData.token
      : secondTenantResponse.headers()['x-dashboard-token'] || '';
    const secondTenantToken = tokenHeader(secondTenantTokenValue);
    expect(secondTenantToken.length > 'Bearer '.length).toBe(true);
    const secondProfile = await getCompanyProfile(request, secondTenantToken);
    expect(secondProfile.response.status()).toBe(200);
    expect(Number(secondProfile.data.tenantId)).not.toBe(fixture.expectedTenantId);

    const ordinaryContext = await browser.newContext();
    const ordinaryPage = await ordinaryContext.newPage();
    const ordinaryAudit = attachNetworkAudit(ordinaryPage);
    const ordinaryToken = await loginDashboard(ordinaryPage, fixture, 'dashboardOrdinary');
    const ordinaryCompany = await getCompanyProfile(request, ordinaryToken);
    expect(ordinaryCompany.response.status()).toBe(403);
    const ordinaryBody = ordinaryCompany.body;
    expect(machineCode(ordinaryBody)).toBe('DASHBOARD_PERMISSION_DENIED');
    await ordinaryContext.close();
    await assertCleanNetwork(ordinaryAudit, 'ordinary Dashboard login');

    const initialProfile = await getCompanyProfile(request, dashboardToken);
    expect(initialProfile.response.status()).toBe(200);
    expect(Number(initialProfile.data.tenantId)).toBe(fixture.expectedTenantId);
    expect(Number(initialProfile.data.corpId)).toBe(fixture.expectedCorpId);
    expect(initialProfile.data.bindingStatus).toBe('pending');
    expect(initialProfile.data.wxCorpId || '').toBe('');

    await page.goto(`${liveBase}/index`);
    await expect(page).toHaveURL(/\/company-setting\/website$/);
    await expect(page.getByText('唯一企业资料', { exact: true }).first()).toBeVisible();
    await page.setViewportSize({ width: 1440, height: 900 });
    await assertSingleCompanySurface(page, 1440);

    const verifyInput = page.getByLabel('待验证 CorpID');
    await verifyInput.fill(fixture.expectedWxCorpId);
    const verifyResponse = page.waitForResponse((response) => (
      new URL(response.url()).pathname === '/dashboard/company/verify' && response.request().method() === 'POST'
    ));
    await page.getByRole('button', { name: '验证企业微信' }).click();
    await expect(page.getByText('确认验证企业微信？', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: '确认' }).click();
    expect((await verifyResponse).status()).toBe(200);
    await expect.poll(async () => (await getCompanyProfile(request, dashboardToken)).data.bindingStatus, { timeout: 60_000 }).toBe('verified');

    const verifiedProfile = await getCompanyProfile(request, dashboardToken);
    expect(Number(verifiedProfile.data.tenantId)).toBe(fixture.expectedTenantId);
    expect(Number(verifiedProfile.data.corpId)).toBe(fixture.expectedCorpId);
    expect(String(verifiedProfile.data.wxCorpId)).toBe(fixture.expectedWxCorpId);

    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`${liveBase}/company-setting/website`);
    await assertSingleCompanySurface(page, 390);
    await expect(page.getByLabel('员工密钥')).toBeVisible();
    await assertCredentialConfirmationDoesNotMutate(page);

    const syncResponse = await request.get(`${liveBase}/dashboard/company/sync-status`, { headers: { Authorization: dashboardToken } });
    expect(syncResponse.status()).toBe(200);
    const syncData = responseData(await syncResponse.json() as JsonObject);
    expect(['queued', 'syncing', 'failed', 'completed']).toContain(syncData.status);
    expect(typeof syncData.departments).toBe('number');
    expect(typeof syncData.employees).toBe('number');
    expect(await page.locator('button', { hasText: '开始员工同步' }).isEnabled()).toBe(true);

    await assertNoSaaSLinks(page);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
    await assertCleanNetwork(audit, 'Dashboard company settings');
  });
});
