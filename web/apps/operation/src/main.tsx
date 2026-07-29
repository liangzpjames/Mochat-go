import { createRoot } from 'react-dom/client';
import { createBrowserRouter, RouterProvider } from 'react-router';
import manifest from './migration-routes.json';
import { operationBasename } from './deployment';
import { OperationApp } from './app/operation-app';
import './styles.css';

const request = async (endpoint: string, init?: RequestInit): Promise<unknown> => {
  const response = await fetch(endpoint, init);
  return response.json() as Promise<unknown>;
};
const routes = manifest.map((route) => ({
  path: route.path,
  element: <OperationApp request={request} />,
}));
const router = createBrowserRouter([...routes, { path: '*', element: <p>页面不存在</p> }], {
  basename: operationBasename(window.location.pathname),
});
const root = document.getElementById('root');
if (root === null) throw new Error('Operation root element was not found');
createRoot(root).render(<RouterProvider router={router} />);
