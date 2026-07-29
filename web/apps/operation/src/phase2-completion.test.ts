import { describe, expect, it } from 'vitest';
import manifest from './migration-routes.json';
import { operationBasename } from './deployment';
import { operationRoutes } from './app/catalog';

describe('Operation Phase 2 migration', () => {
  it.each(manifest)('$path is React and has functional activity content', (route) => {
    expect(route.target).toBe('react');
    expect(operationRoutes[route.path]).toBeDefined();
  });

  it('supports mounted and independent-port paths', () => {
    expect(operationBasename('/operation-app/workFission')).toBe('/operation-app');
    expect(operationBasename('/workFission')).toBe('/');
  });
});
