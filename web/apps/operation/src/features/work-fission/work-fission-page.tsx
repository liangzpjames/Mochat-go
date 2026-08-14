import {
  MobileApiError,
  MobileCard,
  MobileShell,
  MobileState,
  type MobileStateKind,
} from '@mochat/mobile-foundation';
import { useEffect, useState } from 'react';
import { useLocation, useSearchParams } from 'react-router';

import {
  operationAuthHref,
  type OperationActivityKind,
} from '../../auth/operation-session';
import {
  loadWorkFissionParticipant,
  loadWorkFissionProgress,
  type WorkFissionProgress,
  type WorkFissionRequest,
  type WorkFissionReward,
} from './work-fission-api';

type WorkFissionPageProps = {
  activityKind: OperationActivityKind;
  request: WorkFissionRequest;
};

type WorkFissionPageState =
  | { kind: 'loading' }
  | { kind: 'success'; progress: WorkFissionProgress }
  | { kind: 'unauthorized'; href: string }
  | {
    kind: 'failure';
    stateKind: Extract<MobileStateKind, 'error' | 'forbidden' | 'not-found'>;
    title: string;
    message: string;
    retryable: boolean;
  };

type WorkFissionFailure = Extract<WorkFissionPageState, { kind: 'failure' }>;

function workFissionFailure(error: unknown): WorkFissionFailure {
  const message = error instanceof Error ? error.message : '任务进度加载失败。';
  const activityMissing = message.trim() === '数据不存在'
    || /活动.*(?:不存在|失效|结束)/.test(message);
  if (error instanceof MobileApiError) {
    if (error.kind === 'forbidden') {
      return {
        kind: 'failure',
        stateKind: 'forbidden',
        title: '无权访问当前活动',
        message,
        retryable: false,
      };
    }
    if (error.kind === 'not-found' || (error.kind === 'validation' && activityMissing)) {
      return {
        kind: 'failure',
        stateKind: 'not-found',
        title: activityMissing ? '活动已失效' : '活动不存在',
        message,
        retryable: false,
      };
    }
    if (error.kind === 'validation' || error.kind === 'conflict') {
      return {
        kind: 'failure',
        stateKind: 'error',
        title: '任务进度加载失败',
        message,
        retryable: false,
      };
    }
    if (error.kind === 'network' || error.kind === 'server' || error.retryable) {
      return {
        kind: 'failure',
        stateKind: 'error',
        title: '任务进度加载失败',
        message,
        retryable: true,
      };
    }
  }
  return {
    kind: 'failure',
    stateKind: 'error',
    title: '任务进度加载失败',
    message,
    retryable: false,
  };
}

function rewardTypeLabel(reward: WorkFissionReward): string {
  if (reward.type === 0) return '二维码奖励';
  return '链接奖励';
}

function WorkFissionHero() {
  return (
    <div className="operation-hero" aria-hidden="true">
      <div className="operation-hero__copy">
        <strong>邀请好友，逐步达成任务</strong>
        <span>实时查看参与进度与奖励状态</span>
      </div>
      <svg className="operation-hero__graphic" viewBox="0 0 132 96">
        <defs>
          <linearGradient id="operation-hero-card" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor="#ffffff" />
            <stop offset="1" stopColor="#cfe4ff" />
          </linearGradient>
        </defs>
        <circle cx="96" cy="20" r="12" fill="#75e0d1" opacity=".75" />
        <rect x="24" y="20" width="76" height="58" rx="12" fill="url(#operation-hero-card)" />
        <rect x="38" y="34" width="48" height="7" rx="3.5" fill="#3d86f7" opacity=".72" />
        <rect x="38" y="49" width="34" height="6" rx="3" fill="#8dbaff" />
        <path d="M83 64l9 9 18-24" fill="none" stroke="#0bb59b" strokeLinecap="round" strokeLinejoin="round" strokeWidth="7" />
      </svg>
    </div>
  );
}

