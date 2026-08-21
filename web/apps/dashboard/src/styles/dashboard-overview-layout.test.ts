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
  it('uses a four-card snapshot and compact five-card insight grid', () => {
    expect(rule('.dashboard-overview-page .overview-metric-grid')).toContain('repeat(4, minmax(0, 1fr))');
    expect(css).toContain('.dashboard-overview-page .overview-ai-insight-grid');
    expect(css).toContain('grid-template-columns: repeat(5, minmax(0, 1fr))');
  });

  it('uses a 5:7 conversation and quality workspace with scroll-safe tables', () => {
    expect(css).toContain('grid-template-columns: minmax(350px, 5fr) minmax(0, 7fr)');
    expect(css).toContain('.dashboard-overview-page .overview-conversation-chart .dashboard-overview-chart-column');
    expect(css).toContain('flex: 1 1 0');
    expect(css).toContain('.dashboard-overview-page .overview-quality-workspace');
    expect(css).toContain('.dashboard-overview-page .overview-quality-trend');
    expect(css).toContain('.dashboard-overview-page .overview-quality-trend-bars');
    expect(css).toContain('grid-template-columns: repeat(7, minmax(36px, 1fr))');
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

  it('shows a stable pointer and keyboard focus state for clickable quality cards', () => {
    expect(rule('.dashboard-overview-page .overview-quality-grid .overview-metric-link')).toContain('cursor: pointer');
    expect(rule('.dashboard-overview-page .overview-quality-grid .overview-metric-link')).toContain('text-decoration: none');
    expect(css).toContain('.dashboard-overview-page .overview-quality-grid .overview-metric-link:focus-visible');
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
    expect(css).toContain('grid-template-columns: minmax(0, 1fr) minmax(360px, 1.08fr)');
    expect(css).toContain('.dashboard-overview-page .overview-trajectory-panel');
    expect(css).toContain('grid-column: 2');
    expect(css).toContain('.dashboard-overview-page .overview-module-actions');
    expect(css).toContain('.dashboard-overview-page .overview-data-notice');
    expect(css).toContain('.dashboard-overview-page .overview-dashboard-bars-missing');
    expect(css).toContain('@media (prefers-reduced-motion: reduce)');
  });

  it('keeps the trajectory as a full-height right sidebar beside the overview column', () => {
    expect(css).toContain('.dashboard-overview-page .overview-dashboard-main');
    expect(css).toContain('.dashboard-overview-page .overview-dashboard > .overview-trajectory-panel');
    expect(css).toContain('grid-template-columns: minmax(0, 1fr) minmax(380px, .62fr)');
    expect(css).toContain('grid-template-columns: minmax(0, 1fr) minmax(280px, .78fr)');
    expect(css).toContain('height: calc(100vh - 136px)');
  });
});
