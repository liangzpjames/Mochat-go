import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const archiveBehaviorCommands = [
  ['test', './internal/app/runtime', '-run', '^TestRoleResponsibilitiesMatrix$', '-count=1'],
  ['test', './internal/archivebridge', '-run', '^(TestDriverRegistrarFailsClosedAndRollsBackInitializedDrivers|TestDriverRegistrarCloseAttemptsEveryDriverAndReturnsControlledError)$', '-count=1'],
  ['test', './cmd/mochat-archive-bridge', '-run', '^(TestLoadConfigRejectsFixtureAndProductionSDKTogether|TestRunProductionSDKModeRegistrationFailureNeverConstructsServer|TestServeDrainsHTTPBeforeRegistrarCloseAndReturnsCloseFailure)$', '-count=1'],
];

function defaultRunner(root, args) {
  return execFileSync('go', args, { cwd: root, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });
}

export function checkRepository(root, runner = defaultRunner) {
  const errors = [];
  for (const args of archiveBehaviorCommands) {
    try { runner(root, args); }
    catch (error) {
      const detail = String(error?.stderr || error?.stdout || error?.message || error).trim();
      errors.push(`go ${args.join(' ')} failed${detail ? `: ${detail}` : ''}`);
    }
  }
  return errors;
}

function main() {
  const errors = checkRepository(process.cwd());
  if (errors.length) {
    for (const error of errors) process.stderr.write(`FAIL ${error}\n`);
    process.exitCode = 1;
    return;
  }
  process.stdout.write('PASS WeCom archive runtime role, production registration and shutdown behavior\n');
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main();
