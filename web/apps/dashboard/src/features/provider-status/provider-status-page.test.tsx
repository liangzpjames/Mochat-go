import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { ProviderStatusApi } from './provider-status-api';
import { ProviderStatusPage } from './provider-status-page';

afterEach(cleanup);

function renderPage(api: ProviderStatusApi, isSuperAdmin = false) {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <ProviderStatusPage api={api} isSuperAdmin={isSuperAdmin} />
    </QueryClientProvider>,
  );
}

describe('provider status page', () => {
  it('shows state and next action near the provider without rendering diagnostics for ordinary users', async () => {
    const api: ProviderStatusApi = {
      getStatus: vi.fn().mockResolvedValue({
        providers: [{
          kind: 'wecom_archive', state: 'limited', code: 'archive.credentials_missing', source: 'external',
          action: '请联系管理员配置或验证 Provider', reason: 'MOCHAT_ARCHIVE_SECRET', missing: ['MOCHAT_ARCHIVE_SECRET'], capabilities: ['archive_sync'],
        }],
        freshAt: '2026-08-14T08:00:00Z',
      }),
    };
    renderPage(api);

    expect(await screen.findByText('会话存档')).toBeTruthy();
    expect(screen.getByText('受限')).toBeTruthy();
    expect(screen.getByText('请联系管理员配置或验证 Provider')).toBeTruthy();
    expect(screen.queryByText('MOCHAT_ARCHIVE_SECRET')).toBeNull();
  });

  it('shows non-secret diagnostics only when the server grants superadmin diagnostics', async () => {
    const api: ProviderStatusApi = {
      getStatus: vi.fn().mockResolvedValue({
        providers: [{
          kind: 'wecom_archive', state: 'limited', code: 'archive.credentials_missing', source: 'external',
          action: 'configure archive', reason: 'archive credential is missing', missing: ['archive credential'], capabilities: ['archive_sync'],
        }],
        freshAt: '2026-08-14T08:00:00Z',
      }),
    };
    renderPage(api, true);

    expect(await screen.findByText('archive credential is missing')).toBeTruthy();
    expect(screen.getByText(/缺失项：archive credential/)).toBeTruthy();
  });
});
