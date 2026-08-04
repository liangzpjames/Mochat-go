import { readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

const stylesheet = readFileSync('src/styles/index.css', 'utf8');

function declarationBlock(selector: string): string {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = stylesheet.match(new RegExp(`(?:^|\\n)\\s*${escapedSelector}\\s*\\{([^}]*)\\}`));

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

  it('styles the Yuanhu construction placeholder as a surfaced status card', () => {
    expect(declarationBlock('.benchmark-placeholder-page')).toContain('background: #fff');
    expect(declarationBlock('.benchmark-placeholder-page')).toContain('border-radius: 16px');
    expect(declarationBlock('.benchmark-placeholder-breadcrumb')).toContain('color: #7a8294');
    expect(declarationBlock('.benchmark-placeholder-status')).toContain('border-radius: 999px');
  });

  it('provides the Yuanhu-inspired application chrome and page surfaces', () => {
    expect(declarationBlock('.dashboard-header')).toContain('position: sticky');
    expect(declarationBlock('.dashboard-header')).toContain('z-index: 20');
    expect(declarationBlock('.dashboard-sidebar')).toContain('box-shadow: 4px 0 18px');
    expect(declarationBlock('.dashboard-content > .ant-card')).toContain('border-radius: 12px');
    expect(declarationBlock('.dashboard-content > .ant-card')).toContain('box-shadow: 0 4px 18px');
  });

  it('defines reusable page surfaces with spacing, responsive wrapping, and table overflow', () => {
    expect(declarationBlock('.dashboard-page-header')).toContain('flex-wrap: wrap');
    expect(declarationBlock('.dashboard-filter-bar')).toContain('gap: 12px');
    expect(declarationBlock('.dashboard-stat-grid')).toContain('repeat(4, minmax(0, 1fr))');
    expect(declarationBlock('.dashboard-data-card')).toContain('border-radius: 12px');
    expect(declarationBlock('.dashboard-table-actions')).toContain('flex-wrap: wrap');
    expect(declarationBlock('.dashboard-table-scroll')).toContain('overflow-x: auto');
    expect(stylesheet).toContain('.dashboard-filter-bar { flex-direction: column; }');
    expect(stylesheet).toContain('.dashboard-stat-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }');
  });

  it('does not let overview-specific filters override the shared 12px gap', () => {
    expect(declarationBlock('.dashboard-overview-filters')).not.toContain('gap: 10px');
  });

  it('uses the shared surface treatment for PageState', () => {
    expect(declarationBlock('.page-state')).toContain('border-radius: 12px');
    expect(declarationBlock('.page-state-retry')).toContain('border-radius: 8px');
  });

  it('keeps the material group modal backdrop translucent over the current page', () => {
    const backdrop = declarationBlock('.phase34-page .phase34-material-group-backdrop');

    expect(backdrop).toContain('background: rgb(17 24 39 / 42%)');
    expect(backdrop).toContain('border: 0');
    expect(backdrop).toContain('border-radius: 0');
  });
});
