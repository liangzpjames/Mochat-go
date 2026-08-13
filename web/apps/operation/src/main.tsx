import { createMobileApiClient } from '@mochat/mobile-foundation';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router';

import '@mochat/mobile-foundation/styles/mobile.css';
import './styles.css';

import { createOperationRouter } from './app/operation-router';
import { operationBasename } from './deployment';

const apiClient = createMobileApiClient({
  basePath: '/operation',
});

const router = createOperationRouter({
  basename: operationBasename(window.location.pathname),
  request: (path, init) => apiClient.request(path, init),
});

const root = document.getElementById('root');
if (root === null) throw new Error('Operation root element was not found');
createRoot(root).render(<RouterProvider router={router} />);
