import { readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

const stylesheet = readFileSync('src/styles/index.css', 'utf8');

function declarationBlock(selector: string): string {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = stylesheet.match(new RegExp(`${escapedSelector}\\s*\\{([^}]*)\\}`));

  expect(match, `missing ${selector} rule`).not.toBeNull();
  return match?.[1] ?? '';
}

describe('Dashboard independent scroll layout', () => {
  it('keeps the header fixed while sidebar and content scroll independently', () => {
    expect(declarationBlock('.dashboard-corp-frame')).toContain('height: 100vh');
    expect(declarationBlock('.dashboard-corp-frame')).toContain('overflow: hidden');
    expect(declarationBlock('.dashboard-shell')).toContain('height: 100%');
    expect(declarationBlock('.dashboard-shell')).toContain('overflow: hidden');
    expect(declarationBlock('.dashboard-body')).toContain('min-height: 0');
    expect(declarationBlock('.dashboard-sidebar')).toContain('overflow-y: auto');
    expect(declarationBlock('.dashboard-content')).toContain('overflow-y: auto');
  });
});
