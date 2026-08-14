import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MobileApiError } from '@mochat/mobile-foundation';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  loadWorkFissionParticipant,
  loadWorkFissionProgress,
  safeRewardUrl,
} from './work-fission-api';
import { WorkFissionPage } from './work-fission-page';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function renderWorkFission(path: string, request: ReturnType<typeof vi.fn>) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <WorkFissionPage activityKind="workFission" request={request} />
    </MemoryRouter>,
  );
}

const rawParticipant = {
  openid: 'openid-session-1',
  unionid: 'union-session-1',
  nickname: '会话参与者',
  headimgurl: 'https://avatar.example/session-1.png',
};

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

describe('Operation work-fission parameters and participant session', () => {
  it.each([
    ['/workFission', '缺少有效的 fission_id'],
    ['/workFission?id=0', '缺少有效的 fission_id'],
    ['/workFission?id=1.5', '缺少有效的 fission_id'],
  ])('blocks requests for an invalid real activity id: %s', (path, message) => {
    const request = vi.fn();
    renderWorkFission(path, request);

    expect(screen.getByText('活动参数错误')).not.toBeNull();
    expect(screen.getByText(message)).not.toBeNull();
    expect(request).not.toHaveBeenCalled();
  });

  it('loads the participant from the Operation cookie session with the real activity id', async () => {
    const request = vi.fn().mockResolvedValue(rawParticipant);

    await expect(loadWorkFissionParticipant(request, 17)).resolves.toEqual(rawParticipant);
    expect(request).toHaveBeenCalledWith(
      '/openUserInfo/workFission?id=17',
      { method: 'GET' },
    );
  });

  it('maps the Go empty-list session response to no participant', async () => {
    const request = vi.fn().mockResolvedValue([]);

    await expect(loadWorkFissionParticipant(request, 17)).resolves.toBeNull();
  });

  it.each([
    {},
    [{ unionid: 'union-session-1' }],
    { openid: '', unionid: 'union-session-1', nickname: '参与者', headimgurl: '' },
    { openid: 'openid-session-1', unionid: '', nickname: '参与者', headimgurl: '' },
    { openid: 'openid-session-1', unionid: 'union-session-1', nickname: 7, headimgurl: '' },
  ])('rejects malformed participant session data: %o', async (payload) => {
    const request = vi.fn().mockResolvedValue(payload);

    await expect(loadWorkFissionParticipant(request, 17)).rejects.toMatchObject({
      kind: 'validation',
    });
  });

  it('rejects an invalid participant activity id without making a request', async () => {
    const request = vi.fn();

    await expect(loadWorkFissionParticipant(request, 0)).rejects.toMatchObject({
      kind: 'validation',
    });
    expect(request).not.toHaveBeenCalled();
  });
});

