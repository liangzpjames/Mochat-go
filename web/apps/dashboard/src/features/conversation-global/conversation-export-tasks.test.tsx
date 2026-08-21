import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { ConversationExportTask, ConversationExportTaskPage } from './conversation-global-api';
import { ConversationExportTasks } from './conversation-export-tasks';

const completedTask: ConversationExportTask = {
  id: 3,
  exportType: 'employee',
  objectCount: 2,
  startAt: '2026-08-15T00:00:00+08:00',
  endAt: '2026-08-21T10:00:00+08:00',
  fileMode: 'split',
  format: 'zip',
  status: 'completed',
  estimatedMessageCount: 8,
  messageCount: 8,
  fileCount: 2,
  artifactName: 'conversation-export-3.zip',
  artifactSize: 4096,
  expiresAt: '2026-08-28T10:00:00+08:00',
  createdAt: '2026-08-21T09:55:00+08:00',
  finishedAt: '2026-08-21T09:55:03+08:00',
};

const page: ConversationExportTaskPage = { items: [completedTask], total: 1, page: 1, pageSize: 20 };

afterEach(cleanup);

describe('ConversationExportTasks', () => {
  it('renders a compact real-data task summary with a short download action', () => {
    const onDownload = vi.fn();
    render(<ConversationExportTasks data={page} loading={false} refreshing={false} onClose={vi.fn()} onPageChange={vi.fn()} onDownload={onDownload} />);

    expect(screen.getByText('员工导出')).toBeTruthy();
    expect(screen.getByText('#3')).toBeTruthy();
    expect(screen.getByText('2 个对象')).toBeTruthy();
    expect(screen.getByText('8 条消息')).toBeTruthy();
    expect(screen.getByText('2 个文件')).toBeTruthy();
    expect(screen.getByText('创建于 2026-08-21 09:55')).toBeTruthy();
    const download = screen.getByRole('button', { name: '下载 conversation-export-3.zip' });
    expect(download.textContent).toBe('下载文件');
    fireEvent.click(download);
    expect(onDownload).toHaveBeenCalledWith(completedTask);
  });

  it('keeps existing tasks visible during a silent background refresh', () => {
    render(<ConversationExportTasks data={page} loading={false} refreshing onClose={vi.fn()} onPageChange={vi.fn()} onDownload={vi.fn()} />);

    expect(screen.getByText('员工导出')).toBeTruthy();
    expect(screen.getByRole('status').textContent).toContain('正在同步任务状态');
    expect(screen.queryByText('正在加载任务…')).toBeNull();
  });
});
