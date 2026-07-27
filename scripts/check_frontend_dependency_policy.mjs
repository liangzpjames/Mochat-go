import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { join, posix, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const dependencyFields = ['dependencies', 'devDependencies', 'optionalDependencies', 'peerDependencies'];
const requiredWorkspacePatterns = ['web/apps/*', 'web/packages/*'];
const requiredWorkspaceSettings = {
  autoInstallPeers: false,
  dedupePeerDependents: true,
  engineStrict: true,
  preferWorkspacePackages: true,
  saveExact: true,
  strictPeerDependencies: true,
};

function readJson(path, root, errors) {
  try {
    return JSON.parse(readFileSync(path, 'utf8'));
  } catch (error) {
    errors.push(`${relative(root, path).replaceAll('\\', '/')}: invalid JSON (${error.message})`);
    return null;
  }
}

function yamlScalar(value) {
  const trimmed = value.trim();
  if ((trimmed.startsWith("'") && trimmed.endsWith("'")) || (trimmed.startsWith('"') && trimmed.endsWith('"'))) {
    return trimmed.slice(1, -1);
  }
  if (trimmed === 'true') return true;
  if (trimmed === 'false') return false;
  return trimmed;
}

function parseWorkspaceConfig(root, errors) {
  const path = join(root, 'pnpm-workspace.yaml');
  if (!existsSync(path)) {
    errors.push('pnpm-workspace.yaml is missing');
    return { packages: [], settings: {} };
  }

  const packages = [];
  const settings = {};
  let inPackages = false;
  for (const line of readFileSync(path, 'utf8').split(/\r?\n/)) {
    if (/^packages:\s*$/.test(line)) {
      inPackages = true;
      continue;
    }
    const packageMatch = /^\s+-\s+(.+?)\s*$/.exec(line);
    if (inPackages && packageMatch) {
      packages.push(yamlScalar(packageMatch[1]));
      continue;
    }
    const settingMatch = /^([A-Za-z][A-Za-z0-9]*):\s*(.+?)\s*$/.exec(line);
    if (settingMatch) {
      inPackages = false;
      settings[settingMatch[1]] = yamlScalar(settingMatch[2]);
    }
  }

  for (const [name, expected] of Object.entries(requiredWorkspaceSettings)) {
    if (settings[name] !== expected) errors.push(`pnpm-workspace.yaml must set ${name}: ${expected}`);
  }
  for (const pattern of requiredWorkspacePatterns) {
    if (!packages.includes(pattern)) errors.push(`pnpm-workspace.yaml must include ${pattern}`);
  }
  for (const pattern of packages) {
    if (!requiredWorkspacePatterns.includes(pattern)) errors.push(`pnpm-workspace.yaml has unsupported workspace package pattern ${pattern}`);
  }
  return { packages, settings };
}

function directWorkspacePackages(root, patterns) {
  const result = [];
  for (const pattern of patterns) {
    if (typeof pattern !== 'string' || !pattern.endsWith('/*')) continue;
    const parent = join(root, pattern.slice(0, -2));
    if (!existsSync(parent)) continue;
    for (const entry of readdirSync(parent, { withFileTypes: true })) {
      if (!entry.isDirectory()) continue;
      const path = join(parent, entry.name, 'package.json');
      if (existsSync(path)) {
        result.push({ path, workspacePath: relative(root, join(parent, entry.name)).replaceAll('\\', '/') });
      }
    }
  }
  return result;
}

function dependencyOccurrences(manifest) {
  return dependencyFields.flatMap((field) => Object.entries(manifest[field] ?? {}).map(([name, specifier]) => ({ field, name, specifier })));
}

function normalizedLockImporters(lockfile) {
  const importers = new Map();
  const lines = lockfile.split(/\r?\n/);
  let inImporters = false;
  let importer = null;
  let dependencyType = null;
  let dependency = null;

  for (const line of lines) {
    if (!inImporters) {
      if (line === 'importers:') inImporters = true;
      continue;
    }
    if (/^\S/.test(line)) break;
    const importerMatch = /^ {2}([^ ].*?):\s*(?:\{\})?\s*$/.exec(line);
    if (importerMatch) {
      importer = { entries: new Map() };
      importers.set(yamlScalar(importerMatch[1]), importer);
      dependencyType = null;
      dependency = null;
      continue;
    }
    const typeMatch = /^ {4}(dependencies|devDependencies|optionalDependencies|peerDependencies):\s*(?:\{\})?\s*$/.exec(line);
    if (typeMatch && importer) {
      dependencyType = typeMatch[1];
      dependency = null;
      continue;
    }
    const dependencyMatch = /^ {6}([^ ].*?):\s*$/.exec(line);
    if (dependencyMatch && importer && dependencyType) {
      dependency = { field: dependencyType, specifier: null };
      importer.entries.set(yamlScalar(dependencyMatch[1]), dependency);
      continue;
    }
    const specifierMatch = /^ {8}specifier:\s*(.+?)\s*$/.exec(line);
    if (specifierMatch && dependency) dependency.specifier = yamlScalar(specifierMatch[1]);
  }
  return importers;
}

function validateDuplicates(packageInfo, occurrences, errors) {
  const byName = new Map();
  for (const occurrence of occurrences) {
    const previous = byName.get(occurrence.name);
    if (previous) {
      errors.push(`${packageInfo.workspacePath}: declares ${occurrence.name} in both ${previous.field} and ${occurrence.field}`);
    } else {
      byName.set(occurrence.name, occurrence);
    }
  }
}

function validateReact(packageInfo, occurrences, errors) {
  const byField = new Map(dependencyFields.map((field) => [field, new Map()]));
  for (const occurrence of occurrences) byField.get(occurrence.field).set(occurrence.name, occurrence.specifier);
  for (const [field, dependencies] of byField) {
    const react = dependencies.get('react');
    const reactDom = dependencies.get('react-dom');
    if (react !== undefined || reactDom !== undefined) {
      if (react !== reactDom || !/^19\.2\.\d+$/.test(react ?? '')) {
        errors.push(`${packageInfo.workspacePath}: ${field} react and react-dom must use the same exact version in the 19.2.x line`);
      }
    }
  }
}

function packageTarget(packageInfo, occurrence, byName, byWorkspacePath) {
  const direct = byName.get(occurrence.name);
  if (direct) return direct;

  const alias = /^workspace:(@[^/]+\/[^@]+|[^@]+)@/.exec(occurrence.specifier);
  if (alias) return byName.get(alias[1]);

  const local = /^(?:file|link):(.+)$/.exec(occurrence.specifier);
  if (local) {
    const targetPath = posix.normalize(posix.join(packageInfo.workspacePath, local[1].replaceAll('\\', '/'))).replace(/\/package\.json$/, '');
    return byWorkspacePath.get(targetPath);
  }
  return undefined;
}

function checkLockfile(root, packages, workspaceConfig, errors) {
  const path = join(root, 'pnpm-lock.yaml');
  if (!existsSync(path)) {
    errors.push('pnpm-lock.yaml drift: lockfile is missing');
    return;
  }
  const lockfile = readFileSync(path, 'utf8');
  if (!/^lockfileVersion:\s*['"]?\d/.test(lockfile) || !/^importers:/m.test(lockfile)) {
    errors.push('pnpm-lock.yaml drift: lockfile must declare lockfileVersion and importers');
    return;
  }

  const importers = normalizedLockImporters(lockfile);
  const peerImportersOptional = workspaceConfig.settings.autoInstallPeers === false;
  const expectedImporterIds = new Set(packages.map((packageInfo) => packageInfo.workspacePath));
  for (const importerId of importers.keys()) {
    if (!expectedImporterIds.has(importerId)) errors.push(`pnpm-lock.yaml drift: unexpected importer ${importerId}`);
  }
  for (const packageInfo of packages) {
    const importer = importers.get(packageInfo.workspacePath);
    if (!importer) {
      errors.push(`pnpm-lock.yaml drift: missing importer ${packageInfo.workspacePath}`);
      continue;
    }
    const occurrences = dependencyOccurrences(packageInfo.manifest);
    const expectedOccurrences = peerImportersOptional ? occurrences.filter((occurrence) => occurrence.field !== 'peerDependencies') : occurrences;
    for (const occurrence of expectedOccurrences) {
      const lockEntry = importer.entries.get(occurrence.name);
      if (!lockEntry) {
        errors.push(`pnpm-lock.yaml drift: ${packageInfo.workspacePath} dependency ${occurrence.name} is missing from ${occurrence.field}`);
      } else if (lockEntry.field !== occurrence.field) {
        errors.push(`pnpm-lock.yaml drift: ${packageInfo.workspacePath} dependency ${occurrence.name} is recorded under ${lockEntry.field}, expected ${occurrence.field}`);
      } else if (lockEntry.specifier !== occurrence.specifier) {
        errors.push(`pnpm-lock.yaml drift: ${packageInfo.workspacePath} dependency ${occurrence.name} has specifier ${lockEntry.specifier ?? '<missing>'}, expected ${occurrence.specifier}`);
      }
    }
    const manifestNames = new Set(expectedOccurrences.map((occurrence) => occurrence.name));
    for (const [name, lockEntry] of importer.entries) {
      if (peerImportersOptional && lockEntry.field === 'peerDependencies') continue;
      if (!manifestNames.has(name)) errors.push(`pnpm-lock.yaml drift: ${packageInfo.workspacePath} has stale dependency ${name} under ${lockEntry.field}`);
    }
  }
}

export function checkDependencyPolicy(root = process.cwd()) {
  const errors = [];
  const rootManifestPath = join(root, 'package.json');
  const rootManifest = existsSync(rootManifestPath) ? readJson(rootManifestPath, root, errors) : null;
  if (!rootManifest) {
    if (!errors.length) errors.push('root package.json is missing');
    return { ok: false, errors };
  }
  if (typeof rootManifest.packageManager !== 'string' || !/^pnpm@\d+\.\d+\.\d+$/.test(rootManifest.packageManager)) {
    errors.push('root package.json must declare packageManager as an exact pnpm version');
  }

  const workspaceConfig = parseWorkspaceConfig(root, errors);
  const packages = [{ workspacePath: '.', manifest: rootManifest, kind: 'root' }, ...directWorkspacePackages(root, workspaceConfig.packages)];
  for (const packageInfo of packages.slice(1)) packageInfo.manifest = readJson(packageInfo.path, root, errors);
  for (const packageInfo of packages.slice(1)) {
    if (packageInfo.workspacePath.startsWith('web/apps/')) packageInfo.kind = 'app';
    else if (packageInfo.workspacePath.startsWith('web/packages/')) packageInfo.kind = 'shared';
    else packageInfo.kind = 'workspace';
  }
  const validPackages = packages.filter((packageInfo) => packageInfo.manifest);
  const workspacePackages = validPackages.slice(1);
  const byName = new Map(workspacePackages.filter((packageInfo) => typeof packageInfo.manifest.name === 'string').map((packageInfo) => [packageInfo.manifest.name, packageInfo]));
  const byWorkspacePath = new Map(workspacePackages.map((packageInfo) => [packageInfo.workspacePath, packageInfo]));

  for (const packageInfo of validPackages) {
    const occurrences = dependencyOccurrences(packageInfo.manifest);
    validateDuplicates(packageInfo, occurrences, errors);
    validateReact(packageInfo, occurrences, errors);
    for (const occurrence of occurrences) {
      const target = packageTarget(packageInfo, occurrence, byName, byWorkspacePath);
      if (packageInfo.kind === 'app' && target && !occurrence.specifier.startsWith('workspace:')) {
        if (occurrence.name === target.manifest.name) {
          errors.push(`${packageInfo.workspacePath}: internal dependency ${occurrence.name} must use workspace:*`);
        } else {
          errors.push(`${packageInfo.workspacePath}: app internal dependency ${occurrence.name} must use workspace protocol for ${target.manifest.name}`);
        }
      }
      if (packageInfo.kind === 'shared' && target?.kind === 'app') {
        errors.push(`${packageInfo.workspacePath}: shared package ${packageInfo.manifest.name} must not depend on app ${target.manifest.name}`);
      }
    }
  }

  checkLockfile(root, validPackages, workspaceConfig, errors);
  return { ok: errors.length === 0, errors };
}

function main() {
  const result = checkDependencyPolicy();
  if (!result.ok) {
    for (const error of result.errors) console.error(`frontend-dependency-policy: ${error}`);
    process.exitCode = 1;
    return;
  }
  console.log('frontend-dependency-policy: ok');
}

if (resolve(process.argv[1] ?? '') === fileURLToPath(import.meta.url)) main();
