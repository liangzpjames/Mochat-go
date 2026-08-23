import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { useRef, useState } from 'react';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import { ConfirmAction } from './confirm-action';
import { DashboardDataState } from './dashboard-data-state';
import { DashboardDialog } from './dashboard-dialog';
import { DashboardFilterPanel } from './dashboard-filter-panel';
import { DashboardPageShell } from './dashboard-page-shell';
import { DateRangeFields } from './date-range-fields';
import { RowAction } from './row-action';

describe('dashboard interaction primitives', () => {
  beforeAll(() => {
    globalThis.ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  });

  afterEach(cleanup);

  it('does not run a destructive action before confirmation', async () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmAction title="确认停用？" onConfirm={onConfirm}>
        <button type="button">停用</button>
      </ConfirmAction>,
    );

    fireEvent.click(screen.getByRole('button', { name: '停用' }));
    expect(onConfirm).not.toHaveBeenCalled();

    const confirm = await screen.findByRole('button', { name: '确认' });
    expect(confirm.className).toContain('ant-btn-dangerous');
    fireEvent.click(confirm);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('allows non-destructive confirmations to use the primary style', async () => {
    render(
      <ConfirmAction danger={false} title="确认启用？" onConfirm={() => undefined}>
        <button type="button">启用</button>
      </ConfirmAction>,
    );

    fireEvent.click(screen.getByRole('button', { name: '启用' }));
    expect((await screen.findByRole('button', { name: '确认' })).className).not.toContain('ant-btn-dangerous');
  });

  it('rejects an inverted date range without submitting', () => {
    const onSubmit = vi.fn();
    render(
      <DateRangeFields
        value={{ startDate: '2026-09-01', endDate: '2026-08-01' }}
        onChange={() => undefined}
        onValidSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    expect(screen.getByRole('alert').textContent).toContain('开始日期不能晚于结束日期');
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('renders one page heading and one primary page action', () => {
    render(
      <DashboardPageShell
        title="客户标签"
        description="管理企业客户标签"
        primaryAction={<button type="button">新建标签</button>}
      >
        <p>页面内容</p>
      </DashboardPageShell>,
    );

    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
    expect(screen.getByRole('button', { name: '新建标签' })).toBeTruthy();
  });

  it('keeps query and reset actions in a labelled filter panel', () => {
    const onSubmit = vi.fn();
    const onReset = vi.fn();
    render(
      <DashboardFilterPanel onSubmit={onSubmit} onReset={onReset}>
        <label>姓名<input /></label>
      </DashboardFilterPanel>,
    );

    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    fireEvent.click(screen.getByRole('button', { name: '重置' }));
    expect(onSubmit).toHaveBeenCalledOnce();
    expect(onReset).toHaveBeenCalledOnce();
    expect(screen.getByRole('search').getAttribute('aria-label')).toBe('业务筛选');
  });

  it('distinguishes provider unavailability from empty and request errors', () => {
    const { rerender } = render(
      <DashboardDataState
        state="provider-unavailable"
        title="会话归档未开通"
        description="开通后可查看员工会话。"
        action={<a href="/company-setting/website">去配置会话归档</a>}
      />,
    );

    expect(screen.getByRole('status').textContent).toContain('会话归档未开通');
    expect(screen.getByRole('link', { name: '去配置会话归档' }).getAttribute('href')).toBe('/company-setting/website');

    rerender(<DashboardDataState state="empty" />);
    expect(screen.getByRole('status').textContent).toContain('暂无数据');

    rerender(<DashboardDataState state="error" />);
    expect(screen.getByRole('alert').textContent).toContain('加载失败');
  });

  it('closes a dashboard dialog and restores focus to its trigger', async () => {
    const getComputedStyle = window.getComputedStyle.bind(window);
    vi.spyOn(window, 'getComputedStyle').mockImplementation((element) => getComputedStyle(element));

    function DialogHarness() {
      const [open, setOpen] = useState(true);
      const triggerRef = useRef<HTMLButtonElement>(null);
      return (
        <>
          <button ref={triggerRef} id="dialog-trigger" type="button">编辑客户</button>
          <DashboardDialog open={open} title="编辑客户" triggerRef={triggerRef} onCancel={() => setOpen(false)}>
            <label>客户名称<input /></label>
          </DashboardDialog>
        </>
      );
    }

    render(<DialogHarness />);
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('textbox', { name: '客户名称' })));
    fireEvent.click(screen.getByRole('button', { name: '取消' }));
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('button', { name: '编辑客户' })));
  });

  it('keeps focus inside a dialog when cancel requests do not close it', async () => {
    const getComputedStyle = window.getComputedStyle.bind(window);
    vi.spyOn(window, 'getComputedStyle').mockImplementation((element) => getComputedStyle(element));
    const onCancel = vi.fn();

    function LockedDialogHarness() {
      const triggerRef = useRef<HTMLButtonElement>(null);
      return (
        <>
          <button ref={triggerRef} type="button">打开保存中弹窗</button>
          <DashboardDialog open title="保存中" confirmLoading triggerRef={triggerRef} onCancel={onCancel}>
            <label>配置名称<input /></label>
          </DashboardDialog>
        </>
      );
    }

    render(<LockedDialogHarness />);
    const dialog = await screen.findByRole('dialog', { name: '保存中' });
    const input = screen.getByRole('textbox', { name: '配置名称' });
    await waitFor(() => expect(document.activeElement).toBe(input));

    fireEvent.keyDown(document, { key: 'Escape', code: 'Escape' });
    await waitFor(() => expect(onCancel).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true));

    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    await waitFor(() => expect(onCancel).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true));

    const wrap = dialog.closest('.ant-modal-wrap');
    expect(wrap).not.toBeNull();
    if (wrap) {
      fireEvent.mouseDown(wrap);
      fireEvent.mouseUp(wrap);
      fireEvent.click(wrap);
    }
    await waitFor(() => expect(onCancel).toHaveBeenCalledTimes(3));
    await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true));
  });

  it('includes the target name in row action accessible text', () => {
    render(<RowAction action="停用" target="超级管理员" onClick={() => undefined} />);
    expect(screen.getByRole('button', { name: '停用 超级管理员' })).toBeTruthy();
  });
});
