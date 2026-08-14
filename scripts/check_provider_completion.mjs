import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const SOURCE_NAMES = new Set(['SourceExternal', 'SourceSimulated', 'SourceLocal', 'SourceCodeOnly']);

export async function checkProviderCompletion(root = process.cwd()) {
  const providerRoot = path.join(root, 'internal', 'modules', 'providers');
  const files = [
    ...(await goFiles(providerRoot)),
    ...(await goFiles(path.join(root, 'internal', 'dashboard'))),
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
      if (relative.includes('/archive/wecom/') && /\bStateReady\b/.test(source)) {
        errors.push(`${relative}: archive Provider cannot self-certify ready before getchatdata implementation`);
      }
    }
    collectRuntimeRegistrations(relative, source, registrations, errors);
  }

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

function collectRuntimeRegistrations(relative, source, registrations, errors) {
  if (relative !== 'internal/modules/providers/catalog/catalog.go') return;
  for (const body of namedFunctionBodies(source, 'NewRegistry')) {
    if (/\bif\s+(?:\(\s*(?:false|nil|0|!\s*true|0\s*==\s*1|1\s*==\s*0|1\s*!=\s*1|0\s*!=\s*0|1\s*<\s*0|0\s*>\s*1)\s*\)|(?:false|nil|0|!\s*true|0\s*==\s*1|1\s*==\s*0|1\s*!=\s*1|0\s*!=\s*0|1\s*<\s*0|0\s*>\s*1))\s*\{[\s\S]*?(?:Register|providers\.Registration)/.test(body)) {
      errors.push(`${relative}: NewRegistry contains an unreachable Provider registration branch`);
      continue;
    }
    collectRuntimeRegistrationsInBody(relative, body, registrations, errors);
  }
}

function collectRuntimeRegistrationsInBody(relative, source, registrations, errors) {
  const registeredVariables = new Set();
  for (const match of source.matchAll(/\b[A-Za-z_$][\w$]*\.Register\(\s*([A-Za-z_$][\w$]*)\s*\)/g)) {
    registeredVariables.add(match[1]);
  }
  const rangeVariables = new Set();
  for (const match of source.matchAll(/for\s+_,\s*([A-Za-z_$][\w$]*)\s*:=\s*range\s+([A-Za-z_$][\w$]*)\s*\{/g)) {
    if (source.slice(match.index, match.index + 1000).includes(`.Register(${match[1]})`)) {
      rangeVariables.add(match[1]);
      registeredVariables.add(match[2]);
    }
  }
  for (const variable of registeredVariables) {
    if (rangeVariables.has(variable)) continue;
    const escaped = escapeRegExp(variable);
    const sliceDeclaration = new RegExp(`(?:var\\s+)?${escaped}\\s*(?::=|=)\\s*\\[\\]providers\\.Registration\\s*\\{`, 'g');
    const directDeclaration = new RegExp(`(?:var\\s+)?${escaped}\\s*(?::=|=)\\s*providers\\.Registration\\s*\\{`, 'g');
    let found = false;
    for (const match of source.matchAll(sliceDeclaration)) {
      found = true;
      const openIndex = match.index + match[0].lastIndexOf('{');
      const block = balancedBlock(source, openIndex);
      const literals = registrationLiterals(block.body);
      if (literals.length > 0) {
        for (const literal of literals) addRegistration(relative, literal, registrations, errors);
      } else if (/\bKind\s*:\s*"[^"]+"/.test(block.body)) {
        addRegistration(relative, block.body, registrations, errors);
      }
    }
    for (const match of source.matchAll(directDeclaration)) {
      found = true;
      const openIndex = match.index + match[0].lastIndexOf('{');
      const block = balancedBlock(source, openIndex);
      addRegistration(relative, block.body, registrations, errors);
    }
    if (!found) errors.push(`${relative}: runtime Register call has no registration declaration for ${variable}`);
  }
}

function namedFunctionBodies(source, name) {
  const result = [];
  const pattern = new RegExp(`\\bfunc\\s+${escapeRegExp(name)}\\s*\\([^)]*\\)[^{]*\\{`, 'g');
  for (const match of source.matchAll(pattern)) {
    const openIndex = match.index + match[0].lastIndexOf('{');
    result.push(balancedBlock(source, openIndex).body);
  }
  return result;
}

function registrationLiterals(source) {
  const result = [];
  for (const match of source.matchAll(/providers\.Registration\s*\{/g)) {
    const openIndex = match.index + match[0].lastIndexOf('{');
    result.push(balancedBlock(source, openIndex).body);
  }
  return result;
}

function balancedBlock(source, openIndex) {
  let depth = 0;
  for (let index = openIndex; index < source.length; index += 1) {
    if (source[index] === '{') depth += 1;
    if (source[index] === '}') {
      depth -= 1;
      if (depth === 0) return { body: source.slice(openIndex + 1, index), end: index };
    }
  }
  return { body: source.slice(openIndex + 1), end: source.length };
}

function addRegistration(relative, body, registrations, errors) {
  const kind = body.match(/\bKind\s*:\s*"([^"]+)"/)?.[1];
  const sourceName = body.match(/\bSource\s*:\s*(?:providers\.)?(Source[A-Za-z]+)/)?.[1];
  if (!kind || !sourceName || !SOURCE_NAMES.has(sourceName)) return;
  if (registrations.has(kind)) errors.push(`duplicate Provider registration: ${kind}`);
  registrations.set(kind, { file: relative, source: sourceName });
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
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
