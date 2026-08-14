import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const SOURCE_NAMES = new Set(['SourceExternal', 'SourceSimulated', 'SourceLocal', 'SourceCodeOnly']);

export async function checkProviderCompletion(root = process.cwd()) {
  const providerRoot = path.join(root, 'internal', 'modules', 'providers');
  const files = await goFiles(providerRoot);
  const implementationKinds = new Map();
  const registrations = new Map();
  const errors = [];

  for (const file of files) {
    const relative = path.relative(root, file).replaceAll(path.sep, '/');
    const source = stripComments(await fs.readFile(file, 'utf8'));
    if (!relative.includes('/catalog/') && /\bStatus\s*\(\s*\)\s*(?:providers\.)?Status\s*\{/.test(source)) {
      const kinds = [...source.matchAll(/\bKind\s*:\s*"([^"]+)"/g)].map((match) => match[1]);
      for (const kind of kinds) implementationKinds.set(relative, kind);
      if (relative.includes('/archive/wecom/') && /\bStateReady\b/.test(source)) {
        errors.push(`${relative}: archive Provider cannot self-certify ready before getchatdata implementation`);
      }
    }
    for (const match of source.matchAll(/Registration\s*\{([\s\S]*?)\}/g)) {
      const body = match[1];
      const kind = body.match(/\bKind\s*:\s*"([^"]+)"/)?.[1];
      const sourceName = body.match(/\bSource\s*:\s*(?:providers\.)?(Source[A-Za-z]+)/)?.[1];
      if (kind && sourceName && SOURCE_NAMES.has(sourceName)) {
        const registrationFile = relative;
        if (registrations.has(kind)) errors.push(`duplicate Provider registration: ${kind}`);
        registrations.set(kind, { file: registrationFile, source: sourceName });
      }
    }
  }

  for (const [file, kind] of implementationKinds) {
    if (!registrations.has(kind)) errors.push(`${file}: Provider ${kind} has no classified runtime registration`);
  }

  return {
    ok: errors.length === 0,
    errors,
    providers: [...implementationKinds.entries()].map(([file, kind]) => ({ file, kind, registration: registrations.get(kind) ?? null })),
  };
}

async function goFiles(directory) {
  const result = [];
  let entries;
  try {
    entries = await fs.readdir(directory, { withFileTypes: true });
  } catch {
    return result;
  }
  for (const entry of entries) {
    const fullPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === 'fixtures' || entry.name === 'testdata' || entry.name === 'vendor') continue;
      result.push(...await goFiles(fullPath));
      continue;
    }
    if (entry.isFile() && entry.name.endsWith('.go') && !entry.name.endsWith('_test.go')) result.push(fullPath);
  }
  return result;
}

function stripComments(source) {
  return source.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/[^\r\n]*/g, '');
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
  const rootIndex = process.argv.indexOf('--root');
  const root = rootIndex >= 0 ? process.argv[rootIndex + 1] : process.cwd();
  const result = await checkProviderCompletion(root);
  process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
  if (!result.ok) process.exitCode = 1;
}
