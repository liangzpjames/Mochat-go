import { createHash } from 'node:crypto';
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { basename, join, relative, sep } from 'node:path';

import { phase2Apps } from './phase2_current_routes.mjs';

const sha256 = (path) => createHash('sha256').update(readFileSync(path)).digest('hex');
const excludedBuildInputNames = new Set(['coverage', 'dist', 'node_modules', 'test-results']);

const normalizedRelativePath = (root, path) => relative(root, path).split(sep).join('/');

const collectFiles = (root, directory, result) => {
  if (!existsSync(directory)) return;
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    if (excludedBuildInputNames.has(entry.name) || entry.name === '.vite') continue;
    const path = join(directory, entry.name);
    if (entry.isDirectory()) collectFiles(root, path, result);
    else if (entry.isFile() && !entry.name.endsWith('.tsbuildinfo')) result.push(path);
  }
};

export function phase2BuildInputFingerprint(root = process.cwd()) {
  const files = [];
  for (const name of ['package.json', 'pnpm-lock.yaml', 'pnpm-workspace.yaml']) {
    const path = join(root, name);
    if (existsSync(path)) files.push(path);
  }
  for (const app of phase2Apps) collectFiles(root, join(root, 'web', 'apps', app), files);
  collectFiles(root, join(root, 'web', 'packages'), files);

  const hash = createHash('sha256');
  for (const path of files.sort((left, right) => (
    normalizedRelativePath(root, left).localeCompare(normalizedRelativePath(root, right), 'en')
  ))) {
    hash.update(normalizedRelativePath(root, path));
    hash.update('\0');
    hash.update(readFileSync(path));
    hash.update('\0');
  }
  return hash.digest('hex');
}

export function phase2BuildOutputMtimeRange(root = process.cwd()) {
  const mtimes = [];
  for (const app of phase2Apps) {
    const dist = join(root, 'web', 'apps', app, 'dist');
    const index = join(dist, 'index.html');
    const assets = join(dist, 'assets');
    if (!existsSync(index) || !existsSync(assets)) throw new Error(`${app} production build output is missing`);
    mtimes.push(statSync(index).mtimeMs);
    for (const name of readdirSync(assets)) {
      const path = join(assets, name);
      if (statSync(path).isFile()) mtimes.push(statSync(path).mtimeMs);
    }
  }
  if (mtimes.length === 0) throw new Error('Phase 2 production build has no output files');
  return { oldest: Math.min(...mtimes), newest: Math.max(...mtimes) };
}

export function validatePhase2BuildProvenance({
  marker,
  currentSourceFingerprint,
  oldestOutputMtimeMs,
  playwrightStartedAtMs,
}) {
  if (marker?.schemaVersion !== 1 || marker.command !== 'corepack pnpm build') {
    throw new Error('Phase 2 build provenance contract is invalid');
  }
  const startedAtMs = Date.parse(marker.startedAt);
  const completedAtMs = Date.parse(marker.completedAt);
  if (!Number.isFinite(startedAtMs) || !Number.isFinite(completedAtMs) || completedAtMs < startedAtMs) {
    throw new Error('Phase 2 build provenance timestamps are invalid');
  }
  if (!/^[a-f0-9]{64}$/.test(marker.sourceFingerprint ?? '')
    || marker.sourceFingerprint !== currentSourceFingerprint) {
    throw new Error('Phase 2 build source fingerprint does not match the current source');
  }
  if (oldestOutputMtimeMs !== undefined
    && (!Number.isFinite(oldestOutputMtimeMs) || oldestOutputMtimeMs < startedAtMs)) {
    throw new Error('Phase 2 production build output predates the Phase 2 build');
  }
  if (playwrightStartedAtMs !== undefined
    && (!Number.isFinite(playwrightStartedAtMs) || playwrightStartedAtMs < completedAtMs)) {
    throw new Error('Playwright started before the Phase 2 build completed');
  }
  return { startedAtMs, completedAtMs };
}

export function derivePhase2BuildAudit(root = process.cwd()) {
  return Object.fromEntries(phase2Apps.map((app) => {
    const dist = join(root, 'web', 'apps', app, 'dist');
    const assets = join(dist, 'assets');
    const index = join(dist, 'index.html');
    const allNames = existsSync(assets) ? readdirSync(assets).sort() : [];
    const entryAssets = existsSync(index)
      ? Array.from(new Set(readFileSync(index, 'utf8').match(/assets\/[^"' ]+/g) ?? []))
        .map((file) => basename(file))
      : [];
    const entryNames = new Set(entryAssets);
    const javascriptAssets = allNames.filter((name) => name.endsWith('.js'));
    const audit = {
      entryAssets,
      totalBytes: allNames.reduce((sum, name) => sum + statSync(join(assets, name)).size, 0),
      javascriptAssets,
      lazyChunks: javascriptAssets.filter((name) => !entryNames.has(name)),
      assets: allNames.map((name) => ({
        name,
        bytes: statSync(join(assets, name)).size,
        sha256: sha256(join(assets, name)),
      })),
    };
    if (!audit.entryAssets.some((name) => name.endsWith('.js'))) {
      throw new Error(`${app} has no JavaScript entry asset`);
    }
    if (audit.javascriptAssets.length === 0) throw new Error(`${app} has no JavaScript asset`);
    return [app, audit];
  }));
}
