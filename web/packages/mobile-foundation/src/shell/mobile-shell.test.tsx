import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { MobileShell } from './mobile-shell';
import {
  MobileActionDock,
  MobileErrorBoundary,
  MobileState,
} from './mobile-state';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('MobileShell', () => {
  it('renders semantic header and main content without app-specific knowledge', () => {
    render(
      <MobileShell
        appName="当前应用"
        title="联系人"
        subtitle="客户摘要"
        leading={<button type="button">返回</button>}
      >
        <p>联系人内容</p>
      </MobileShell>,
    );

    expect(screen.getByRole('banner').textContent).toContain('当前应用');
    expect(screen.getByRole('banner').textContent).toContain('联系人');
    expect(screen.getByRole('banner').textContent).toContain('客户摘要');
    expect(screen.getByRole('main').textContent).toContain('联系人内容');
  });

  it('exposes safe-area classes and a 44px action contract', () => {
    render(
      <MobileShell
        appName="当前应用"
        title="活动"
        actions={<button type="button">参加活动</button>}
      >
        <p>活动内容</p>
      </MobileShell>,
    );

    expect(screen.getByTestId('mobile-shell').classList).toContain('mobile-shell--safe-area');
    const dock = screen.getByTestId('mobile-action-dock');
    expect(dock.classList).toContain('mobile-action-dock--safe-area');
    expect(dock.style.minHeight).toBe('44px');
  });
});

describe('MobileState', () => {
  it.each([
    ['loading', '正在加载'],
    ['empty', '暂无数据'],
    ['error', '加载失败'],
    ['forbidden', '无权访问'],
    ['not-found', '未找到'],
  ] as const)('renders the %s variant', (kind, defaultTitle) => {
    render(<MobileState kind={kind} />);

    const role = kind === 'error' ? 'alert' : 'status';
    expect(screen.getByRole(role).textContent).toContain(defaultTitle);
  });

  it('renders a retry action for recoverable state', () => {
    const retry = vi.fn();
    render(
      <MobileState
        kind="error"
        title="网络不可用"
        description="请检查网络后重试。"
        actionLabel="重试"
        onAction={retry}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: '重试' }));
    expect(retry).toHaveBeenCalledOnce();
    expect(screen.getByRole('alert').textContent).toContain('请检查网络后重试。');
  });
});

describe('MobileActionDock', () => {
  it('renders actions in a named semantic region', () => {
    render(
      <MobileActionDock>
        <button type="button">保存</button>
      </MobileActionDock>,
    );

    expect(
      screen.getByRole('region', { name: '页面操作' }).contains(
        screen.getByRole('button', { name: '保存' }),
      ),
    ).toBe(true);
  });
});

describe('MobileErrorBoundary', () => {
  it('renders a supplied fallback after a child render error', () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    function BrokenContent(): never {
      throw new Error('render failed');
    }

    render(
      <MobileErrorBoundary fallback={<p role="alert">自定义回退</p>}>
        <BrokenContent />
      </MobileErrorBoundary>,
    );

    expect(screen.getByRole('alert').textContent).toContain('自定义回退');
  });

  it('renders the default error state when no fallback is supplied', () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    function BrokenContent(): never {
      throw new Error('render failed');
    }

    render(
      <MobileErrorBoundary>
        <BrokenContent />
      </MobileErrorBoundary>,
    );

    expect(screen.getByRole('alert').textContent).toContain('页面暂时不可用');
  });
});
