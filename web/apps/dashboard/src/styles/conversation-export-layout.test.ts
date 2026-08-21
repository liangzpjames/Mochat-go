import { readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

const css = readFileSync('src/styles/index.css', 'utf8');

describe('conversation export workbench layout contract', () => {
  it('fills the application content area and keeps the two-step workspace readable', () => {
    expect(css).toContain('.conversation-export-page { align-content: start; background: #f7f9fc;');
    expect(css).toContain('min-height: calc(100dvh - 120px)');
    expect(css).toContain('.conversation-export-stepper');
    expect(css).toContain('align-content: start;');
    expect(css).toContain('margin: 0; max-width: none;');
    expect(css).toContain('width: 100%;');
    expect(css).toContain('.conversation-export-type-grid { display: grid;');
    expect(css).toContain('grid-template-columns: repeat(3, minmax(0, 1fr))');
  });

  it('uses explicit query and fixed twenty-item pagination', () => {
    expect(css).toContain('.conversation-export-query');
    expect(css).toContain('.conversation-export-pagination');
    expect(css).toContain('.conversation-export-table');
    expect(css).not.toContain('.conversation-export-page input:focus { transform:');
  });

  it('supports a full-screen dialog and right task drawer on narrow screens', () => {
    expect(css).toContain('.conversation-export-modal, .conversation-export-drawer-mask');
    expect(css).toContain('.conversation-export-drawer {');
    expect(css).toContain('@media (max-width: 900px)');
    expect(css).toContain('@media (max-width: 600px)');
    expect(css).toContain('.conversation-export-drawer { max-width: none; width: 100%; }');
  });

  it('keeps task rows compact instead of stretching them to fill the drawer', () => {
    expect(css).toContain('max-width: 520px; width: min(520px, 92vw);');
    expect(css).toContain('.conversation-export-task-list { align-content: start;');
    expect(css).toContain('grid-auto-rows: max-content;');
    expect(css).toContain('.conversation-export-task-metrics');
    expect(css).toContain('.conversation-export-task-actions');
  });
});
