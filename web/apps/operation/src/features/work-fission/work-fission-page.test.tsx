import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MobileApiError } from '@mochat/mobile-foundation';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { loadWorkFissionProgress } from './work-fission-api';
import { WorkFissionPage } from './work-fission-page';

afterEach(cleanup);

function renderWorkFission(
  path: string,
  request: ReturnType<typeof vi.fn>,
) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <WorkFissionPage request={request} />
    </MemoryRouter>,
  );
}

const rawProgress = {
  invite_count: 2,
  differ_count: 1,
  end_time: 2_130_000_000,
  task: [
    {
      count: 3,
      status: 0,
      receive_status: 0,
      gift_type: 0,
      gift_url: 'https://gift.example/qr.png',
    },
    {
      count: 5,
      status: 1,
      receive_status: 1,
      gift_type: 1,
      gift_url: '',
    },
  ],
};

describe('Operation work-fission parameters', () => {
  it.each([
    ['/workFission?fission_id=9', '缺少 union_id'],
    ['/workFission?union_id=union-1', '缺少有效的 fission_id'],
    ['/workFission?union_id=union-1&fission_id=0', '缺少有效的 fission_id'],
    ['/workFission?union_id=union-1&fission_id=1.5', '缺少有效的 fission_id'],
  ])('blocks requests for invalid activity parameters: %s', (path, message) => {
    const request = vi.fn();
    renderWorkFission(path, request);

    expect(screen.getByText('活动参数错误')).not.toBeNull();
    expect(screen.getByText(message)).not.toBeNull();
    expect(request).not.toHaveBeenCalled();
  });
});

describe('Operation work-fission task progress', () => {
  it('requests the real taskData path and adapts the raw Go task fields', async () => {
    const request = vi.fn().mockResolvedValue(rawProgress);

    renderWorkFission('/workFission?union_id=union-1&fission_id=9', request);

    expect(await screen.findByText('已邀请 2 位好友')).not.toBeNull();
    expect(screen.getByText('距下一任务还差 1 位')).not.toBeNull();
    expect(screen.getByText('任务 1：邀请 3 位好友')).not.toBeNull();
    expect(screen.getByText('任务 2：邀请 5 位好友')).not.toBeNull();
    expect(screen.getByText(/二维码奖励/)).not.toBeNull();
    expect(screen.getByText(/链接奖励/)).not.toBeNull();
    expect(screen.getByRole('link', { name: '查看奖励' }).getAttribute('href')).toBe(
      'https://gift.example/qr.png',
    );
    expect(request).toHaveBeenCalledWith(
      '/workFission/taskData?union_id=union-1&fission_id=9',
      { method: 'GET' },
    );
  });

  it('maps count/status/receive_status/gift_type/gift_url without inventing rewards', async () => {
    const request = vi.fn().mockResolvedValue(rawProgress);

    await expect(loadWorkFissionProgress(request, {
      unionId: 'union-1',
      fissionId: 9,
    })).resolves.toEqual({
      inviteCount: 2,
      differCount: 1,
      endTime: 2_130_000_000,
      tasks: [
        {
          level: 1,
          target: 3,
          completed: false,
          received: false,
          reward: { type: 0, url: 'https://gift.example/qr.png' },
        },
        {
          level: 2,
          target: 5,
          completed: true,
          received: true,
          reward: { type: 1, url: null },
        },
      ],
    });
  });

  it.each([
    [{ unionId: '', fissionId: 9 }, '缺少 union_id'],
    [{ unionId: 'union-1', fissionId: 0 }, 'fission_id 必须为正整数'],
    [{ unionId: 'union-1', fissionId: 1.5 }, 'fission_id 必须为正整数'],
  ])('blocks invalid parameters at the feature API boundary: %o', async (params, message) => {
    const request = vi.fn();

    await expect(loadWorkFissionProgress(request, params)).rejects.toThrow(message);
    expect(request).not.toHaveBeenCalled();
  });

  it('renders an honest empty task state', async () => {
    const request = vi.fn().mockResolvedValue({
      invite_count: 0,
      differ_count: 0,
      end_time: 0,
      task: [],
    });

    renderWorkFission('/workFission?union_id=union-1&fission_id=9', request);

    expect(await screen.findByText('暂无任务')).not.toBeNull();
    expect(screen.queryByText(/奖励已准备/)).toBeNull();
  });

  it('offers the current activity OAuth on 401 and preserves query and hash', async () => {
    const request = vi.fn().mockRejectedValue(
      new MobileApiError('unauthorized', '活动授权已失效', { status: 401 }),
    );

    renderWorkFission('/workFission?union_id=union-1&fission_id=9#tasks', request);

    const link = await screen.findByRole('link', { name: '重新授权' });
    expect(link.getAttribute('href')).toBe(
      '/auth/workFission?id=9&target=%2FworkFission%3Funion_id%3Dunion-1%26fission_id%3D9%23tasks',
    );
  });

  it('rejects malformed successful task data as an invalid response', async () => {
    const request = vi.fn().mockResolvedValue({
      invite_count: 2,
      differ_count: 1,
      end_time: 2_130_000_000,
      task: [{ count: 3, status: 0 }],
    });

    await expect(loadWorkFissionProgress(request, {
      unionId: 'union-1',
      fissionId: 9,
    })).rejects.toMatchObject({ kind: 'validation' });
  });

  it('rejects an unknown reward type instead of naming an invented reward', async () => {
    const request = vi.fn().mockResolvedValue({
      ...rawProgress,
      task: [{ ...rawProgress.task[0], gift_type: 2 }],
    });

    await expect(loadWorkFissionProgress(request, {
      unionId: 'union-1',
      fissionId: 9,
    })).rejects.toMatchObject({ kind: 'validation' });
  });

  it('renders an invalid envelope error without inventing progress', async () => {
    const request = vi.fn().mockRejectedValue(
      new MobileApiError('validation', 'Invalid API response.', { status: 200 }),
    );

    renderWorkFission('/workFission?union_id=union-1&fission_id=9', request);

    expect(await screen.findByText('任务进度加载失败')).not.toBeNull();
    expect(screen.getByText('Invalid API response.')).not.toBeNull();
    expect(screen.queryByText(/已邀请/)).toBeNull();
  });

  it('shows an expired activity error and retries the real request', async () => {
    const request = vi.fn()
      .mockRejectedValueOnce(new MobileApiError('validation', '活动不存在', { status: 400 }))
      .mockResolvedValueOnce(rawProgress);

    renderWorkFission('/workFission?union_id=union-1&fission_id=9', request);

    expect(await screen.findByText('活动已失效')).not.toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '重试' }));
    expect(await screen.findByText('已邀请 2 位好友')).not.toBeNull();
    await waitFor(() => expect(request).toHaveBeenCalledTimes(2));
  });
});
