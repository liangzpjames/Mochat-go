import { createMobileApiClient } from '@mochat/mobile-foundation';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router';

import '@mochat/mobile-foundation/styles/mobile.css';
import './styles.css';

import { createSidebarRouter } from './app/sidebar-router';
import {
  createCookieSidebarSessionAdapter,
  createSessionStorageSidebarSessionAdapter,
  documentCookieAdapter,
  readSidebarSession,
} from './auth/sidebar-session';
import { sidebarBasename } from './deployment';
import { createWeComBridge, type WeComSDK } from './wecom/wecom-bridge';

const basename = sidebarBasename(window.location.pathname);
const session = basename === '/sidebar-app'
  ? createCookieSidebarSessionAdapter(
      documentCookieAdapter,
      window.location.protocol === 'https:',
    )
  : createSessionStorageSidebarSessionAdapter(window.sessionStorage);

const apiClient = createMobileApiClient({
  basePath: '/sidebar',
  getToken: () => readSidebarSession(session).token,
});
const request = <T,>(path: string, init?: RequestInit) => apiClient.request<T>(path, init);
const bridge = createWeComBridge({
  request,
  agentId: () => readSidebarSession(session).agentId,
  href: () => window.location.href,
  sdk: () => (window as Window & { ww?: WeComSDK }).ww,
});

const router = createSidebarRouter({
  basename,
  session,
  origin: window.location.origin,
  request,
  bridge,
});

const root = document.getElementById('root');
if (root === null) throw new Error('Sidebar root element was not found');
createRoot(root).render(<RouterProvider router={router} />);
