import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const css = readFileSync('src/styles/index.css', 'utf8');

describe('group conversation workspace layout contract', () => {
  it('fills a wide desktop with stable three-column geometry', () => {
    expect(css).toContain('@media (min-width: 1600px)');
    expect(css).toContain('grid-template-columns: 320px minmax(720px, 1fr) 300px');
    expect(css).toContain('.group-conversation-profile.is-drawer-open { background: transparent;');
    expect(css).toContain('.group-conversation-profile-backdrop { display: none; }');
    expect(css).toContain('.group-conversation-workspace.is-profile-closed { grid-template-columns: 320px minmax(0, 1fr); }');
    expect(css).toContain('.group-conversation-workspace {');
    expect(css).toContain('min-width: 0');
    expect(css).toContain('overflow-x: hidden');
  });

  it('keeps narrow layouts in drawers/single column and avoids blur or scale jitter', () => {
    expect(css).toContain('@media (max-width: 1599px)');
    expect(css).toContain('@media (max-width: 1199px)');
    expect(css).toContain('@media (max-width: 768px)');
    expect(css).toContain('.group-conversation-profile.is-drawer-open');
    expect(css).not.toContain('.group-conversation-room-card:hover { transform:');
    expect(css).not.toContain('.group-conversation-room-card:hover { filter: blur');
  });
});
