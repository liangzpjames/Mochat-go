import { describe, expect, it } from 'vitest';
import { benchmarkManifest } from './benchmark-manifest';

describe('Phase 3.4 Manifest provider status', () => {
  it('keeps the full 53-page Dashboard inventory unique and implementation-backed', () => {
    expect(benchmarkManifest.pages).toHaveLength(53);
    expect(new Set(benchmarkManifest.pages.map((page) => page.path)).size).toBe(53);
    expect(benchmarkManifest.pages.every((page) => page.implementation === 'native' || page.implementation === 'legacy-adapter')).toBe(true);
  });

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
