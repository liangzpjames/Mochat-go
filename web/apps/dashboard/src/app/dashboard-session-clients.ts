import { createApiClient } from '@mochat/api-client';

export type DashboardSessionClientsDeps = {
  baseUrl: string;
  getToken: () => string | null;
  onSessionInvalid: () => void;
};

export function createDashboardSessionClients(deps: DashboardSessionClientsDeps) {
  return {
    apiClient: createApiClient({
      baseUrl: deps.baseUrl,
      getToken: deps.getToken,
      onTenantAccessDenied: deps.onSessionInvalid,
      onUnauthorized: deps.onSessionInvalid,
    }),
    accessProfileClient: createApiClient({
      baseUrl: deps.baseUrl,
      getToken: deps.getToken,
      onUnauthorized: () => undefined,
    }),
  };
}
