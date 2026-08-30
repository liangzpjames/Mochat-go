import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

export const phase2Apps = Object.freeze(['dashboard', 'sidebar', 'operation']);

const readJson = (path) => JSON.parse(readFileSync(path, 'utf8'));

export function currentPhase2Routes(root = process.cwd()) {
  const migrations = Object.fromEntries(phase2Apps.map((app) => [
    app,
    readJson(resolve(root, 'web', 'apps', app, 'src', 'migration-routes.json')),
  ]));
  const benchmark = readJson(resolve(root, 'web', 'apps', 'dashboard', 'src', 'benchmark', 'manifest.json'));
  const currentDashboardPaths = new Set(benchmark.pages.map((page) => page.path));

  return {
    dashboard: migrations.dashboard.filter((route) => currentDashboardPaths.has(route.path)),
    sidebar: migrations.sidebar,
    operation: migrations.operation,
  };
}

export function currentPhase2RouteCount(routes) {
  return phase2Apps.reduce((sum, app) => sum + routes[app].length, 0);
}

export function expectedPhase2PlaywrightTitles(routes) {
  return [
    'unknown legacy URL returns 404',
    ...phase2Apps.flatMap((app) => routes[app].map(
      (route) => `${app} ${route.path} renders React and records visual evidence`,
    )),
    'API requests never return the SPA document',
  ];
}
