function assertUniqueRoutes(routes, label) {
  if (routes.length !== 53) throw new Error(`${label} must contain exactly 53 routes`);
  const seen = new Set();
  for (const route of routes) {
    if (seen.has(route)) throw new Error(`duplicate ${label.replace(/ routes$/, '')} route: ${route}`);
    seen.add(route);
  }
  return seen;
}

function sameSet(left, right) {
  return left.size === right.size && [...left].every((item) => right.has(item));
}

function expectedResponse(route, response, expected) {
  if (response.status === 401 || response.machineCode === 'UNAUTHORIZED'
    || response.machineCode === 'TENANT_ACCESS_DENIED'
    || response.machineCode === 'DASHBOARD_PERMISSION_DENIED') return false;
  return expected.some((item) => item.route === route
    && item.url === response.url
    && item.status === response.status
    && (item.machineCode ?? null) === (response.machineCode ?? null));
}

function assertNoSensitiveFields(value, path = 'evidence') {
  if (Array.isArray(value)) {
    value.forEach((item, index) => assertNoSensitiveFields(item, `${path}[${index}]`));
    return;
  }
  if (value === null || typeof value !== 'object') return;
  for (const [key, item] of Object.entries(value)) {
    if (/^(?:body|headers|token|authorization)$/i.test(key)) {
      throw new Error(`${path}.${key} is forbidden in evidence`);
    }
    assertNoSensitiveFields(item, `${path}.${key}`);
  }
}

export function validateDashboardAllPagesEvidence({
  manifestRoutes,
  actionRegistry,
  evidence,
  expectedResponses = [],
}) {
  if (!Array.isArray(manifestRoutes) || !Array.isArray(actionRegistry) || !Array.isArray(evidence)) {
    throw new Error('manifest routes, action registry and evidence must be arrays');
  }
  const manifest = assertUniqueRoutes(manifestRoutes, 'manifest routes');
  const actionRoutes = assertUniqueRoutes(actionRegistry.map((item) => item.route), 'action registry routes');
  const evidenceRoutes = assertUniqueRoutes(evidence.map((item) => item.route), 'evidence routes');
  if (!sameSet(manifest, actionRoutes)) throw new Error('action registry routes must exactly match manifest routes');
  if (!sameSet(manifest, evidenceRoutes)) throw new Error('evidence routes must exactly match manifest routes');

  const actions = new Map(actionRegistry.map((item) => [item.route, item.action]));
  for (const [route, action] of actions) {
    if (typeof action !== 'string' || action.trim() === '') throw new Error(`${route} is missing a safe action`);
    if (!/^(?:button|link):\S/.test(action)) throw new Error(`${route} action must locate a clickable button or link`);
  }

  assertNoSensitiveFields(evidence);
  for (const page of evidence) {
    if (page.action !== actions.get(page.route)) throw new Error(`${page.route} evidence action does not match the registry`);
    if (page.titleVisible !== true) throw new Error(`${page.route} title was not visible`);
    if (typeof page.screenshot !== 'string' || page.screenshot.trim() === '') throw new Error(`${page.route} screenshot is missing`);
    if (!Array.isArray(page.unexpectedResponses) || page.unexpectedResponses.length > 0) throw new Error(`${page.route} has unexpected responses`);
    for (const response of page.responseStatuses ?? []) {
      if (response.status >= 400 && !expectedResponse(page.route, response, expectedResponses)) {
        throw new Error(`${page.route} unexpected response ${response.status} ${response.url}${response.machineCode ? ` ${response.machineCode}` : ''}`);
      }
    }
    if (!Array.isArray(page.consoleErrors) || page.consoleErrors.length > 0) throw new Error(`${page.route} has console errors`);
    if (!Array.isArray(page.pageErrors) || page.pageErrors.length > 0) throw new Error(`${page.route} has page errors`);
    if (page.returnedToIndex !== true) throw new Error(`${page.route} could not return to /index`);
    if (page.userNameVisible !== true) throw new Error(`${page.route} user name disappeared`);
    if (page.corpNameVisible !== true) throw new Error(`${page.route} corp name disappeared`);
  }
  return { pageCount: evidence.length, expectedResponseCount: expectedResponses.length };
}
