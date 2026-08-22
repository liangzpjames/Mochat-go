import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const css = readFileSync('src/styles/index.css', 'utf8');

describe('Phase 3.4 live code responsive layout contract', () => {
  it('keeps drawers and nested pickers inside a narrow dynamic viewport', () => {
    expect(css).toContain('@media (max-width: 640px)');
    expect(css).toContain('.phase34-live-code-drawer .phase34-detail-panel { border-radius: 0; height: 100dvh; width: 100%; }');
    expect(css).toContain('.phase34-live-code-picker { max-height: calc(100dvh - 20px); width: calc(100vw - 20px); }');
    expect(css).toContain('.phase34-live-code-picker-grid { grid-template-columns: 1fr; overflow-y: auto; }');
    expect(css).toContain('.phase34-live-code-choice-list { grid-template-columns: 1fr; }');
  });

  it('uses a bounded desktop drawer and a dedicated scrolling body', () => {
    expect(css).toContain('width: min(520px, calc(100vw - 24px))');
    expect(css).toContain('grid-template-rows: auto minmax(0, 1fr) auto');
    expect(css).toContain('.phase34-live-code-drawer-body { display: grid; align-content: start; gap: 14px; min-height: 0; overflow-y: auto;');
  });
});
