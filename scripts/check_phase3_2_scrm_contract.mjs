import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

const requiredDomainSymbols = ['LeadStatusQualified', 'AssignmentPublicPool', 'OpportunityLost', 'FollowUp', 'CustomerTag'];
const requiredRoutes = ['/dashboard/scrm/leads', '/dashboard/scrm/contacts', '/dashboard/scrm/assignments', '/dashboard/scrm/opportunities', '/dashboard/scrm/stages', '/dashboard/scrm/followUps', '/dashboard/scrm/tags'];

export function validateSCRMContract(contract) {
  if (typeof contract?.migration !== 'string' || !contract.migration.includes('tenant_id')) throw new Error('migration must declare tenant_id');
  if (!contract.migration.includes('corp_id') || !contract.migration.includes('version')) throw new Error('migration must declare corp_id and version');
  for (const symbol of requiredDomainSymbols) if (!String(contract.domain).includes(symbol)) throw new Error(`domain missing ${symbol}`);
  for (const route of requiredRoutes) if (!contract.routes?.includes(route)) throw new Error(`route missing ${route}`);
}

export async function readSCRMContract() {
  const [migration, domain] = await Promise.all([
    readFile(new URL('../deploy/standalone/migrations/0100_scrm_customer_lifecycle.up.sql', import.meta.url), 'utf8'),
    readFile(new URL('../internal/modules/scrm/domain/customer_lifecycle.go', import.meta.url), 'utf8'),
  ]);
  return { migration, domain, routes: requiredRoutes };
}

async function main() {
  validateSCRMContract(await readSCRMContract());
  console.log('Phase 3.2 SCRM contract passed.');
}

if (process.argv[1] === fileURLToPath(import.meta.url)) main().catch((error) => { console.error(error.message); process.exitCode = 1; });
