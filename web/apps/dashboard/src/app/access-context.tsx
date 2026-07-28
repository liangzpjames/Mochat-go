import {
  createContext,
  type ReactNode,
  useContext,
} from 'react';

import type { AccessContext } from './access-loader';

const DashboardAccessContext = createContext<AccessContext | null>(null);

export function DashboardAccessProvider({
  children,
  value,
}: {
  children: ReactNode;
  value: AccessContext;
}) {
  return (
    <DashboardAccessContext.Provider value={value}>
      {children}
    </DashboardAccessContext.Provider>
  );
}

export function useDashboardAccess(): AccessContext {
  const value = useContext(DashboardAccessContext);
  if (value === null) {
    throw new Error('Dashboard access context is unavailable');
  }
  return value;
}
