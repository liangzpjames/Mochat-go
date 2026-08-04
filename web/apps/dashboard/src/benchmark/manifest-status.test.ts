import { describe, expect, it } from 'vitest';
import { benchmarkManifest } from './benchmark-manifest';

describe('Phase 3.4 Manifest provider status', () => {
  it('keeps externally blocked acquisition pages partial', () => {
    const externallyBlockedPaths = new Set([
      '/acquisition/redirect-link',
      '/acquisition/wechat-customer-service',
    ]);
    const pages = benchmarkManifest.pages
      .filter((page) => externallyBlockedPaths.has(page.path))
      .map((page) => page.backend);

    expect(pages).toEqual(['partial', 'partial']);
  });
});
