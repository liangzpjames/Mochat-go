import { describe, expect, it } from 'vitest';

import migrationRoutes from '../migration-routes.json';
import {
  businessRouteCatalog,
  specializedDashboardRoutes,
} from '../features/business-workbench/catalog';

describe('Phase 2 Dashboard route completion', () => {
  it.each(migrationRoutes)('$path is switched to React', (route) => {
    expect(route.target).toBe('react');
  });

  it.each(migrationRoutes)('$path has an explicit functional implementation', (route) => {
    expect(
      specializedDashboardRoutes.has(route.path) ||
      businessRouteCatalog[route.path] !== undefined,
    ).toBe(true);
  });

  it('does not retain legacy routes', () => {
    expect(migrationRoutes.filter((route) => route.target === 'legacy')).toEqual([]);
  });
});
