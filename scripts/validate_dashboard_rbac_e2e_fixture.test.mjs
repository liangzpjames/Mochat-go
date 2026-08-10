import assert from 'node:assert/strict';
import test from 'node:test';
import { validateFixture } from './validate_dashboard_rbac_e2e_fixture.mjs';

const routes = [...Array.from({ length: 49 }, (_, index) => `/route-${index + 1}`), '/company-setting/staff', '/setting/role', '/setting/additional', '/setting/authorization'];
const account = (exactAllowedRoutes) => ({ phone: 'fixture', password: 'fixture', exactAllowedRoutes });

test('fixture validator requires the complete account matrix and exact route contracts', () => {
  const fixture = Object.fromEntries(['tenantDenied', 'noPermission', 'direct', 'twoRole', 'roleDisabledDirectRetained', 'ordinary49', 'superadmin'].map((name) => [name, account([])]));
  fixture.direct.directPermissionCode = 'direct'; fixture.direct.expectedSources = [{ code: 'direct', type: 'direct' }];
  fixture.twoRole.twoRolePermissionCode = 'union'; fixture.twoRole.twoRoleIds = [11, 12]; fixture.twoRole.expectedSources = [{ code: 'union', type: 'role', id: 11 }, { code: 'union', type: 'role', id: 12 }];
  fixture.roleDisabledDirectRetained.disabledRolePermissionCode = 'disabled'; fixture.roleDisabledDirectRetained.disabledRoleId = 13; fixture.roleDisabledDirectRetained.directRetainedPermissionCode = 'retained'; fixture.roleDisabledDirectRetained.expectedSources = [{ code: 'retained', type: 'direct' }]; fixture.roleDisabledDirectRetained.forbiddenCodes = ['disabled']; fixture.roleDisabledDirectRetained.forbiddenRoleIds = [13];
  fixture.ordinary49.exactAllowedRoutes = [...routes.slice(0, 48), '/bad'];
  fixture.superadmin.exactAllowedRoutes = routes;
  assert.throws(() => validateFixture(fixture, { pages: routes.map((path) => ({ path })) }), /unknown route/);
});

test('fixture validator rejects missing exact routes', () => {
  assert.throws(() => validateFixture({}, { pages: [] }), /tenantDenied requires/);
});
