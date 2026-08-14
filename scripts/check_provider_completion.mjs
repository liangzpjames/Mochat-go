import fs from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const SOURCE_NAMES = new Set(['SourceExternal', 'SourceSimulated', 'SourceLocal', 'SourceCodeOnly']);

export async function checkProviderCompletion(root = process.cwd()) {
  const files = [
    ...(await productionGoFiles(root, 'internal/modules/providers')),
    ...(await productionGoFiles(root, 'internal/dashboard')),
  ];
  const implementationKinds = new Map();
  const registrations = new Map();
  const errors = [];

  for (const file of files) {
    const relative = path.relative(root, file).replaceAll(path.sep, '/');
    const source = stripComments(await fs.readFile(file, 'utf8'));
    if (relative !== 'internal/modules/providers/catalog/catalog.go' && /\bStatus\s*\(\s*\)\s*(?:providers\.)?Status\s*\{/.test(source)) {
      const kinds = [...source.matchAll(/\bKind\s*:\s*"([^"]+)"/g)].map((match) => match[1]);
      const knownKinds = implementationKinds.get(relative) ?? new Set();
      for (const kind of kinds) knownKinds.add(kind);
      implementationKinds.set(relative, knownKinds);
    }
  }

  collectRuntimeRegistrations(root, registrations, errors);
  errors.push(...checkArchiveStatusAST(root));

  for (const [file, kinds] of implementationKinds) {
    for (const kind of kinds) {
      if (!registrations.has(kind)) errors.push(`${file}: Provider ${kind} has no classified runtime registration`);
    }
  }

  return {
    ok: errors.length === 0,
    errors,
    providers: [...implementationKinds.entries()].flatMap(([file, kinds]) => [...kinds].map((kind) => ({ file, kind, registration: registrations.get(kind) ?? null }))),
  };
}

function checkArchiveStatusAST(root) {
  const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'provider_completion_ast.go');
  try {
    const output = execFileSync('go', ['run', script, '--root', root], {
      cwd: path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..'),
      encoding: 'utf8',
      maxBuffer: 2 * 1024 * 1024,
    });
    return JSON.parse(output).errors ?? [];
  } catch (error) {
    const output = String(error.stdout ?? '').trim();
    if (output) {
      try {
        return JSON.parse(output).errors ?? [`archive AST gate failed: ${output}`];
      } catch {
        return [`archive AST gate failed: ${output}`];
      }
    }
    return [`archive AST gate could not run: ${error.message}`];
  }
}

function collectRuntimeRegistrations(root, registrations, errors) {
  const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'catalog_contract', 'main.go');
  let output;
  try {
    output = execFileSync('go', ['run', script, '--root', root], {
      cwd: path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..'),
      encoding: 'utf8',
      maxBuffer: 2 * 1024 * 1024,
    });
  } catch (error) {
    output = String(error.stdout ?? '').trim();
    if (!output) {
      errors.push(`catalog composition AST gate could not run: ${error.message}`);
      return;
    }
  }
  let result;
  try {
    result = JSON.parse(output);
  } catch (error) {
    errors.push(`catalog composition AST gate returned invalid JSON: ${error.message}`);
    return;
  }
  errors.push(...(result.errors ?? []));
  for (const registration of result.registrations ?? []) {
    if (!registration.kind || !SOURCE_NAMES.has(registration.source)) {
      errors.push(`catalog composition returned an unclassified registration: ${JSON.stringify(registration)}`);
      continue;
    }
    if (registrations.has(registration.kind)) errors.push(`duplicate Provider registration: ${registration.kind}`);
    registrations.set(registration.kind, { file: 'internal/modules/providers/catalog/catalog.go', source: registration.source });
  }
}

function productionGoFiles(root, relativeDirectory) {
  const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'buildfiles', 'main.go');
  const output = execFileSync('go', ['run', script, '--root', root, '--dir', relativeDirectory], {
    cwd: path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..'),
    encoding: 'utf8',
    maxBuffer: 2 * 1024 * 1024,
  });
  return JSON.parse(output).map((relative) => path.resolve(root, relative));
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
