import manifest from '../migration-routes.json';
import { operationCatalog } from '../app/catalog';

export type OperationModuleKey =
  | 'operation-home'
  | 'activity-explain'
  | 'lottery-activity'
  | 'room-clock-in-activity'
  | 'room-fission-activity'
  | 'room-fission-progress'
  | 'room-infinite-pull-activity'
  | 'shop-code-activity'
  | 'work-fission-activity'
  | 'work-fission-progress';

export type OperationRouteRegistration = {
  path: string;
  moduleKey: OperationModuleKey;
  auth: boolean;
  activityKind: 'workFission' | null;
  requiredParams: readonly string[];
  requiresActivitySession: boolean;
  title: string;
  description: string;
};

const routeDefinitions = {
  '/': { moduleKey: 'operation-home', activityKind: null, requiredParams: [], requiresActivitySession: false },
  '/explain': { moduleKey: 'activity-explain', activityKind: null, requiredParams: [], requiresActivitySession: false },
  '/lottery': { moduleKey: 'lottery-activity', activityKind: null, requiredParams: [], requiresActivitySession: false },
  '/roomClockIn': { moduleKey: 'room-clock-in-activity', activityKind: null, requiredParams: [], requiresActivitySession: false },
  '/roomFission': { moduleKey: 'room-fission-activity', activityKind: null, requiredParams: [], requiresActivitySession: false },
  '/fissionSpeed': { moduleKey: 'room-fission-progress', activityKind: null, requiredParams: [], requiresActivitySession: false },
  '/roomInfinitePull': { moduleKey: 'room-infinite-pull-activity', activityKind: null, requiredParams: [], requiresActivitySession: false },
  '/shopCode': { moduleKey: 'shop-code-activity', activityKind: null, requiredParams: [], requiresActivitySession: false },
  '/workFission': { moduleKey: 'work-fission-activity', activityKind: 'workFission', requiredParams: ['id'], requiresActivitySession: true },
  '/speed': { moduleKey: 'work-fission-progress', activityKind: null, requiredParams: [], requiresActivitySession: false },
} as const satisfies Record<string, {
  moduleKey: OperationModuleKey;
  activityKind: 'workFission' | null;
  requiredParams: readonly string[];
  requiresActivitySession: boolean;
}>;

type RegisteredPath = keyof typeof routeDefinitions;

export const operationRouteRegistry: readonly OperationRouteRegistration[] = manifest.map((route) => {
  const path = route.path as RegisteredPath;
  const definition = routeDefinitions[path];
  const catalogEntry = operationCatalog[path];
  if (definition === undefined || catalogEntry === undefined) {
    throw new Error(`Operation route ${route.path} is not registered consistently.`);
  }
  return {
    path,
    moduleKey: definition.moduleKey,
    auth: route.auth,
    activityKind: definition.activityKind,
    requiredParams: definition.requiredParams,
    requiresActivitySession: definition.requiresActivitySession,
    title: catalogEntry.title,
    description: catalogEntry.description,
  };
});
