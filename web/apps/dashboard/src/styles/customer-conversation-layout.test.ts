import { readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

const css = readFileSync('src/styles/index.css', 'utf8');

describe('customer conversation workspace layout contract', () => {
  it('keeps the wide workspace in three fixed semantic columns with bounded height', () => {
    expect(css).toMatch(/\.customer-conversation-workspace\s*\{[^}]*grid-template-columns:\s*288px 360px minmax\(560px,\s*1fr\)/s);
    expect(css).toMatch(/\.customer-conversation-workspace\s*\{[^}]*height:\s*calc\(100dvh\s*-\s*72px\s*-\s*32px\)/s);
    expect(css).toMatch(/\.customer-conversation-scroll\s*\{[^}]*overflow-y:\s*auto/s);
  });

  it('provides drawer and single-column fallbacks for narrow screens', () => {
    expect(css).not.toMatch(/@media\s*\(max-width:\s*1599px\)[\s\S]{0,500}\.customer-conversation-directory[\s\S]{0,250}position:\s*fixed/s);
    expect(css).toMatch(/@media\s*\(max-width:\s*1199px\)[\s\S]*\.customer-conversation-directory[^}]*position:\s*fixed/s);
    expect(css).toMatch(/@media\s*\(max-width:\s*1199px\)[\s\S]*customer-conversation-workspace/s);
    expect(css).toMatch(/@media\s*\(max-width:\s*768px\)[\s\S]*\.customer-conversation-workspace[^}]*grid-template-columns:\s*1fr/s);
  });

  it('keeps the action bar inside the available desktop content width', () => {
    expect(css).toMatch(/@media\s*\(min-width:\s*1200px\)\s*and\s*\(max-width:\s*1399px\)[\s\S]*\.customer-conversation-workspace[\s\S]*grid-template-columns:\s*240px\s+minmax\(260px,\s*1fr\)\s+minmax\(320px,\s*1\.2fr\)/s);
  });

  it('defines keyboard, state and pagination contrast hooks for customer controls', () => {
    expect(css).toContain('.customer-conversation-card.is-selected');
    expect(css).toContain('.customer-conversation-card:focus-visible');
    expect(css).toContain('.customer-conversation-capability-alert');
    expect(css).toContain('.customer-conversation-profile-warning');
    expect(css).toContain('.customer-conversation-directory-filters button[aria-pressed="true"]');
    expect(css).toContain('.customer-conversation-list .dashboard-pagination button:hover:not(:disabled)');
    expect(css).toContain('.customer-conversation-list .dashboard-pagination button:disabled');
    expect(css).toContain(".customer-conversation-customer-list[role='list']");
    expect(css).toContain('.customer-conversation-action-button');
    expect(css).toContain('.customer-conversation-action-button--primary');
    expect(css).toContain('.customer-conversation-action-button:focus-visible');
    expect(css).toContain('.customer-conversation-action-button:disabled');
    expect(css).toContain('.customer-conversation-action-button svg');
    expect(css).toMatch(/\.customer-conversation-detail-header\s*\{[^}]*display:\s*flex/s);
  });
});
