import { createRoot } from 'react-dom/client';

import { createAuthStore } from '@mochat/auth';

import { createDashboardQueryClient, DashboardProviders } from './app/providers';
import { createDashboardRouter } from './app/router';
import './styles/index.css';

const rootElement = document.getElementById('root');
if (rootElement === null) {
  throw new Error('Dashboard root element was not found');
}

const authStore = createAuthStore();
const queryClient = createDashboardQueryClient();
const router = createDashboardRouter({
  getSession: () => authStore.getSession(),
  loadInitialData: () => Promise.resolve(),
});

createRoot(rootElement).render(
  <DashboardProviders queryClient={queryClient} router={router} />,
);
