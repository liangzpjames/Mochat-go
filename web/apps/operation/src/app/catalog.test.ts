import { describe, expect, it } from 'vitest';

import manifest from '../migration-routes.json';
import { operationRoutes } from './catalog';

describe('Operation functional route catalog', () => {
  it('covers every activity route with stages and a primary interaction', () => {
    expect(Object.keys(operationRoutes).sort()).toEqual(
      manifest.map((route) => route.path).sort(),
    );
    for (const config of Object.values(operationRoutes)) {
      expect(config.title.length).toBeGreaterThan(1);
      expect(config.stages.length).toBeGreaterThan(1);
      expect(config.primaryAction.length).toBeGreaterThan(1);
    }
  });
});
