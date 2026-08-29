import { describe, expect, it } from 'vitest';

import migrationRoutes from '../../migration-routes.json';
import { businessRouteCatalog, specializedDashboardRoutes } from './catalog';

describe('Dashboard business route catalog', () => {
  it('leaves the unique company-profile route to its dedicated module', () => {
    expect(specializedDashboardRoutes.has('/company-setting/website')).toBe(true);
    expect(businessRouteCatalog['/company-setting/website']).toBeUndefined();
  });

  it('defines a route-specific module for every non-specialized route', () => {
    const expected = migrationRoutes
      .map((route) => route.path)
      .filter((path) => !specializedDashboardRoutes.has(path))
      .sort();

    expect(Object.keys(businessRouteCatalog).sort()).toEqual(expected);
  });

  it('provides business copy, an API contract and useful controls for every route', () => {
    for (const [path, config] of Object.entries(businessRouteCatalog)) {
      expect(config.path).toBe(path);
      expect(config.title.length).toBeGreaterThan(1);
      expect(config.description.length).toBeGreaterThan(3);
      expect(config.readEndpoint).toMatch(/^\//);
      expect(config.fields.length).toBeGreaterThan(0);
      expect(config.actions.length).toBeGreaterThan(0);
    }
  });
});
