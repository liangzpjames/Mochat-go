import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const sensitiveFieldPattern = /"(?:password|passwd|token|secret|cookie|authorization|api_key|private_key|content|prompt|body)"\s*,/gi;

function read(repoRoot, relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8');
}

function serviceBlock(compose, serviceName, nextServiceName) {
	const marker = `\n  ${serviceName}:`;
	const markerIndex = compose.indexOf(marker);
	const start = markerIndex < 0 ? -1 : markerIndex + 1;
  if (start < 0) return '';
	const end = nextServiceName ? compose.indexOf(`\n  ${nextServiceName}:`, start + 1) : compose.indexOf('\nvolumes:', start + 1);
  return compose.slice(start, end < 0 ? compose.length : end);
}

function logCallSnippets(source) {
  const snippets = [];
  const callPattern = /\.(?:Debug|Info|Warn|Error|Log)(?:Context)?\s*\(/g;
  for (const match of source.matchAll(callPattern)) {
    const opening = source.indexOf('(', match.index);
    let depth = 0;
    let quote = '';
    let escaped = false;
    for (let index = opening; index < source.length; index += 1) {
      const char = source[index];
      if (quote) {
        if (quote !== '`' && escaped) escaped = false;
        else if (quote !== '`' && char === '\\') escaped = true;
        else if (char === quote) quote = '';
        continue;
      }
      if (char === '"' || char === "'" || char === '`') { quote = char; continue; }
      if (char === '(') depth += 1;
      if (char === ')' && --depth === 0) {
        snippets.push(source.slice(match.index, index + 1));
        break;
      }
    }
  }
  return snippets;
}

export function checkLoggingPolicy(repoRoot) {
  const violations = [];
  const main = read(repoRoot, 'cmd/mochat-go/main.go');
  if (/log\.Printf\("go [^"\r\n]*routes? enabled/.test(main)) {
    violations.push('cmd/mochat-go/main.go still logs each enabled route at INFO');
  }
  if (/log\.Print(?:f|ln)?\s*\(/.test(main)) {
    violations.push('cmd/mochat-go/main.go still emits legacy startup details at INFO');
  }
  if (/debugf\("go [^"\r\n]*\broutes? enabled:/.test(main)) {
    violations.push('cmd/mochat-go/main.go labels route registration as a generic runtime detail');
  }
  for (const required of [
    'loggedAPIHandler := withHTTPLogging(handler)',
    'frontend.WrapDashboard(loggedAPIHandler',
    'frontend.WrapApp(loggedAPIHandler, frontend.AppConfig{DistDir: dist})',
  ]) {
    if (!main.includes(required)) violations.push(`cmd/mochat-go/main.go side listeners can bypass HTTP logging: missing ${required}`);
  }
  const scrm = read(repoRoot, 'cmd/mochat-go/scrm.go');
  if (/log\.Print(?:f|ln)?\s*\(/.test(scrm)) {
    violations.push('cmd/mochat-go/scrm.go still logs each auth resolver at INFO');
  }

  const archive = read(repoRoot, 'internal/dashboard/work_message_archive_sync_cron.go');
  if (/LimitReader\(resp\.Body/.test(archive) || /missing msgid[^\r\n]*string\(raw\)/.test(archive)) {
    violations.push('archive bridge error can include an external response or raw message body');
  }

  for (const relativePath of [
    'internal/observability/logging.go',
    'internal/observability/http.go',
    'internal/taskrunner/taskrunner.go',
    'internal/taskrunner/sql_recorder.go',
    'internal/dashboard/work_message_archive_sync_cron.go',
    'internal/modules/providers/ai/openai/openai.go',
    'internal/wecomsuitecallback/handler.go',
    'cmd/mochat-go/logging.go',
    'cmd/mochat-migrate/main.go',
  ]) {
    const source = read(repoRoot, relativePath);
    const fields = logCallSnippets(source).flatMap((snippet) => snippet.match(sensitiveFieldPattern) ?? []);
    for (const field of fields) violations.push(`${relativePath} uses forbidden log field ${field.split(',')[0]}`);
  }

  const compose = read(repoRoot, 'deploy/standalone/docker-compose.yml');
  for (const required of ['x-mochat-logging:', 'driver: json-file', 'max-size:', 'max-file:']) {
    if (!compose.includes(required)) violations.push(`standalone compose missing ${required}`);
  }
  const services = ['app', 'archive-bridge', 'archive-simulator', 'mysql', 'redis'];
  for (let index = 0; index < services.length; index += 1) {
    const block = serviceBlock(compose, services[index], services[index + 1]);
    if (!block.includes('logging: *mochat-logging')) violations.push(`standalone service ${services[index]} missing logging rotation`);
  }
  for (const service of ['app', 'archive-bridge', 'archive-simulator']) {
    const next = services[services.indexOf(service) + 1];
    const block = serviceBlock(compose, service, next);
    for (const variable of ['MOCHAT_LOG_LEVEL:', 'MOCHAT_LOG_FORMAT:', 'MOCHAT_LOG_SOURCE:']) {
      if (!block.includes(variable)) violations.push(`standalone service ${service} missing ${variable}`);
    }
  }
  if (/\/var\/log|\.log:\/|logs?:\//i.test(compose)) {
    violations.push('standalone compose mounts an application log path; stdout must be the only application sink');
  }
  if (!compose.includes('MOCHAT_GO_ENABLE_CONVERSATION_EXPORT_WORKER: "${MOCHAT_GO_ENABLE_CONVERSATION_EXPORT_WORKER:-0}"')) {
    violations.push('standalone compose enables the export worker before migrations are confirmed');
  }
  return { violations };
}

const invokedPath = process.argv[1] ? path.resolve(process.argv[1]) : '';
if (invokedPath === fileURLToPath(import.meta.url)) {
  const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
  const result = checkLoggingPolicy(repoRoot);
  if (result.violations.length > 0) {
    for (const violation of result.violations) console.error(`logging-policy: ${violation}`);
    process.exitCode = 1;
  } else {
    console.log('logging-policy: PASS');
  }
}
