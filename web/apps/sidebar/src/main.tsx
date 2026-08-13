import { createMobileApiClient } from '@mochat/mobile-foundation';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router';

import '@mochat/mobile-foundation/styles/mobile.css';
import './styles.css';

import { createSidebarRouter } from './app/sidebar-router';
import {
  documentCookieAdapter,
  readSidebarSession,
} from './auth/sidebar-session';
import { sidebarBasename } from './deployment';

const apiClient = createMobileApiClient({
  basePath: '/sidebar',
  getToken: () => readSidebarSession(documentCookieAdapter).token,
});

const router = createSidebarRouter({
  basename: sidebarBasename(window.location.pathname),
  cookies: documentCookieAdapter,
  origin: window.location.origin,
  request: (path, init) => apiClient.request(path, init),
  secure: window.location.protocol === 'https:',
});

const root = document.getElementById('root');
if (root === null) throw new Error('Sidebar root element was not found');
createRoot(root).render(<RouterProvider router={router} />);
