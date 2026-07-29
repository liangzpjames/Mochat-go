import { describe, expect, it } from 'vitest';

import manifest from '../migration-routes.json';
import { sidebarRoutes } from './catalog';

describe('Sidebar functional route catalog', () => {
  it('covers every migrated route with route-specific content and actions', () => {
    expect(Object.keys(sidebarRoutes).sort()).toEqual(
      manifest.map((route) => route.path).sort(),
    );
    for (const config of Object.values(sidebarRoutes)) {
      expect(config.title.length).toBeGreaterThan(1);
      expect(config.sections.length).toBeGreaterThan(0);
      expect(config.actions.length).toBeGreaterThan(0);
    }
  });
});
