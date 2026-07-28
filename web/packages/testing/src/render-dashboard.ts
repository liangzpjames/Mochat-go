import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  render,
  type RenderOptions,
  type RenderResult,
} from '@testing-library/react';
import { createElement, type ReactElement, type ReactNode } from 'react';

export type RenderDashboardOptions = Omit<RenderOptions, 'wrapper'> & {
  queryClient?: QueryClient;
  wrapper?: (children: ReactNode) => ReactElement;
};

export function renderDashboard(
  ui: ReactElement,
  options: RenderDashboardOptions = {},
): RenderResult & { queryClient: QueryClient } {
  const queryClient = options.queryClient ?? new QueryClient({
    defaultOptions: {
      mutations: { retry: false },
      queries: { retry: false },
    },
  });
  const Wrapper = ({ children }: { children: ReactNode }) => createElement(
    QueryClientProvider,
    { client: queryClient },
    options.wrapper?.(children) ?? children,
  );
  const { queryClient: _queryClient, wrapper: _wrapper, ...renderOptions } = options;
  void _queryClient;
  void _wrapper;
  return {
    ...render(ui, { ...renderOptions, wrapper: Wrapper }),
    queryClient,
  };
}
