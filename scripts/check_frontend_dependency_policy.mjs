import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const dependencyFields = ['dependencies', 'devDependencies', 'optionalDependencies', 'peerDependencies'];

function readJson(path, errors) {
  try {
    return JSON.parse(readFileSync(path, 'utf8'));
  } catch (error) {
    errors.push(`${relative(process.cwd(), path).replaceAll('\\', '/')}: invalid JSON (${error.message})`);
    return null;
  }
}

function packageManifests(root, directory) {
  const result = [];
  if (!existsSync(directory)) return result;

  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) result.push(...packageManifests(root, path));
    if (entry.isFile() && entry.name === 'package.json') {
      result.push({ path, workspacePath: relative(root, dirname(path)).replaceAll('\\', '/') });
    }
  }
  return result;
}

function allDependencies(manifest) {
  return dependencyFields.flatMap((field) => Object.entries(manifest[field] ?? {}));
}

function escaped(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function lockImporter(lockfile, workspacePath) {
  const key = escaped(workspacePath);
  const matcher = new RegExp(`^  (?:'${key}'|"${key}"|${key}):[^\\n]*(?:\\r?\\n|$)`, 'm');
  const match = matcher.exec(lockfile);
  if (!match) return null;
  const start = match.index + match[0].length;
  const remainder = lockfile.slice(start);
  const next = /^  (?:'[^']+'|"[^"]+"|[^\s][^:]*):[^\n]*(?:\r?\n|$)/m.exec(remainder);
  return remainder.slice(0, next?.index ?? remainder.length);
}

function lockSpecifier(importer, packageName) {
  const name = escaped(packageName);
  const matcher = new RegExp(
    `^      (?:'${name}'|"${name}"|${name}):\\r?\\n(?:^        [^\\n]*\\r?\\n)*?^        specifier: ([^\\r\\n]+)$`,
    'm',
  );
  return matcher.exec(importer)?.[1]?.trim() ?? null;
}

function checkLockfile(root, packages, errors) {
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

  for (const packageInfo of packages) {
    const importer = lockImporter(lockfile, packageInfo.workspacePath);
    if (importer === null) {
      errors.push(`pnpm-lock.yaml drift: missing importer ${packageInfo.workspacePath}`);
      continue;
    }
    for (const [name, specifier] of allDependencies(packageInfo.manifest)) {
      const actual = lockSpecifier(importer, name);
      if (actual !== specifier) {
        errors.push(`pnpm-lock.yaml drift: ${packageInfo.workspacePath} dependency ${name} has specifier ${actual ?? '<missing>'}, expected ${specifier}`);
      }
    }
  }
}

export function checkDependencyPolicy(root = process.cwd()) {
  const errors = [];
  const rootManifestPath = join(root, 'package.json');
  const rootManifest = existsSync(rootManifestPath) ? readJson(rootManifestPath, errors) : null;
  if (!rootManifest) {
    if (!errors.length) errors.push('root package.json is missing');
    return { ok: false, errors };
  }
  if (typeof rootManifest.packageManager !== 'string' || !/^pnpm@\d+\.\d+\.\d+$/.test(rootManifest.packageManager)) {
    errors.push('root package.json must declare packageManager as an exact pnpm version');
  }

  const apps = packageManifests(root, join(root, 'web', 'apps')).map((info) => ({ ...info, kind: 'app' }));
  const shared = packageManifests(root, join(root, 'web', 'packages')).map((info) => ({ ...info, kind: 'shared' }));
  const packages = [{ workspacePath: '.', manifest: rootManifest, kind: 'root' }, ...apps, ...shared];

  for (const packageInfo of packages.slice(1)) {
    packageInfo.manifest = readJson(packageInfo.path, errors);
  }
  const validPackages = packages.filter((packageInfo) => packageInfo.manifest);
  const byName = new Map(validPackages.slice(1).filter((packageInfo) => typeof packageInfo.manifest.name === 'string').map((packageInfo) => [packageInfo.manifest.name, packageInfo]));

  for (const packageInfo of validPackages) {
    const dependencies = new Map(allDependencies(packageInfo.manifest));
    const react = dependencies.get('react');
    const reactDom = dependencies.get('react-dom');
    if (react !== undefined || reactDom !== undefined) {
      if (react !== reactDom || !/^19\.2\.\d+$/.test(react ?? '')) {
        errors.push(`${packageInfo.workspacePath}: react and react-dom must use the same exact version in the 19.2.x line`);
      }
    }

    for (const [dependencyName, specifier] of dependencies) {
      const target = byName.get(dependencyName);
      if (packageInfo.kind === 'app' && target && specifier !== 'workspace:*') {
        errors.push(`${packageInfo.workspacePath}: internal dependency ${dependencyName} must use workspace:*`);
      }
      if (packageInfo.kind === 'shared' && target?.kind === 'app') {
        errors.push(`${packageInfo.workspacePath}: shared package ${packageInfo.manifest.name} must not depend on app ${dependencyName}`);
      }
    }
  }

  checkLockfile(root, validPackages, errors);
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
