import assert from 'node:assert/strict';
import test from 'node:test';

import { archiveBehaviorCommands, checkRepository } from './check_wecom_archive_saas_activation.mjs';

test('gate executes the role matrix, fail-closed registration and shutdown behavior', () => {
  const calls = [];
  const errors = checkRepository('/fixture', (_root, args) => {
    calls.push(args.join(' '));
    return 'ok\n';
  });
  assert.deepEqual(errors, []);
  assert.equal(calls.length, archiveBehaviorCommands.length);
  assert.ok(calls.some((call) => call.includes('TestRoleResponsibilitiesMatrix')));
  assert.ok(calls.some((call) => call.includes('TestRunProductionSDKModeRegistrationFailureNeverConstructsServer')));
  assert.ok(calls.some((call) => call.includes('TestServeDrainsHTTPBeforeRegistrarCloseAndReturnsCloseFailure')));
  assert.ok(calls.some((call) => call.includes('TestLoadConfigRejectsFixtureAndProductionSDKTogether')));
});

test('a behavior-test mutation cannot be hidden by production source comments or dead fragments', () => {
  for (const failingIndex of archiveBehaviorCommands.keys()) {
    let callIndex = 0;
    const errors = checkRepository('/fixture', (_root, args) => {
      if (callIndex++ === failingIndex) throw Object.assign(new Error('mutation survived'), { stderr: 'FAIL mutation survived' });
      return 'ok\n';
    });
    assert.ok(errors.some((error) => error.includes('mutation survived')), `failure ${failingIndex} was ignored`);
  }
});
