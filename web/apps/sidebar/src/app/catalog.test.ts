import { describe, expect, it } from 'vitest';

import manifest from '../migration-routes.json';
import { sidebarCatalog } from './catalog';

describe('Sidebar route catalog', () => {
  it('provides valid Chinese metadata without fake business claims', () => {
    expect(Object.keys(sidebarCatalog).sort()).toEqual(
      manifest.map((route) => route.path).sort(),
    );

    for (const entry of Object.values(sidebarCatalog)) {
      expect(entry.title).toMatch(/[\u4e00-\u9fff]/u);
      expect(entry.description).not.toMatch(/fake|已完成|已加载|成功/u);
    }
  });
});
