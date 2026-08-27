import { createContext, useContext, type ReactNode } from 'react';

type JsonApiClient = {
  request<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T>;
};

export type ArchiveComponentSession = { sessionUrl: string; expiresIn: number };
export type ArchiveComponentClient = {
  createSession(path: string): Promise<ArchiveComponentSession>;
};

const componentPath = /^\/dashboard\/archive\/components\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\/session$/;
const sessionPath = /^\/dashboard\/archive\/components\/session\/[A-Za-z0-9_-]{43}$/;
const ArchiveComponentClientContext = createContext<ArchiveComponentClient | null>(null);

export function createArchiveComponentClient(apiClient: JsonApiClient, origin = window.location.origin): ArchiveComponentClient {
  const trustedOrigin = new URL(origin).origin;
  return {
    async createSession(path) {
      const target = new URL(path, trustedOrigin);
      if (target.origin !== trustedOrigin || !componentPath.test(target.pathname) || target.search !== '' || target.hash !== '') {
        throw new Error('invalid archive component URL');
      }
      const result = await apiClient.request<ArchiveComponentSession>(target, { method: 'POST' });
      if (!result || !sessionPath.test(result.sessionUrl) || !Number.isInteger(result.expiresIn) || result.expiresIn < 1 || result.expiresIn > 120) {
        throw new Error('invalid archive component session');
      }
      return result;
    },
  };
}

export function ArchiveComponentClientProvider({ client, children }: { client: ArchiveComponentClient; children: ReactNode }) {
  return <ArchiveComponentClientContext.Provider value={client}>{children}</ArchiveComponentClientContext.Provider>;
}

export function useArchiveComponentClient(): ArchiveComponentClient | null {
  return useContext(ArchiveComponentClientContext);
}
