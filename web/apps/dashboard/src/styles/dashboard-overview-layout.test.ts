import { readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

const css = readFileSync('src/styles/index.css', 'utf8');

function rule(selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = css.match(new RegExp(`(?:^|\\n)\\s*${escaped}\\s*\\{([^}]*)\\}`));
  expect(match, `missing ${selector}`).not.toBeNull();
  return match?.[1] ?? '';
}

describe('dashboard overview cockpit layout', () => {
  it('uses a four-card snapshot and a 5:7 intelligence grid', () => {
    expect(rule('.dashboard-overview-page .overview-metric-grid')).toContain('repeat(4, minmax(0, 1fr))');
    expect(rule('.dashboard-overview-page .overview-intelligence-grid')).toContain('minmax(0, 5fr) minmax(0, 7fr)');
  });

  it('uses a 4:8 conversation workspace and scroll-safe tables', () => {
    expect(rule('.dashboard-overview-page .overview-conversation')).toContain('minmax(280px, 4fr) minmax(0, 8fr)');
    expect(rule('.dashboard-overview-page .dashboard-table-scroll')).toContain('overflow-x: auto');
  });

  it('defines tablet, mobile and reduced-motion behavior', () => {
    expect(css).toContain('@media (max-width: 1180px)');
    expect(css).toContain('@media (max-width: 720px)');
    expect(css).toContain('@media (prefers-reduced-motion: reduce)');
    expect(css).toContain('.dashboard-overview-page .overview-intelligence-grid { grid-template-columns: 1fr; }');
    expect(css).toContain('.dashboard-overview-page .overview-metric-grid { grid-template-columns: 1fr; }');
  });

  it('keeps conversation and pagination labels readable in hover states', () => {
    expect(css).toContain('.dashboard-overview-page .overview-conversation-group-active:hover');
    expect(css).toContain('.dashboard-overview-page .overview-conversation-group-active:hover > span:first-child');
    expect(css).toContain('.dashboard-pagination button[aria-current="page"]:hover:not(:disabled)');
    expect(css).toContain('.benchmark-demo-pagination button[aria-current="page"]:hover:not(:disabled)');
    expect(css).toContain('.conversation-global-pagination button[aria-current="page"]:hover:not(:disabled)');
    expect(css).toContain('.employee-conversation-pagination button[aria-current="page"]:hover:not(:disabled)');
  });

  it('keeps growth chart date labels inside the visible chart area', () => {
    expect(css).toContain('overflow-y: visible');
    expect(css).toContain('.dashboard-overview-page .dashboard-overview-chart-date');
    expect(css).toContain('flex: 0 0 72px');
    expect(css).toContain('white-space: nowrap');
  });

  it('defines compact insight, capability and data-notice modules', () => {
    expect(css).toContain('.dashboard-overview-page .overview-ai-insight-grid');
    expect(css).toContain('.dashboard-overview-page .overview-capability-grid-layout');
    expect(css).toContain('.dashboard-overview-page .overview-data-notice');
    expect(css).toContain('.dashboard-overview-page .overview-dashboard-bars-missing');
    expect(css).toContain('@media (prefers-reduced-motion: reduce)');
  });
});
