import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const SOURCE_NAMES = new Set(['SourceExternal', 'SourceSimulated', 'SourceLocal', 'SourceCodeOnly']);
const PRODUCTION_GOOS = 'linux';
const PRODUCTION_GOARCH = 'amd64';

export async function checkProviderCompletion(root = process.cwd()) {
  const implementationKinds = new Map();
  const registrations = new Map();
  const errors = [];

  collectRuntimeRegistrations(root, registrations, errors);
  const statusEvidence = collectProviderStatusAST(root, errors);
  for (const evidence of statusEvidence) {
    const knownKinds = implementationKinds.get(evidence.file) ?? new Set();
    knownKinds.add(evidence.kind);
    implementationKinds.set(evidence.file, knownKinds);
  }

  for (const [file, kinds] of implementationKinds) {
    for (const kind of kinds) {
      if (!registrations.has(kind)) errors.push(`${file}: Provider ${kind} has no classified runtime registration`);
    }
  }

  const activeKinds = new Set([...implementationKinds.values()].flatMap((kinds) => [...kinds]));
  for (const [kind] of registrations) {
    const matchingEvidence = statusEvidence.filter((evidence) => evidence.kind === kind);
    const registration = registrations.get(kind);
    const sourceEvidence = matchingEvidence.filter((evidence) => !evidence.source || evidence.source === registration.source);
    if (!activeKinds.has(kind) || sourceEvidence.length === 0) {
      errors.push(`runtime registration ${kind} has no active ${registration.source} Status evidence`);
    }
  }

  return {
    ok: errors.length === 0,
    errors,
    providers: [...implementationKinds.entries()].flatMap(([file, kinds]) => [...kinds].map((kind) => ({ file, kind, registration: registrations.get(kind) ?? null }))),
  };
}

function collectProviderStatusAST(root, errors) {
  const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'provider_completion_ast.go');
  try {
    const output = execFileSync('go', ['run', script, '--root', root, '--goos', PRODUCTION_GOOS, '--goarch', PRODUCTION_GOARCH], {
      cwd: path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..'),
      encoding: 'utf8',
      maxBuffer: 2 * 1024 * 1024,
    });
    const result = JSON.parse(output);
    errors.push(...(result.errors ?? []));
    return result.statuses ?? [];
  } catch (error) {
    const output = String(error.stdout ?? '').trim();
    if (output) {
      try {
        const result = JSON.parse(output);
        errors.push(...(result.errors ?? [`provider Status AST gate failed: ${output}`]));
        return result.statuses ?? [];
      } catch {
        errors.push(`provider Status AST gate failed: ${output}`);
        return [];
      }
    }
    errors.push(`provider Status AST gate could not run: ${error.message}`);
    return [];
  }
}

function collectRuntimeRegistrations(root, registrations, errors) {
  const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'catalog_contract', 'main.go');
  let output;
  try {
    output = execFileSync('go', ['run', script, '--root', root, '--goos', PRODUCTION_GOOS, '--goarch', PRODUCTION_GOARCH], {
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

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
  const rootIndex = process.argv.indexOf('--root');
  const root = rootIndex >= 0 ? process.argv[rootIndex + 1] : process.cwd();
  const result = await checkProviderCompletion(root);
  process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
  if (!result.ok) process.exitCode = 1;
}
