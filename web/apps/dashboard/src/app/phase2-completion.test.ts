import { describe, expect, it } from 'vitest';

import migrationRoutes from '../migration-routes.json';
import { dashboardPageLoaders } from '../pages/dashboard-page-loaders';

describe('Phase 2 Dashboard route completion', () => {
  it.each(migrationRoutes)('$path is switched to React', (route) => {
    expect(route.target).toBe('react');
  });

  it.each(migrationRoutes)('$path has a page-level dynamic import', (route) => {
    expect(dashboardPageLoaders[route.path]).toEqual(expect.any(Function));
  });

  it('does not retain legacy routes', () => {
    expect(migrationRoutes.filter((route) => route.target === 'legacy')).toEqual([]);
  });
});
