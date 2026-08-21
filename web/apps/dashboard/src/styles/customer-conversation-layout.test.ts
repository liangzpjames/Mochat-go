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

  it('lets empty and loading states fill the pane instead of leaving a short card at the top', () => {
    expect(css).toMatch(/\.customer-conversation-detail\s+\.page-state[\s\S]*flex:\s*1 1 auto/s);
    expect(css).toMatch(/\.customer-conversation-list\s+\.page-state[\s\S]*flex:\s*1 1 auto/s);
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

  it('keeps customer detail statistics and filters compact on desktop', () => {
    expect(css).toMatch(/\.customer-conversation-stats\s*\{[^}]*grid-template-columns:\s*repeat\(4,/s);
    expect(css).toMatch(/\.customer-conversation-stats article\s*\{[^}]*border-right:\s*1px solid/s);
    expect(css).toMatch(/\.customer-conversation-stats article:last-child\s*\{[^}]*border-right:\s*0/s);
    expect(css).toMatch(/\.customer-conversation-detail-filters\s*\{[^}]*display:\s*grid/s);
    expect(css).toMatch(/\.customer-conversation-detail-filters\s*\{[^}]*grid-template-columns:/s);
    expect(css).toContain('.customer-conversation-message-types input:checked + span');
    expect(css).toContain('.customer-conversation-message-types input { position: absolute; opacity: 0; pointer-events: none; }');
  });

  it('matches employee conversation message rhythm and bubble typography', () => {
    expect(css).toMatch(/\.customer-conversation-messages\s*\{[^}]*background:\s*#f8fafc;[^}]*flex:\s*1;[^}]*padding:\s*15px 18px 22px;/s);
    expect(css).toMatch(/\.customer-conversation-message\s*\{[^}]*margin:\s*0 auto 13px 0;[^}]*max-width:\s*min\(74%, 720px\);/s);
    expect(css).toMatch(/\.customer-conversation-message > div\s*\{[^}]*background:\s*#fff;[^}]*font-size:\s*12px;[^}]*line-height:\s*1\.6;[^}]*padding:\s*9px 11px;/s);
    expect(css).toMatch(/\.customer-conversation-message\.outbound > div\s*\{[^}]*background:\s*#eaf2ff;[^}]*border-color:\s*#d7e5ff;[^}]*border-radius:\s*10px 4px 10px 10px;/s);
    expect(css).toMatch(/\.customer-conversation-message header strong\s*\{[^}]*color:\s*#64758a;[^}]*font-size:\s*10px;/s);
    expect(css).toMatch(/\.customer-conversation-message header time\s*\{[^}]*color:\s*#a1acb9;[^}]*font-size:\s*9px;/s);
  });
});
