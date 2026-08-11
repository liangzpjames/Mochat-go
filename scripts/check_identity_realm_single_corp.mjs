import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const GO_EXT = '.go';
const FRONTEND_EXTENSIONS = ['.ts', '.tsx', '.js', '.jsx'];
const ignoredName = (name) => /(?:\.test|\.spec)\.[^.]+$/.test(name) || name.endsWith('_test.go');

function walk(directory, predicate = () => true) {
  if (!fs.existsSync(directory)) return [];
  const entries = fs.readdirSync(directory, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const full = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      if (/^(?:node_modules|vendor|dist|build|fixtures?|migrations?)$/i.test(entry.name)) continue;
      files.push(...walk(full, predicate));
    } else if (predicate(full, entry.name)) {
      files.push(full);
    }
  }
  return files;
}

function stripComments(source, extension) {
  if (extension === GO_EXT) {
    return source
      .replace(/\/\*[\s\S]*?\*\//g, (value) => value.replace(/[^\n]/g, ' '))
      .replace(/(^|\s)\/\/.*$/gm, '$1');
  }
  return source
    .replace(/\/\*[\s\S]*?\*\//g, (value) => value.replace(/[^\n]/g, ' '))
    .replace(/(^|\s)\/\/.*$/gm, '$1');
}

function locationFor(file, source, pattern, label = '') {
  const match = source.match(pattern);
  if (!match) return null;
  const line = source.slice(0, match.index).split('\n').length;
  return { file: file.replaceAll('\\', '/'), line, match: label || match[0] };
}

function locationsFor(files, pattern, label = '') {
  const locations = [];
  for (const file of files) {
    const extension = path.extname(file);
    const source = stripComments(fs.readFileSync(file, 'utf8'), extension);
    const location = locationFor(file, source, pattern, label);
    if (location) locations.push(location);
  }
  return locations;
}

function productionGoFiles(root) {
  return walk(path.join(root, 'internal'), (file, name) => file.endsWith(GO_EXT) && !ignoredName(name))
    .concat(walk(path.join(root, 'cmd'), (file, name) => file.endsWith(GO_EXT) && !ignoredName(name)));
}

function resolveImport(file, specifier) {
  if (!specifier.startsWith('.')) return null;
  const base = path.resolve(path.dirname(file), specifier);
  const candidates = [base, ...FRONTEND_EXTENSIONS.map((extension) => `${base}${extension}`), ...FRONTEND_EXTENSIONS.map((extension) => path.join(base, `index${extension}`))];
  return candidates.find((candidate) => fs.existsSync(candidate) && fs.statSync(candidate).isFile()) || null;
}

function frontendProductionFiles(root, app) {
  const sourceRoot = path.join(root, 'web', 'apps', app, 'src');
  const all = walk(sourceRoot, (file, name) => FRONTEND_EXTENSIONS.includes(path.extname(file)) && !ignoredName(name));
  const entries = all.filter((file) => /(?:^|[\\/])(?:main|App)\.(?:ts|tsx|js|jsx)$/.test(file));
  const queue = [...entries];
  const seen = new Set();
  while (queue.length) {
    const file = queue.shift();
    if (!file || seen.has(file)) continue;
    seen.add(file);
    const source = stripComments(fs.readFileSync(file, 'utf8'), path.extname(file));
    for (const match of source.matchAll(/(?:from\s*|import\s*\()(['"])([^'"]+)\1/g)) {
      const imported = resolveImport(file, match[2]);
      if (imported && !ignoredName(path.basename(imported))) queue.push(imported);
    }
  }
  // A missing entry should be a visible failure, but a small fixture may expose
  // its production source under a single file with a non-standard name.
  return seen.size ? [...seen] : all;
}

function assertNo(evidence, message) {
  if (!evidence.length) return;
  const details = evidence.map(({ file, line, match }) => `${file}:${line}${match ? ` (${match})` : ''}`).join(', ');
  throw new Error(`${message}: ${details}`);
}

function requireEvidence(evidence, message) {
  if (evidence.length) return;
  throw new Error(`${message}: no production source evidence`);
}

function runIdentitySingleCorpGate(root = process.cwd()) {
  const goFiles = productionGoFiles(root);
  const saasGo = goFiles.filter((file) => /[\\/]saasauth[\\/]/.test(file));
  const dashboardAuthGo = goFiles.filter((file) => /[\\/]dashboardauth[\\/]/.test(file));
  const dashboardGo = goFiles.filter((file) => /[\\/]dashboard[\\/]/.test(file));
  const frontend = [
    ...frontendProductionFiles(root, 'saas-admin'),
    ...frontendProductionFiles(root, 'dashboard'),
  ];
  const allProduction = [...new Set([...goFiles, ...frontend])];

  const saasIdentityTables = locationsFor(saasGo, /mochat_go_saas_admin_users/);
  const dashboardIdentityTables = locationsFor(dashboardAuthGo, /mochat_go_dashboard_identities/);
  requireEvidence(saasIdentityTables, 'SaaS identity table must be owned by saasauth');
  requireEvidence(dashboardIdentityTables, 'Dashboard identity table must be owned by dashboardauth');
  const saasSharedIdentity = locationsFor(saasGo, /\bmc_user\b/);
  assertNo(saasSharedIdentity, 'SaaS identity/auth store must not read mc_user');

  const jwtRealms = {
    saas_admin: locationsFor([...saasGo, ...frontendProductionFiles(root, 'saas-admin')], /saas_admin/),
    dashboard: locationsFor([...dashboardAuthGo, ...frontendProductionFiles(root, 'dashboard')], /(?:^|[^A-Za-z])dashboard(?:$|[^A-Za-z])/),
  };
  requireEvidence(jwtRealms.saas_admin, 'SaaS JWT realm must be explicit');
  requireEvidence(jwtRealms.dashboard, 'Dashboard JWT realm must be explicit');

  const sharedJWT = locationsFor([...saasGo, ...dashboardAuthGo, ...frontend], /MOCHAT_SIMPLE_JWT_SECRET|(?:shared|common)[_-]?(?:jwt|token)|JWT_SECRET\s*=\s*['"][^'"]+['"]/i);
  assertNo(sharedJWT, 'shared JWT configuration or legacy simple JWT reference');

  const forbiddenCorpRoutes = locationsFor(allProduction, /(?:GET\s+|POST\s+|['"`])\/?dashboard\/corp\/(?:select|bind|store)|dashboard\/corp\/(?:select|bind|store)/i, 'legacy corp route');
  assertNo(forbiddenCorpRoutes, 'legacy corp route registered in production');

  const forbiddenSessionCorpFields = locationsFor(
    [...dashboardGo, ...frontend],
    /(?:persistCorpId|mochat_dashboard_corp_id|selectedCorpID|\bcorpId\b\s*[:=])/,
    'corp selection/session field',
  );
  assertNo(forbiddenSessionCorpFields, 'Dashboard session or request still carries corp selection');

  const companySelector = locationsFor(frontend, /新建企业|(?:corp|company)[_-]?(?:provider|selector|switcher)|企业选择器|企业列表/i, 'company selector');
  assertNo(companySelector, 'Dashboard production UI still exposes company selector/new-company flow');

  const plaintextSecretReads = locationsFor(
    allProduction,
    /(?:\.(?:EmployeeSecret|ContactSecret|WxSecret|SessionArchiveSecret|EncodingAESKey|CallbackToken)|\b(?:employee_secret|contact_secret|wx_secret|session_archive_secret)\b(?!_ciphertext))/,
    'plaintext secret read',
  );
  assertNo(plaintextSecretReads, 'plaintext credential read in production');

  const dashboardPrincipalConsumers = locationsFor(dashboardGo, /DashboardPrincipalFromContext|RequireDashboardPrincipal|DashboardPrincipal\b/);
  const unboundHandlers = [];
  const handlerPattern = /func\s+(?:\([^)]*\)\s*)?[A-Za-z_][A-Za-z0-9_]*Handler\b|func\s+\([^)]*\)\s*ServeHTTP\b/;
  for (const file of dashboardGo) {
    const source = stripComments(fs.readFileSync(file, 'utf8'), GO_EXT);
    if (!handlerPattern.test(source)) continue;
    if (!/DashboardPrincipalFromContext|RequireDashboardPrincipal|DashboardPrincipal\b/.test(source)) {
      unboundHandlers.push(locationFor(file, source, handlerPattern));
    }
  }
  assertNo(unboundHandlers, 'Dashboard handler does not consume DashboardPrincipal');
  requireEvidence(dashboardPrincipalConsumers, 'DashboardPrincipal consumer evidence is required');

  return {
    saasIdentityTables,
    dashboardIdentityTables,
    jwtRealms,
    dashboardPrincipalConsumers,
    forbiddenCorpRoutes,
    forbiddenSessionCorpFields,
    plaintextSecretReads,
  };
}

export { runIdentitySingleCorpGate };

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const result = runIdentitySingleCorpGate();
  console.log(`identity single-corp gate PASS: SaaS realm=${result.jwtRealms.saas_admin.length}, Dashboard realm=${result.jwtRealms.dashboard.length}, principal consumers=${result.dashboardPrincipalConsumers.length}`);
}
