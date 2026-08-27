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
  it('renders concise operator-facing service summaries without technical diagnostics', async () => {
    const api: ProviderStatusApi = {
      getStatus: vi.fn().mockResolvedValue({
        providers: [
          { kind: 'wecom_standard', state: 'ready', code: 'wecom.runtime_verified', source: 'external', capabilities: ['employee_sync'], lastSyncAt: '2026-08-14T08:00:00Z', lastSuccessAt: '2026-08-14T08:00:00Z', capabilityStatuses: [{ capability: 'employee_sync', state: 'ready', source: 'external', lastSuccessAt: '2026-08-14T08:00:00Z' }] },
          { kind: 'wecom_archive', state: 'limited', code: 'archive.credentials_missing', source: 'external', action: 'configure archive', reason: 'MOCHAT_ARCHIVE_SECRET', missing: ['MOCHAT_ARCHIVE_SECRET'], capabilities: ['archive_sync'], lastFailureAt: '2026-08-14T08:01:00Z', lastErrorCode: 'archive.credentials_missing', capabilityStatuses: [{ capability: 'archive_sync', state: 'limited', source: 'external', action: 'configure archive', reason: 'MOCHAT_ARCHIVE_SECRET', lastFailureAt: '2026-08-14T08:01:00Z', lastErrorCode: 'archive.credentials_missing' }] },
          { kind: 'audio_storage', state: 'unavailable', code: 'audio_storage.unavailable', source: 'local', capabilities: ['audio_object_storage'], capabilityStatuses: [] },
          { kind: 'ai', state: 'ready', code: 'ai.ready', source: 'code_only', capabilities: ['chat'], capabilityStatuses: [] },
        ],
        freshAt: '2026-08-14T08:02:00Z',
      }),
    };

    renderPage(api, true);

    expect(await screen.findByRole('heading', { name: '服务运行状态' })).toBeTruthy();
    expect(screen.getByText('企业微信')).toBeTruthy();
    expect(screen.getByText('会话存档')).toBeTruthy();
    expect(screen.getByText('文件与录音')).toBeTruthy();
    expect(screen.getByText('智能分析')).toBeTruthy();
    expect(screen.getAllByText('运行正常')).toHaveLength(2);
    expect(screen.getByText('需要完善')).toBeTruthy();
    expect(screen.getByText('暂不可用')).toBeTruthy();
    expect(screen.getAllByText('服务运行正常')).toHaveLength(2);
    expect(screen.getByText('部分功能尚未配置完成')).toBeTruthy();
    expect(screen.getByText('当前无法使用，请联系管理员')).toBeTruthy();

    for (const technicalText of ['Provider', 'external', 'local', 'code_only', 'wecom.runtime_verified', 'archive.credentials_missing', 'MOCHAT_ARCHIVE_SECRET', 'employee_sync', 'archive_sync', 'configure archive', '最近同步', '最近成功', '最近失败', '最近错误']) {
      expect(screen.queryByText(new RegExp(technicalText, 'i'))).toBeNull();
    }
  });

  it('uses business language for loading and empty states', () => {
    const pendingApi: ProviderStatusApi = { getStatus: vi.fn().mockReturnValue(new Promise(() => undefined)) };
    const { unmount } = renderPage(pendingApi);
    expect(screen.getByText('正在读取服务运行状态…')).toBeTruthy();
    unmount();

    const emptyApi: ProviderStatusApi = { getStatus: vi.fn().mockResolvedValue({ providers: [], freshAt: '2026-08-14T08:00:00Z' }) };
    renderPage(emptyApi);
    return expect(screen.findByText('暂无服务状态信息。')).resolves.toBeTruthy();
  });

  it('uses business language and retains retry for authorization and unavailable errors', async () => {
    const forbiddenApi: ProviderStatusApi = {
      getStatus: vi.fn().mockRejectedValue(new ApiError('forbidden', 'denied', { status: 403, machineCode: 'DASHBOARD_PERMISSION_DENIED' })),
    };
    const { unmount } = renderPage(forbiddenApi, true);
    expect(await screen.findByText('当前账号无权查看服务运行状态。')).toBeTruthy();
    expect(screen.getByRole('button', { name: '重试' })).toBeTruthy();
    unmount();

    const unavailableApi: ProviderStatusApi = {
      getStatus: vi.fn().mockRejectedValue(new ApiError('server', 'source unavailable', { status: 503, machineCode: 'PROVIDER_STATUS_SOURCE_UNAVAILABLE' })),
    };
    renderPage(unavailableApi, true);
    expect(await screen.findByText('服务状态暂时无法获取，请稍后重试。')).toBeTruthy();
    expect(screen.getByRole('button', { name: '重试' })).toBeTruthy();
  });

  it('uses business language and retains retry for ordinary errors', async () => {
    const api: ProviderStatusApi = { getStatus: vi.fn().mockRejectedValue(new Error('unexpected')) };
    renderPage(api, true);

    expect(await screen.findByText('服务状态暂时无法获取，请稍后重试。')).toBeTruthy();
    expect(screen.getByRole('button', { name: '重试' })).toBeTruthy();
  });
});
