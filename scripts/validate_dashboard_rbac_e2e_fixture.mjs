import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const manifestPath = path.resolve('web/apps/dashboard/src/benchmark/manifest.json');
const requiredAccounts = ['tenantDenied', 'noPermission', 'direct', 'twoRole', 'roleDisabledDirectRetained', 'ordinary49', 'superadmin'];
const protectedRoutes = new Set(['/company-setting/staff', '/setting/role', '/setting/additional', '/setting/authorization']);

export function validateFixture(fixture, manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))) {
  if (!fixture || typeof fixture !== 'object') throw new Error('fixture must be a JSON object');
  const routes = manifest.pages.map((page) => page.path);
  for (const name of requiredAccounts) {
    const account = fixture[name];
    if (!account || typeof account.phone !== 'string' || typeof account.password !== 'string') throw new Error(`${name} requires phone/password`);
    if (!Array.isArray(account.exactAllowedRoutes)) throw new Error(`${name}.exactAllowedRoutes is required`);
    for (const route of account.exactAllowedRoutes) if (!routes.includes(route)) throw new Error(`${name} contains unknown route ${route}`);
    if (name === 'tenantDenied' && account.exactAllowedRoutes.length !== 0) throw new Error('tenantDenied must have no routes');
    if (name === 'noPermission' && account.exactAllowedRoutes.length !== 0) throw new Error('noPermission must have no routes');
    if (name === 'ordinary49' && (account.exactAllowedRoutes.length !== 49 || account.exactAllowedRoutes.some((route) => protectedRoutes.has(route)))) throw new Error('ordinary49 must be exactly the 49 ordinary routes');
    if (name === 'superadmin' && account.exactAllowedRoutes.length !== routes.length) throw new Error('superadmin must have every manifest route');
    if (account.expectedSources && (!Array.isArray(account.expectedSources) || account.expectedSources.some((source) => typeof source.code !== 'string' || !['direct', 'role', 'inherited'].includes(source.type)))) throw new Error(`${name}.expectedSources requires permission code and valid source type`);
    if (name === 'direct' && (!account.directPermissionCode || !account.expectedSources?.some((source) => source.code === account.directPermissionCode && source.type === 'direct'))) throw new Error('direct requires directPermissionCode and a matching direct source');
    if (name === 'twoRole' && (!account.twoRolePermissionCode || !Array.isArray(account.twoRoleIds) || account.twoRoleIds.length !== 2 || account.twoRoleIds[0] === account.twoRoleIds[1] || account.twoRoleIds.some((id) => !account.expectedSources?.some((source) => source.code === account.twoRolePermissionCode && source.type === 'role' && source.id === id)))) throw new Error('twoRole requires two distinct role IDs contributing to one permission');
    if (name === 'roleDisabledDirectRetained' && (!account.disabledRolePermissionCode || typeof account.disabledRoleId !== 'number' || !account.directRetainedPermissionCode || !account.expectedSources?.some((source) => source.code === account.directRetainedPermissionCode && source.type === 'direct'))) throw new Error('roleDisabledDirectRetained requires disabled role and retained direct contracts');
  }
  return { accounts: requiredAccounts.length, routes: routes.length };
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
  const fixturePath = process.argv[2];
  if (!fixturePath) throw new Error('usage: node scripts/validate_dashboard_rbac_e2e_fixture.mjs <fixture.json>');
  const result = validateFixture(JSON.parse(fs.readFileSync(path.resolve(fixturePath), 'utf8')));
  console.log(`dashboard RBAC E2E fixture valid: ${result.accounts} accounts, ${result.routes} routes`);
}
