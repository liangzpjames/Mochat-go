import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { join, posix, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const dependencyFields = ['dependencies', 'devDependencies', 'optionalDependencies', 'peerDependencies'];
const requiredWorkspacePatterns = ['web/apps/*', 'web/e2e', 'web/packages/*'];
const requiredNodeRange = '>=22.12 <25';
const requiredNodeInterval = {
  lower: { version: [22, 12, 0], inclusive: true },
  upper: { version: [25, 0, 0], inclusive: false },
};
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
    if (typeof pattern !== 'string') continue;
    if (!pattern.endsWith('/*')) {
      const path = join(root, pattern, 'package.json');
      if (existsSync(path)) result.push({ path, workspacePath: pattern });
      continue;
    }
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

function compareVersions(left, right) {
  for (let index = 0; index < 3; index += 1) {
    const difference = left[index] - right[index];
    if (difference !== 0) return Math.sign(difference);
  }
  return 0;
}

function parseVersion(value) {
  const match = /^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?$/.exec(value);
  if (!match) return null;
  return {
    version: [Number(match[1]), Number(match[2] ?? 0), Number(match[3] ?? 0)],
    parts: match[3] === undefined ? (match[2] === undefined ? 1 : 2) : 3,
  };
}

function rangeForWildcardVersion(value) {
  const parts = value.toLowerCase().split('.');
  if (parts.length > 3 || parts.some((part) => !/^(?:\d+|x|\*)$/.test(part))) return null;
  const wildcardIndex = parts.findIndex((part) => part === 'x' || part === '*');
  if (wildcardIndex <= 0 || parts.slice(wildcardIndex).some((part) => part !== 'x' && part !== '*')) return null;
  const lowerVersion = [0, 0, 0];
  for (let index = 0; index < wildcardIndex; index += 1) lowerVersion[index] = Number(parts[index]);
  const upperVersion = [...lowerVersion];
  upperVersion[wildcardIndex - 1] += 1;
  for (let index = wildcardIndex; index < upperVersion.length; index += 1) upperVersion[index] = 0;
  return {
    lower: { version: lowerVersion, inclusive: true },
    upper: { version: upperVersion, inclusive: false },
  };
}

function mergeLower(current, candidate) {
  if (!current) return candidate;
  const comparison = compareVersions(current.version, candidate.version);
  if (comparison > 0 || (comparison === 0 && !current.inclusive)) return current;
  if (comparison < 0 || (comparison === 0 && !candidate.inclusive)) return candidate;
  return current;
}

function mergeUpper(current, candidate) {
  if (!current) return candidate;
  const comparison = compareVersions(current.version, candidate.version);
  if (comparison < 0 || (comparison === 0 && !current.inclusive)) return current;
  if (comparison > 0 || (comparison === 0 && !candidate.inclusive)) return candidate;
  return current;
}

function rangeForToken(token) {
  if (token === '*' || token.toLowerCase() === 'x') return {};
  const match = /^(\^|~|>=|<=|>|<|=)?(.+)$/.exec(token);
  const operator = match?.[1] ?? '';
  const wildcardRange = rangeForWildcardVersion(match?.[2] ?? '');
  if (wildcardRange) {
    if (!operator || operator === '=') return wildcardRange;
    if (operator === '>=') return { lower: wildcardRange.lower };
    if (operator === '>') return { lower: { ...wildcardRange.upper, inclusive: true } };
    if (operator === '<') return { upper: { ...wildcardRange.lower, inclusive: false } };
    if (operator === '<=') return { upper: wildcardRange.upper };
    return null;
  }
  const parsed = parseVersion(match?.[2] ?? '');
  if (!parsed) return null;
  const { version, parts } = parsed;
  if (operator === '>=') return { lower: { version, inclusive: true } };
  if (operator === '>') return { lower: { version, inclusive: false } };
  if (operator === '<=') return { upper: { version, inclusive: true } };
  if (operator === '<') return { upper: { version, inclusive: false } };
  if (operator === '^') {
    const upper = [...version];
    const pivot = version.findIndex((part) => part !== 0);
    upper[pivot === -1 ? 0 : pivot] += 1;
    for (let index = (pivot === -1 ? 0 : pivot) + 1; index < 3; index += 1) upper[index] = 0;
    return { lower: { version, inclusive: true }, upper: { version: upper, inclusive: false } };
  }
  if (operator === '~') {
    const upper = [...version];
    upper[parts === 1 ? 0 : 1] += 1;
    for (let index = (parts === 1 ? 0 : 1) + 1; index < 3; index += 1) upper[index] = 0;
    return { lower: { version, inclusive: true }, upper: { version: upper, inclusive: false } };
  }
  if (operator === '=') return { lower: { version, inclusive: true }, upper: { version, inclusive: true } };
  const upper = [...version];
  upper[parts - 1] += 1;
  for (let index = parts; index < 3; index += 1) upper[index] = 0;
  return { lower: { version, inclusive: true }, upper: { version: upper, inclusive: false } };
}

function parseNodeEngineRange(range) {
  return range.trim().replace(/(\^|~|>=|<=|>|<|=)\s+/g, '$1').split(/\s*\|\|\s*/).map((clause) => {
    const interval = {};
    for (const token of clause.trim().split(/\s+/)) {
      const tokenRange = rangeForToken(token);
      if (!tokenRange) return null;
      if (tokenRange.lower) interval.lower = mergeLower(interval.lower, tokenRange.lower);
      if (tokenRange.upper) interval.upper = mergeUpper(interval.upper, tokenRange.upper);
    }
    if (interval.lower && interval.upper) {
      const comparison = compareVersions(interval.lower.version, interval.upper.version);
      if (comparison > 0 || (comparison === 0 && (!interval.lower.inclusive || !interval.upper.inclusive))) return null;
    }
    return interval;
  });
}

function lowerStartsAtOrBefore(lower, version) {
  if (!lower) return true;
  const comparison = compareVersions(lower.version, version);
  return comparison < 0 || (comparison === 0 && lower.inclusive);
}

function upperExtendsBeyond(upper, version) {
  if (!upper) return true;
  return compareVersions(upper.version, version) > 0;
}

function intervalSort(left, right) {
  if (!left.lower) return right.lower ? -1 : 0;
  if (!right.lower) return 1;
  const comparison = compareVersions(left.lower.version, right.lower.version);
  if (comparison !== 0) return comparison;
  return Number(right.lower.inclusive) - Number(left.lower.inclusive);
}

function supportsRequiredNodeRange(range) {
  const intervals = parseNodeEngineRange(range);
  if (intervals.some((interval) => interval === null)) return false;
  let cursor = requiredNodeInterval.lower.version;
  for (const interval of intervals.sort(intervalSort)) {
    if (!lowerStartsAtOrBefore(interval.lower, cursor) || !upperExtendsBeyond(interval.upper, cursor)) continue;
    if (!interval.upper || compareVersions(interval.upper.version, requiredNodeInterval.upper.version) >= 0) return true;
    cursor = interval.upper.version;
  }
  return false;
}

function validateLockfileNodeEngines(lockfile, errors) {
  let inPackages = false;
  let packageName = null;
  for (const line of lockfile.split(/\r?\n/)) {
    if (!inPackages) {
      if (line === 'packages:') inPackages = true;
      continue;
    }
    if (/^\S/.test(line)) break;
    const packageMatch = /^ {2}(.+):\s*$/.exec(line);
    if (packageMatch) {
      packageName = yamlScalar(packageMatch[1]);
      continue;
    }
    const enginesMatch = /^ {4}engines:\s+\{(.+)\}\s*$/.exec(line);
    const nodeMatch = enginesMatch && /(?:^|,\s*)node:\s*('(?:[^']*)'|"(?:[^"]*)"|[^,}]+)/.exec(enginesMatch[1]);
    if (nodeMatch && packageName) {
      const range = yamlScalar(nodeMatch[1]);
      if (!supportsRequiredNodeRange(range)) {
        errors.push(`pnpm-lock.yaml: ${packageName} node engine ${range} excludes required Node range ${requiredNodeRange}`);
      }
    }
  }
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

  validateLockfileNodeEngines(lockfile, errors);

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
      if (packageInfo.kind === 'app' && target) {
        if (occurrence.name === target.manifest.name && occurrence.specifier !== 'workspace:*') {
          errors.push(`${packageInfo.workspacePath}: internal dependency ${occurrence.name} must use workspace:*`);
        } else if (occurrence.name !== target.manifest.name && !occurrence.specifier.startsWith('workspace:')) {
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
