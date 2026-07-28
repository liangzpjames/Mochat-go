import { App as AntdApp, ConfigProvider } from 'antd';
import {
  QueryClient,
  QueryClientProvider,
} from '@tanstack/react-query';
import {
  StrictMode,
} from 'react';
import {
  RouterProvider,
  type RouterProviderProps,
} from 'react-router';

export function createDashboardQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      mutations: {
        retry: 0,
      },
      queries: {
        refetchOnWindowFocus: false,
        retry: 1,
      },
    },
  });
}

export type DashboardProvidersProps = {
  queryClient: QueryClient;
  router: RouterProviderProps['router'];
};

export function DashboardProviders({
  queryClient,
  router,
}: DashboardProvidersProps) {
  return (
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <ConfigProvider>
          <AntdApp>
            <RouterProvider router={router} />
          </AntdApp>
        </ConfigProvider>
      </QueryClientProvider>
    </StrictMode>
  );
}
