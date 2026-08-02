import assert from 'node:assert/strict';
import { test } from 'node:test';

import { validateSCRMContract } from './check_phase3_2_scrm_contract.mjs';

test('the SCRM contract requires lifecycle resources and versioned writes', () => {
  assert.doesNotThrow(() => validateSCRMContract({
    migration: 'CREATE TABLE mochat_go_scrm_contacts (tenant_id bigint, corp_id bigint, version bigint);',
    domain: 'LeadStatusQualified AssignmentPublicPool OpportunityLost FollowUp CustomerTag',
    routes: ['/dashboard/scrm/leads', '/dashboard/scrm/contacts', '/dashboard/scrm/assignments', '/dashboard/scrm/opportunities', '/dashboard/scrm/stages', '/dashboard/scrm/followUps', '/dashboard/scrm/tags'],
  }));
});

test('the SCRM contract rejects a migration without tenant isolation', () => {
  assert.throws(() => validateSCRMContract({ migration: 'CREATE TABLE contacts (id bigint);', domain: 'LeadStatusQualified' }), /tenant_id/);
});
