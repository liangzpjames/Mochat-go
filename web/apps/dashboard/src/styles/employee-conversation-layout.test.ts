import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const css = readFileSync('src/styles/index.css', 'utf8');
const searchButtonRule = css.match(/\.employee-conversation-directory-search button \{[\s\S]*?\n}/)?.[0] ?? '';

describe('employee conversation workspace layout', () => {
  it('uses one viewport-filling continuous three-column workspace', () => {
    expect(css).toContain('grid-template-columns: 272px 380px minmax(0, 1fr)');
    expect(css).toContain('height: calc(100dvh - 128px)');
    expect(css).toContain('.employee-conversation-pane { min-height: 0; overflow: hidden; }');
    expect(css).toContain('.employee-conversation-scroll { min-height: 0; overflow-y: auto; }');
    expect(css).not.toContain('grid-template-columns: 250px minmax(300px, 390px) minmax(0, 1fr)');
  });

  it('uses a compact department trigger and bounded popup instead of a fixed department pane', () => {
    expect(css).toContain('.employee-conversation-department-picker');
    expect(css).toContain('.employee-conversation-department-trigger');
    expect(css).toContain('.employee-conversation-department-menu');
    expect(css).toContain('max-height: 240px;');
    expect(css).not.toContain('flex: 0 0 190px;');
  });

  it('keeps directory search actions at a stable width while refreshing', () => {
    expect(searchButtonRule).toContain('width: 56px;');
    expect(searchButtonRule).toContain('min-width: 56px;');
  });

  it('declares drawer, two-column and mobile step-down breakpoints', () => {
    expect(css).toContain('@media (max-width: 1599px)');
    expect(css).toContain('@media (max-width: 1199px)');
    expect(css).toContain('@media (max-width: 768px)');
    expect(css).toContain('.employee-conversation-workspace.is-directory-open .employee-conversation-directory');
    expect(css).toContain('.employee-conversation-workspace { grid-template-columns: 360px minmax(520px, 1fr); height: calc(100dvh - 144px); }');
  });
});
