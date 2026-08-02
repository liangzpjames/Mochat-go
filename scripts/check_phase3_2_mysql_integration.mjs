import { execFileSync } from 'node:child_process';

const root = process.cwd();
const project = 'mochat-go-phase32-mysql';
const port = process.env.MOCHAT_PHASE32_MYSQL_PORT ?? '13342';
const composeArgs = ['compose', '-p', project, '-f', 'deploy/standalone/docker-compose.yml'];
const run = (file, args, env = process.env) => execFileSync(file, args, { cwd: root, env: { ...env, MOCHAT_MYSQL_PORT: port }, stdio: 'inherit' });
const capture = (file, args) => execFileSync(file, args, { cwd: root, env: { ...process.env, MOCHAT_MYSQL_PORT: port }, encoding: 'utf8' });
try {
  run('docker', [...composeArgs, 'up', '-d', 'mysql']);
  let healthy = false;
  for (let i = 0; i < 90; i += 1) {
    try {
      const raw = capture('docker', [...composeArgs, 'ps', '--format', 'json', 'mysql']).trim();
      healthy = raw.length > 0 && JSON.parse(raw).Health === 'healthy';
    } catch {}
    if (healthy) break;
    execFileSync(process.platform === 'win32' ? 'powershell.exe' : 'sleep', process.platform === 'win32' ? ['-NoProfile', '-Command', 'Start-Sleep -Seconds 2'] : ['2'], { stdio: 'ignore' });
  }
  if (!healthy) throw new Error('MariaDB did not become healthy');
  // The isolated Compose database imports the same ordered migration set through 0103
  // via docker-entrypoint-initdb.d; this avoids replaying ALTER statements already in schema/mochat.sql.
  run('go', ['test', '-count=1', './internal/modules/scrm/adapters/mysql', './internal/store'], { ...process.env, MOCHAT_MYSQL_PORT: port, MOCHAT_MYSQL_DSN: `mochat:mochat_pass@tcp(127.0.0.1:${port})/mochat?parseTime=true&loc=Local`, MOCHAT_REQUIRE_MYSQL_INTEGRATION: '1' });
  console.log('Phase 3.2 isolated MariaDB integration passed.');
} finally {
  try { run('docker', [...composeArgs, 'down', '-v', '--remove-orphans']); } catch {}
}
