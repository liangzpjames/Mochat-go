import manifest from './migration-routes.json';

const loadPage = () => import('./page');
export const pageLoaders = Object.fromEntries(manifest.map((route) => [route.path, loadPage]));
