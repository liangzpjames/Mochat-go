import { createHash } from 'node:crypto';
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { basename, join } from 'node:path';

import { phase2Apps } from './phase2_current_routes.mjs';

const sha256 = (path) => createHash('sha256').update(readFileSync(path)).digest('hex');

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