describe('Operation work-fission task progress', () => {
  it.each([
    ['https://gift.example/reward', 'https://gift.example/reward'],
    ['http://gift.example/reward', 'http://gift.example/reward'],
    ['/static/rewards/gift.png', '/static/rewards/gift.png'],
    ['  /static/rewards/gift.png  ', '/static/rewards/gift.png'],
  ])('allows a controlled reward URL: %s', (raw, expected) => {
    expect(safeRewardUrl(raw)).toBe(expected);
  });

  it.each([
    '',
    '//evil.example/reward',
    'javascript:alert(1)',
    'JaVaScRiPt:alert(1)',
    'data:text/html,<script>alert(1)</script>',
    'custom://reward',
    '\\evil.example\\reward',
    '/static\\reward.png',
    'https://[invalid',
  ])('rejects an unsafe reward URL: %s', (raw) => {
    expect(safeRewardUrl(raw)).toBeNull();
  });

  it('loads participant first and uses only the session unionid for taskData', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce(rawParticipant)
      .mockResolvedValueOnce(rawProgress);

    renderWorkFission('/workFission?id=9&union_id=attacker-controlled', request);

    expect(await screen.findByText('已邀请 2 位好友')).not.toBeNull();
    expect(screen.getByRole('region', { name: '任务宝活动概览' })).not.toBeNull();
    expect(screen.getByText('距下一任务还差 1 位')).not.toBeNull();
    expect(screen.getByText('任务 1：邀请 3 位好友')).not.toBeNull();
    expect(screen.getByText('任务 2：邀请 5 位好友')).not.toBeNull();
    expect(screen.getByText(/二维码奖励/)).not.toBeNull();
    expect(screen.getByText(/链接奖励/)).not.toBeNull();
    expect(screen.getByRole('link', { name: '查看奖励' }).getAttribute('href')).toBe(
      'https://gift.example/qr.png',
    );
    expect(screen.getAllByRole('listitem')).toHaveLength(2);
    expect(screen.queryByRole('navigation', { name: '员工工作台' })).toBeNull();
    expect(request).toHaveBeenNthCalledWith(
      1,
      '/openUserInfo/workFission?id=9',
      { method: 'GET' },
    );
    expect(request).toHaveBeenNthCalledWith(
      2,
      '/workFission/taskData?union_id=union-session-1&fission_id=9',
      { method: 'GET' },
    );
  });

  it('accepts fission_id as a compatibility alias but still ignores URL union_id', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce(rawParticipant)
      .mockResolvedValueOnce(rawProgress);

    renderWorkFission('/workFission?fission_id=9&union_id=attacker-controlled', request);

    expect(await screen.findByText('已邀请 2 位好友')).not.toBeNull();
    expect(request).toHaveBeenNthCalledWith(
      2,
      '/workFission/taskData?union_id=union-session-1&fission_id=9',
      { method: 'GET' },
    );
  });

  it('maps count/status/receive_status/gift_type/gift_url without inventing rewards', async () => {
    const request = vi.fn().mockResolvedValue(rawProgress);

    await expect(loadWorkFissionProgress(request, {
      unionId: 'union-session-1',
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

  it.each([0, -1, 1.5, '2130000000'])('rejects an invalid end_time contract value: %o', async (endTime) => {
    const request = vi.fn().mockResolvedValue({ ...rawProgress, end_time: endTime });

    await expect(loadWorkFissionProgress(request, {
      unionId: 'union-session-1',
      fissionId: 9,
    })).rejects.toMatchObject({ kind: 'validation' });
  });

  it.each([
    [{ unionId: '', fissionId: 9 }, '缺少 union_id'],
    [{ unionId: 'union-session-1', fissionId: 0 }, 'fission_id 必须为正整数'],
    [{ unionId: 'union-session-1', fissionId: 1.5 }, 'fission_id 必须为正整数'],
  ])('blocks invalid parameters at the feature API boundary: %o', async (params, message) => {
    const request = vi.fn();

    await expect(loadWorkFissionProgress(request, params)).rejects.toThrow(message);
    expect(request).not.toHaveBeenCalled();
  });

  it('renders an honest empty task state', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce(rawParticipant)
      .mockResolvedValueOnce({ invite_count: 0, differ_count: 0, end_time: 2_130_000_000, task: [] });

    renderWorkFission('/workFission?id=9', request);

    expect(await screen.findByText('暂无任务')).not.toBeNull();
    expect(screen.queryByText(/奖励已准备/)).toBeNull();
  });

  it('offers OAuth for raw [] and never requests taskData from a URL union_id', async () => {
    const request = vi.fn().mockResolvedValue([]);

    renderWorkFission('/workFission?id=9&union_id=attacker-controlled#tasks', request);

    const link = await screen.findByRole('link', { name: '重新授权' });
    expect(link.getAttribute('href')).toBe(
      '/auth/workFission?id=9&target=%2FworkFission%3Fid%3D9%26union_id%3Dattacker-controlled%23tasks',
    );
    expect(request).toHaveBeenCalledTimes(1);
    expect(request).toHaveBeenCalledWith('/openUserInfo/workFission?id=9', { method: 'GET' });
  });

  it('keeps participant 401 on the activity OAuth path', async () => {
    const request = vi.fn().mockRejectedValue(
      new MobileApiError('unauthorized', '活动授权已失效', { status: 401 }),
    );

    renderWorkFission('/workFission?id=9#tasks', request);

    const link = await screen.findByRole('link', { name: '重新授权' });
    expect(link.getAttribute('href')).toBe(
      '/auth/workFission?id=9&target=%2FworkFission%3Fid%3D9%23tasks',
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
      unionId: 'union-session-1',
      fissionId: 9,
    })).rejects.toMatchObject({ kind: 'validation' });
  });

  it('rejects an unknown reward type instead of naming an invented reward', async () => {
    const request = vi.fn().mockResolvedValue({
      ...rawProgress,
      task: [{ ...rawProgress.task[0], gift_type: 2 }],
    });

    await expect(loadWorkFissionProgress(request, {
      unionId: 'union-session-1',
      fissionId: 9,
    })).rejects.toMatchObject({ kind: 'validation' });
  });

  it('renders end_time=0 as a non-retryable validation failure without inventing progress', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce(rawParticipant)
      .mockResolvedValueOnce({ ...rawProgress, end_time: 0 });

    renderWorkFission('/workFission?id=9', request);

    expect(await screen.findByText('任务进度加载失败')).not.toBeNull();
    expect(screen.getByText('任务进度响应格式无效。')).not.toBeNull();
    expect(screen.queryByText(/已邀请/)).toBeNull();
    expect(screen.queryByText('任务进行中')).toBeNull();
    expect(screen.queryByRole('button', { name: '重试' })).toBeNull();
  });

  it.each([
    ['forbidden', new MobileApiError('forbidden', '没有活动查看权限', { status: 403 }), 'forbidden', false],
    ['not-found', new MobileApiError('not-found', '任务宝活动不存在', { status: 404 }), 'not-found', false],
    ['validation', new MobileApiError('validation', '任务参数无效', { status: 400 }), 'error', false],
    ['conflict', new MobileApiError('conflict', '活动状态冲突', { status: 409 }), 'error', false],
    ['network', new MobileApiError('network', '活动网络请求失败'), 'error', true],
    ['server', new MobileApiError('server', '活动服务暂时不可用', { status: 503 }), 'error', true],
    ['explicit retryable', new MobileApiError('aborted', '活动请求超时', { retryable: true }), 'error', true],
    ['unknown', new Error('未知活动错误'), 'error', false],
  ] as const)(
    'maps %s to MobileState %s with retry=%s',
    async (_label, error, expectedKind, retryable) => {
      const request = vi.fn()
        .mockResolvedValueOnce(rawParticipant)
        .mockRejectedValueOnce(error);

      renderWorkFission('/workFission?id=9', request);

      expect(await screen.findByText(error.message)).not.toBeNull();
      expect(document.querySelector(`.mobile-state--${expectedKind}`)).not.toBeNull();
      if (retryable) {
        expect(screen.getByRole('button', { name: '重试' })).not.toBeNull();
      } else {
        expect(screen.queryByRole('button', { name: '重试' })).toBeNull();
      }
    },
  );

  it.each([
    '数据不存在',
    '活动不存在',
    '任务宝活动已失效',
    '活动已经结束',
  ])('maps a validation activity-missing message to not-found without retry: %s', async (message) => {
    const request = vi.fn()
      .mockResolvedValueOnce(rawParticipant)
      .mockRejectedValueOnce(new MobileApiError('validation', message, { status: 400 }));

    renderWorkFission('/workFission?id=9', request);

    expect(await screen.findByText(message)).not.toBeNull();
    expect(document.querySelector('.mobile-state--not-found')).not.toBeNull();
    expect(screen.getByText('活动已失效')).not.toBeNull();
    expect(screen.queryByRole('button', { name: '重试' })).toBeNull();
  });

  it.each([
    2_000_000_000,
    1_999_999_999,
  ])('renders an ended activity without active tasks or retry when end_time=%s is not future', async (endTime) => {
    vi.spyOn(Date, 'now').mockReturnValue(2_000_000_000_000);
    const request = vi.fn()
      .mockResolvedValueOnce(rawParticipant)
      .mockResolvedValueOnce({ ...rawProgress, end_time: endTime });

    renderWorkFission('/workFission?id=9', request);

    expect(await screen.findByText('活动已结束')).not.toBeNull();
    expect(document.querySelector('.mobile-state--not-found')).not.toBeNull();
    expect(screen.queryByText('任务进行中')).toBeNull();
    expect(screen.queryByRole('button', { name: '重试' })).toBeNull();
  });

  it('renders progress when end_time is a valid future Unix timestamp', async () => {
    vi.spyOn(Date, 'now').mockReturnValue(2_000_000_000_000);
    const request = vi.fn()
      .mockResolvedValueOnce(rawParticipant)
      .mockResolvedValueOnce({ ...rawProgress, end_time: 2_000_000_001 });

    renderWorkFission('/workFission?id=9', request);

    expect(await screen.findByText('任务进行中')).not.toBeNull();
    expect(screen.queryByText('活动已结束')).toBeNull();
  });

  it('retries a retryable progress failure with the same session participant', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce(rawParticipant)
      .mockRejectedValueOnce(new MobileApiError('network', '网络请求失败', { retryable: true }))
      .mockResolvedValueOnce(rawParticipant)
      .mockResolvedValueOnce(rawProgress);

    renderWorkFission('/workFission?id=9', request);

    fireEvent.click(await screen.findByRole('button', { name: '重试' }));
    expect(await screen.findByText('已邀请 2 位好友')).not.toBeNull();
    await waitFor(() => expect(request).toHaveBeenCalledTimes(4));
  });
});
