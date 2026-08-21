import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const css = readFileSync(resolve(process.cwd(), 'src/styles/conversation-operations.css'), 'utf8');

describe('conversation operations layout contract', () => {
  it('fills wide screens and defines the resigned employee three-column grid', () => {
    expect(css).toContain('.conversation-operations-page');
    expect(css).toMatch(/max-width:\s*none/);
    expect(css).toMatch(/grid-template-columns:\s*300px\s+minmax\(360px,\s*0\.9fr\)\s+minmax\(560px,\s*1\.35fr\)/);
  });

  it('does not move controls on hover and collapses below 1024px', () => {
    expect(css).not.toMatch(/:hover[^}]*transform\s*:/s);
    expect(css).toMatch(/@media\s*\(max-width:\s*1024px\)/);
    expect(css).toMatch(/\.resigned-employee-workspace[^\{]*\{[^}]*grid-template-columns:\s*1fr/s);
  });
});
