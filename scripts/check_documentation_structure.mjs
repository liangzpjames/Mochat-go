import {
  existsSync,
  readdirSync,
  readFileSync,
  statSync,
} from 'node:fs';
import { dirname, extname, join, relative, resolve, sep } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const REQUIRED_FILES = [
  'docs/PROJECT_PROGRESS.zh-CN.md',
  'docs/phases/phase-pre0-standalone/README.md',
  'docs/phases/phase-0-go-foundation/README.md',
  'docs/phases/phase-1-frontend-foundation/README.md',
  'docs/phases/phase-2-frontend-migration/README.md',
];

function toPosix(path) {
  return path.split(sep).join('/');
}

function listFiles(directory) {
  if (!existsSync(directory)) {
    return [];
  }
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    return entry.isDirectory() ? listFiles(path) : [path];
  });
}

function linkTarget(rawTarget) {
  const withoutTitle = rawTarget.trim().split(/\s+["']/u, 1)[0];
  const withoutAnchor = withoutTitle.split('#', 1)[0];
  try {
    return decodeURIComponent(withoutAnchor);
  } catch {
    return withoutAnchor;
  }
}

function isIgnoredLink(target) {
  return target === ''
    || target.startsWith('#')
    || /^(?:https?:|mailto:|data:)/iu.test(target)
    || /^[a-zA-Z]:[\\/]/u.test(target)
    || target.startsWith('/');
}

function collectBrokenLinks(root, markdownFiles) {
  const broken = [];
  const markdownLink = /!?\[[^\]]*\]\(([^)]+)\)/gu;
  for (const file of markdownFiles) {
    const content = readFileSync(file, 'utf8');
    for (const match of content.matchAll(markdownLink)) {
      const rawTarget = match[1].trim();
      if (isIgnoredLink(rawTarget)) {
        continue;
      }
      const target = linkTarget(rawTarget);
      if (target !== '' && !existsSync(resolve(dirname(file), target))) {
        broken.push({
          file: toPosix(relative(root, file)),
          target: rawTarget,
        });
      }
    }
  }
  return broken;
}

export function checkDocumentation(root) {
  const docsDirectory = join(root, 'docs');
  const files = listFiles(docsDirectory);
  const missingRequired = REQUIRED_FILES.filter(
    (path) => !existsSync(join(root, path)),
  );
  const forbiddenArtifacts = files
    .filter((path) => / 2\./u.test(path) || statSync(path).size === 0)
    .map((path) => toPosix(relative(root, path)))
    .sort();
  const markdownFiles = files.filter((path) => extname(path) === '.md');
  const brokenLinks = collectBrokenLinks(root, markdownFiles);

  return {
    missingRequired,
    forbiddenArtifacts,
    brokenLinks,
  };
}

function printGroup(label, values) {
  if (values.length === 0) {
    return;
  }
  process.stderr.write(`${label}:\n`);
  for (const value of values) {
    process.stderr.write(`- ${typeof value === 'string' ? value : JSON.stringify(value)}\n`);
  }
}

function main() {
  const root = resolve(fileURLToPath(new URL('..', import.meta.url)));
  const result = checkDocumentation(root);
  printGroup('missing required documentation', result.missingRequired);
  printGroup('forbidden documentation artifacts', result.forbiddenArtifacts);
  printGroup('broken documentation links', result.brokenLinks);

  const failed = Object.values(result).some((values) => values.length > 0);
  if (failed) {
    process.exitCode = 1;
    return;
  }
  process.stdout.write('documentation structure: ok\n');
}

if (
  process.argv[1]
  && import.meta.url === pathToFileURL(resolve(process.argv[1])).href
) {
  main();
}
