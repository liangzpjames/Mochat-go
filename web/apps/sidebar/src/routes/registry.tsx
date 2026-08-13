import manifest from '../migration-routes.json';
import { sidebarCatalog } from '../app/catalog';

export type SidebarModuleKey =
  | 'sidebar-home'
  | 'sidebar-auth-callback'
  | 'sidebar-code-auth'
  | 'contact-summary'
  | 'contact-edit-detail'
  | 'contact-remark'
  | 'contact-setting-tag'
  | 'contact-batch-add'
  | 'contact-sop'
  | 'sidebar-login'
  | 'medium-library'
  | 'room-sop';

export type SidebarRouteRegistration = {
  path: string;
  moduleKey: SidebarModuleKey;
  auth: boolean;
  title: string;
  description: string;
};

const routeDefinitions = {
  '/': { moduleKey: 'sidebar-home', auth: true },
  '/auth': { moduleKey: 'sidebar-auth-callback', auth: false },
  '/codeAuth': { moduleKey: 'sidebar-code-auth', auth: false },
  '/contact': { moduleKey: 'contact-summary', auth: true },
  '/contact/editDetail': { moduleKey: 'contact-edit-detail', auth: true },
  '/contact/remark': { moduleKey: 'contact-remark', auth: true },
  '/contact/settingTag': { moduleKey: 'contact-setting-tag', auth: true },
  '/contactBatchAdd': { moduleKey: 'contact-batch-add', auth: true },
  '/contactSop': { moduleKey: 'contact-sop', auth: true },
  '/login': { moduleKey: 'sidebar-login', auth: false },
  '/medium': { moduleKey: 'medium-library', auth: true },
  '/roomSop': { moduleKey: 'room-sop', auth: true },
} as const satisfies Record<string, { moduleKey: SidebarModuleKey; auth: boolean }>;

type RegisteredPath = keyof typeof routeDefinitions;

export const sidebarRouteRegistry: readonly SidebarRouteRegistration[] = manifest.map((route) => {
  const path = route.path as RegisteredPath;
  const definition = routeDefinitions[path];
  const catalogEntry = sidebarCatalog[path];
  if (definition === undefined || catalogEntry === undefined || definition.auth !== route.auth) {
    throw new Error(`Sidebar route ${route.path} is not registered consistently.`);
  }
  return {
    path,
    moduleKey: definition.moduleKey,
    auth: definition.auth,
    title: catalogEntry.title,
    description: catalogEntry.description,
  };
});
