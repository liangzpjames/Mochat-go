import { createContext, useContext, type ReactNode } from 'react';
import type { ApiDownload } from '@mochat/api-client';

type BinaryApiClient = {
  download(input: RequestInfo | URL, init?: RequestInit): Promise<ApiDownload>;
};

export type ArchiveMediaClient = {
  download(path: string, init?: RequestInit): Promise<ApiDownload>;
};

const ArchiveMediaClientContext = createContext<ArchiveMediaClient | null>(null);
const archiveMediaPath = /^\/dashboard\/archive\/media\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\/content$/;

export function createArchiveMediaClient(apiClient: BinaryApiClient, origin = window.location.origin): ArchiveMediaClient {
  const trustedOrigin = new URL(origin).origin;
  return {
    download(path, init) {
      const target = new URL(path, trustedOrigin);
      const validQuery = target.search === '' || target.search === '?download=1';
      if (target.origin !== trustedOrigin || !archiveMediaPath.test(target.pathname) || !validQuery || target.hash !== '') {
        return Promise.reject(new Error('invalid archive media URL'));
      }
      return apiClient.download(target, init);
    },
  };
}

export function ArchiveMediaClientProvider({ client, children }: { client: ArchiveMediaClient; children: ReactNode }) {
  return <ArchiveMediaClientContext.Provider value={client}>{children}</ArchiveMediaClientContext.Provider>;
}

export function useArchiveMediaClient(): ArchiveMediaClient | null {
  return useContext(ArchiveMediaClientContext);
}
