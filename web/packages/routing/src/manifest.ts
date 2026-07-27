import { z } from 'zod';

export type RouteTarget = 'react' | 'legacy';

export type MigrationRoute = {
  path: string;
  target: RouteTarget;
  auth: boolean;
  corpContext: boolean;
  permission: string | null;
};

export const migrationRouteSchema = z.object({
  path: z.string().startsWith('/'),
  target: z.enum(['react', 'legacy']),
  auth: z.boolean(),
  corpContext: z.boolean(),
  permission: z.string().nullable(),
});

function canonicalPattern(path: string): string {
  return path === '/' ? path : path.replace(/\/+$/, '');
}

function patternsOverlap(left: string, right: string): boolean {
  const leftSegments = left.split('/');
  const rightSegments = right.split('/');
  return leftSegments.length === rightSegments.length
    && leftSegments.every((segment, index) => (
      segment === rightSegments[index]
      || segment.startsWith(':')
      || rightSegments[index]?.startsWith(':')
    ));
}

const routeManifestSchema = z.array(migrationRouteSchema).superRefine((routes, context) => {
  routes.forEach((route, index) => {
    const pattern = canonicalPattern(route.path);
    for (let previousIndex = 0; previousIndex < index; previousIndex += 1) {
      const previousRoute = routes[previousIndex];
      if (!previousRoute) {
        continue;
      }
      const previousPattern = canonicalPattern(previousRoute.path);
      if (previousPattern === pattern) {
        context.addIssue({
          code: 'custom',
          message: `Duplicate route pattern: ${route.path}`,
          path: [index, 'path'],
        });
      } else if (patternsOverlap(previousPattern, pattern)) {
        context.addIssue({
          code: 'custom',
          message: `Ambiguous route pattern: ${route.path}`,
          path: [index, 'path'],
        });
      }
    }
  });
});

export function parseRouteManifest(input: unknown): readonly MigrationRoute[] {
  return routeManifestSchema.parse(input);
}