export function WorkFissionPage({ activityKind, request }: WorkFissionPageProps) {
  const location = useLocation();
  const [params] = useSearchParams();
  const rawFissionId = (params.get('id') ?? params.get('fission_id') ?? '').trim();
  const fissionId = Number(rawFissionId);
  const validFissionId = rawFissionId.length > 0
    && Number.isInteger(fissionId)
    && fissionId > 0;
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<WorkFissionPageState>({ kind: 'loading' });

  useEffect(() => {
    if (!validFissionId) return undefined;
    let active = true;
    const target = `${location.pathname}${location.search}${location.hash}`;
    const authorize = () => {
      if (active) {
        setState({
          kind: 'unauthorized',
          href: operationAuthHref(activityKind, target, { fissionId }),
        });
      }
    };

    setState({ kind: 'loading' });
    void loadWorkFissionParticipant(request, fissionId)
      .then(async (participant) => {
        if (!active) return;
        if (participant === null) {
          authorize();
          return;
        }
        const progress = await loadWorkFissionProgress(request, {
          unionId: participant.unionid,
          fissionId,
        });
        if (!active) return;
        if (progress.endTime <= Math.floor(Date.now() / 1000)) {
          setState({
            kind: 'failure',
            stateKind: 'not-found',
            title: '活动已结束',
            message: '当前任务宝活动已经结束。',
            retryable: false,
          });
          return;
        }
        setState({ kind: 'success', progress });
      })
      .catch((error: unknown) => {
        if (!active) return;
        if (error instanceof MobileApiError && error.kind === 'unauthorized') {
          authorize();
          return;
        }
        setState(workFissionFailure(error));
      });
    return () => {
      active = false;
    };
  }, [activityKind, attempt, fissionId, location.hash, location.pathname, location.search, request, validFissionId]);

  if (!validFissionId) {
    return (
      <MobileShell appName="MoChat 营销活动" title="任务宝活动">
        <MobileState
          kind="error"
          title="活动参数错误"
          description="缺少有效的 fission_id"
        />
      </MobileShell>
    );
  }

  if (state.kind === 'loading') {
    return (
      <MobileShell appName="MoChat 营销活动" title="任务宝活动">
        <MobileState kind="loading" description="正在读取活动参与者与任务进度。" />
      </MobileShell>
    );
  }

  if (state.kind === 'unauthorized') {
    return (
      <MobileShell appName="MoChat 营销活动" title="任务宝活动">
        <MobileState
          kind="error"
          title="活动授权已失效"
          description="请完成当前任务宝活动授权。"
        />
        <a className="operation-primary-link" href={state.href}>重新授权</a>
      </MobileShell>
    );
  }

  if (state.kind === 'failure') {
    return (
      <MobileShell appName="MoChat 营销活动" title="任务宝活动">
        <MobileState
          kind={state.stateKind}
          title={state.title}
          description={state.message}
          {...(state.retryable ? {
            actionLabel: '重试',
            onAction: () => setAttempt((value) => value + 1),
          } : {})}
        />
      </MobileShell>
    );
  }

  const { progress } = state;
  return (
    <MobileShell
      appName="MoChat 营销活动"
      title="任务宝活动"
      eyebrow="好友助力"
      subtitle="当前参与者任务进度"
      hero={<WorkFissionHero />}
    >
      <section className="work-fission-overview" aria-label="任务宝活动概览">
        <MobileCard tone="accent" padding="comfortable">
          <div className="work-fission-summary">
            <span className="work-fission-summary__label">参与进度</span>
            <strong>已邀请 {progress.inviteCount} 位好友</strong>
            <span>距下一任务还差 {progress.differCount} 位</span>
          </div>
        </MobileCard>
      </section>
      {progress.tasks.length === 0 ? (
        <MobileState
          kind="empty"
          title="暂无任务"
          description="当前活动尚未配置任务。"
        />
      ) : (
        <ol className="work-fission-tasks">
          {progress.tasks.map((task) => (
            <li key={task.level}>
              <MobileCard tone="surface" padding="comfortable">
                <div className="work-fission-task__heading">
                  <span className="work-fission-task__level">第 {task.level} 阶段</span>
                  <span className={`work-fission-task__status${task.completed ? ' is-complete' : ''}`}>
                    {task.completed ? '任务已完成' : '任务进行中'}
                  </span>
                </div>
                <h2>任务 {task.level}：邀请 {task.target} 位好友</h2>
                <div className="work-fission-task__meta">
                  <p>{rewardTypeLabel(task.reward)}</p>
                  <p>{task.received ? '奖励已领取' : '奖励未领取'}</p>
                </div>
                {task.reward.url === null ? null : (
                  <a href={task.reward.url}>查看奖励</a>
                )}
              </MobileCard>
            </li>
          ))}
        </ol>
      )}
    </MobileShell>
  );
}
