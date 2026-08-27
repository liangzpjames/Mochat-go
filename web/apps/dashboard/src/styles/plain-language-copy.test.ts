import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const userFacingSources = [
  'features/ai-insight/ai-insight-pages.tsx',
  'features/ai-insight/ai-insight-workspace.tsx',
  'features/dashboard-overview/dashboard-overview-widgets.tsx',
  'features/phase33/phase33-closure-pages.tsx',
  'features/phase34/acquisition-pages.tsx',
  'features/phase34/content-reach-pages.tsx',
  'features/phase34/conversion-pages.tsx',
  'features/phase34/live-code/live-code-pages.tsx',
  'features/phase34/material-management/material-management-page.tsx',
];

describe('plain-language Dashboard copy', () => {
  it.each(userFacingSources)('%s does not contain Provider in a user-facing string', (path) => {
    const source = readFileSync(resolve(process.cwd(), 'src', path), 'utf8');
    expect(source).not.toContain('Provider');
    expect(source).not.toContain('接口');
    expect(source).not.toContain('数据表');
  });
});
