import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const css = readFileSync('src/styles/risk-warning-workspace.css', 'utf8');

describe('risk warning workspace layout contract', () => {
  it('uses a full-width workspace and stable wide-screen columns', () => {
    expect(css).toMatch(/\.risk-warning-workspace\s*\{[^}]*max-width:\s*none/s);
    expect(css).toMatch(/\.sensitive-word-config-workspace\s*\{[^}]*grid-template-columns:\s*260px\s+minmax\(0,\s*1fr\)/s);
    expect(css).toMatch(/\.risk-warning-drawer\s*\{[^}]*width:\s*min\(100%,\s*600px\)/s);
    expect(css).toMatch(/\.risk-warning-workspace\s+\.risk-warning-drawer-overlay\s*\{[^}]*background:\s*rgb\(23\s+35\s+61\s+\/\s*12%\)[^}]*backdrop-filter:\s*blur\(6px\)/s);
    expect(css).not.toMatch(/:hover[^}]*transform\s*:/s);
  });
});
