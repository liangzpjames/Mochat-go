/* eslint-disable @typescript-eslint/require-await -- async query tests share one setup; the no-corp case intentionally performs no await */
import { cleanup, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AiInsightPage } from './ai-insight-pages';
import type { AiInsightApi } from './ai-insight-api';
import { DashboardAccessProvider } from '../../app/access-context';

afterEach(cleanup);

const limitedResult = {
  page: 'emotion', title: '情绪识别', capability: 'limited', provider: 'none',
  limitations: ['情绪 Provider 未提供'], data: [],
};

function renderPage(api: AiInsightApi) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const access = { corp: { id: '9' }, session: {}, menu: [], allowedRoutes: new Set(), allowedActions: new Set() } as never;
  return render(
    <DashboardAccessProvider value={access}>
      <QueryClientProvider client={client}>
        <AiInsightPage api={api} page="emotion" />
      </QueryClientProvider>
    </DashboardAccessProvider>,
  );
}

describe('AI 洞察页面（受限态）', () => {
  it('展示业务化限制说明，不伪造数据或暴露技术术语', async () => {
    const api = { read: vi.fn().mockResolvedValue(limitedResult) };
    renderPage(api);
    expect((await screen.findAllByText(/当前尚未配置可用的 AI 分析服务，暂无分析结果/)).length).toBeGreaterThan(0);
    expect(screen.queryByText(/Provider/i)).toBeNull();
    expect(await screen.findByRole('link', { name: '前往接入 AI 能力' })).toBeTruthy();
  });

  it('能力未接入时展示受限状态徽标', async () => {
    const api = { read: vi.fn().mockResolvedValue(limitedResult) };
    renderPage(api);
    expect(await screen.findByText(/AI 能力状态：AI 能力未接入/)).toBeTruthy();
  });

  it('错误态可重试', async () => {
    const api = { read: vi.fn().mockRejectedValue(new Error('boom')) };
    renderPage(api);
    expect(await screen.findByText(/数据加载失败/)).toBeTruthy();
    expect(screen.getByRole('button', { name: '重新加载' })).toBeTruthy();
  });

  it('未授权 corp 时不发起请求', async () => {
    const api = { read: vi.fn() };
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { container } = render(
      <QueryClientProvider client={client}>
        <AiInsightPage api={api} page="emotion" />
      </QueryClientProvider>,
    );
    expect(container.textContent).toContain('情绪识别');
    expect(api.read).not.toHaveBeenCalled();
  });
});
