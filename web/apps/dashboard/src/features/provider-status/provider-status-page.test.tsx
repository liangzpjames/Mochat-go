import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '@mochat/api-client';

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
  it('shows the source label and only the available sync diagnostics', async () => {
    const api: ProviderStatusApi = {
      getStatus: vi.fn().mockResolvedValue({
        providers: [
          { kind: 'wecom_standard', state: 'ready', code: 'wecom.runtime_verified', source: 'external', capabilities: ['employee_sync'], lastSyncAt: '2026-08-14T08:00:00Z', lastSuccessAt: '2026-08-14T08:00:00Z', capabilityStatuses: [{ capability: 'employee_sync', state: 'ready', source: 'external', lastSuccessAt: '2026-08-14T08:00:00Z' }] },
          { kind: 'simulated', state: 'limited', code: 'provider.simulated', source: 'simulated', capabilities: ['demo'], lastFailureAt: '2026-08-14T08:01:00Z', lastErrorCode: 'provider.simulated_failure', capabilityStatuses: [{ capability: 'demo', state: 'limited', source: 'simulated', lastFailureAt: '2026-08-14T08:01:00Z', lastErrorCode: 'provider.simulated_failure' }] },
          { kind: 'audio_storage', state: 'ready', code: 'audio_storage.ready', source: 'local', capabilities: ['audio_object_storage'], capabilityStatuses: [] },
          { kind: 'ai', state: 'limited', code: 'ai.disabled', source: 'code_only', capabilities: ['chat'], capabilityStatuses: [] },
        ],
        freshAt: '2026-08-14T08:02:00Z',
      }),
    };
    renderPage(api, true);

    expect(await screen.findByText(/真实外部 Provider/)).toBeTruthy();
    expect(screen.getByText(/模拟 Provider/)).toBeTruthy();
    expect(screen.getByText(/本地 Provider/)).toBeTruthy();
    expect(screen.getByText(/仅代码支持/)).toBeTruthy();
    expect(screen.getByText(/最近同步：/)).toBeTruthy();
    expect(screen.getByText(/最近成功：/)).toBeTruthy();
    expect(screen.getByText(/最近失败：/)).toBeTruthy();
    expect(screen.getByText('provider.simulated_failure')).toBeTruthy();
    expect(screen.queryByText('最近错误：', { exact: true })).toBeTruthy();
  });

  it('shows state and next action near the provider without rendering diagnostics for ordinary users', async () => {
    const api: ProviderStatusApi = {
      getStatus: vi.fn().mockResolvedValue({
        providers: [{
          kind: 'wecom_archive', state: 'limited', code: 'archive.credentials_missing', source: 'external',
          action: '请联系管理员配置或验证 Provider', reason: 'MOCHAT_ARCHIVE_SECRET', missing: ['MOCHAT_ARCHIVE_SECRET'], capabilities: ['archive_sync'], capabilityStatuses: [],
        }],
        freshAt: '2026-08-14T08:00:00Z',
      }),
    };
    renderPage(api);

    expect(await screen.findByText('会话存档')).toBeTruthy();
    expect(screen.getByText('受限')).toBeTruthy();
    expect(screen.getByText('下一步：请联系管理员配置或验证 Provider')).toBeTruthy();
    expect(screen.queryByText('MOCHAT_ARCHIVE_SECRET')).toBeNull();
  });

  it('shows non-secret diagnostics only when the server grants superadmin diagnostics', async () => {
    const api: ProviderStatusApi = {
      getStatus: vi.fn().mockResolvedValue({
        providers: [{
          kind: 'wecom_archive', state: 'limited', code: 'archive.credentials_missing', source: 'external',
          action: 'configure archive', reason: 'archive credential is missing', missing: ['archive credential'], capabilities: ['archive_sync'], capabilityStatuses: [],
        }],
        freshAt: '2026-08-14T08:00:00Z',
      }),
    };
    renderPage(api, true);

    expect(await screen.findByText('archive credential is missing')).toBeTruthy();
    expect(screen.getByText('下一步：configure archive')).toBeTruthy();
    expect(screen.getByText(/缺失项：archive credential/)).toBeTruthy();
  });

  it('distinguishes forbidden access from unavailable provider source errors', async () => {
    const forbiddenApi: ProviderStatusApi = {
      getStatus: vi.fn().mockRejectedValue(new ApiError('forbidden', 'denied', { status: 403, machineCode: 'DASHBOARD_PERMISSION_DENIED' })),
    };
    const { unmount } = renderPage(forbiddenApi, true);
    expect(await screen.findByText('当前账号无权查看 Provider 状态。')).toBeTruthy();
    unmount();

    const unavailableApi: ProviderStatusApi = {
      getStatus: vi.fn().mockRejectedValue(new ApiError('server', 'source unavailable', { status: 503, machineCode: 'PROVIDER_STATUS_SOURCE_UNAVAILABLE' })),
    };
    renderPage(unavailableApi, true);
    expect(await screen.findByText('Provider 状态来源暂不可用，请稍后重试。')).toBeTruthy();
  });
});
