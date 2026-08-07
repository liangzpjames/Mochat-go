import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useMemo,
  useState,
} from 'react';

export type DashboardSessionActions = {
  isLoggingOut: boolean;
  logout: () => Promise<void>;
  userId: string | null;
  userName: string | null;
};

type DashboardSessionActionsProviderProps = {
  children: ReactNode;
  onLogout: () => Promise<void>;
  userId: string | null;
  userName?: string | null;
};

type LogoutDependencies = {
  clearQueries: () => void;
  clearSession: () => void;
  invalidateServerSession: () => Promise<void>;
  navigateToLogin: () => void;
};

const DashboardSessionActionsContext =
  createContext<DashboardSessionActions | null>(null);

export function createLogoutAction({
  clearQueries,
  clearSession,
  invalidateServerSession,
  navigateToLogin,
}: LogoutDependencies): () => Promise<void> {
  return async () => {
    try {
      await invalidateServerSession();
    } catch {
      // Local logout must remain available when the server is unreachable.
    } finally {
      clearSession();
      clearQueries();
      navigateToLogin();
    }
  };
}

export function DashboardSessionActionsProvider({
  children,
  onLogout,
  userId,
  userName = null,
}: DashboardSessionActionsProviderProps) {
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const handleLogout = useCallback(async () => {
    if (isLoggingOut) return;
    setIsLoggingOut(true);
    try {
      await onLogout();
    } finally {
      setIsLoggingOut(false);
    }
  }, [isLoggingOut, onLogout]);
  const value = useMemo<DashboardSessionActions>(() => ({
    isLoggingOut,
    logout: handleLogout,
    userId,
    userName,
  }), [handleLogout, isLoggingOut, userId, userName]);

  return (
    <DashboardSessionActionsContext.Provider value={value}>
      {children}
    </DashboardSessionActionsContext.Provider>
  );
}

export function useDashboardSessionActions(): DashboardSessionActions {
  const value = useContext(DashboardSessionActionsContext);
  if (value === null) {
    throw new Error('Dashboard session actions provider is missing');
  }
  return value;
}
