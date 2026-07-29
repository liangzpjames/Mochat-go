import { lazy, Suspense } from 'react';
import { createRoot } from 'react-dom/client';
import { createBrowserRouter, RouterProvider } from 'react-router';
import manifest from './migration-routes.json';
import { pageLoaders } from './page-loaders';
import { sidebarBasename } from './deployment';
import './styles.css';

const routes = manifest.map((route) => {
  const loader = pageLoaders[route.path];
  if (loader === undefined) throw new Error(`Missing Sidebar loader for ${route.path}`);
  const Page = lazy(loader);
  return { path: route.path, element: <Suspense fallback={<p>加载中…</p>}><Page /></Suspense> };
});
const router = createBrowserRouter([...routes, { path: '*', element: <p>页面不存在</p> }], {
  basename: sidebarBasename(window.location.pathname),
});
const root = document.getElementById('root');
if (root === null) throw new Error('Sidebar root element was not found');
createRoot(root).render(<RouterProvider router={router} />);
