import {
  MobileApiError,
  MobileShell,
  MobileState,
  type MobileStateKind,
} from '@mochat/mobile-foundation';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';

import {
  loadContactSummary,
  type ContactRequest,
  type ContactSummary,
} from './contact-api';

type ContactPageProps = {
  request: ContactRequest;
  onReauthenticate: () => void;
};

type ContactFailure = {
  stateKind: Extract<MobileStateKind, 'error' | 'forbidden' | 'not-found'>;
  title: string;
  message: string;
  retryable: boolean;
};

type ContactPageState =
  | { kind: 'loading' }
  | { kind: 'success'; contact: ContactSummary }
  | { kind: 'unauthorized' }
  | ({ kind: 'failure' } & ContactFailure);

function contactFailure(error: unknown): ContactFailure {
  const message = error instanceof Error ? error.message : '客户资料加载失败。';
  if (error instanceof MobileApiError) {
    if (error.kind === 'forbidden') {
      return { stateKind: 'forbidden', title: '无权访问客户资料', message, retryable: false };
    }
    if (error.kind === 'not-found') {
      return { stateKind: 'not-found', title: '客户不存在', message, retryable: false };
    }
    if (error.kind === 'validation' || error.kind === 'conflict') {
      return { stateKind: 'error', title: '客户资料加载失败', message, retryable: false };
    }
    if (error.kind === 'network' || error.kind === 'server' || error.retryable) {
      return { stateKind: 'error', title: '客户资料加载失败', message, retryable: true };
    }
  }
  return { stateKind: 'error', title: '客户资料加载失败', message, retryable: false };
}

export function ContactPage({ request, onReauthenticate }: ContactPageProps) {
  const [params] = useSearchParams();
  const externalUserId = params.get('wxExternalUserid')?.trim() ?? '';
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<ContactPageState>({ kind: 'loading' });

  useEffect(() => {
    if (externalUserId.length === 0) return undefined;
    let active = true;
    setState({ kind: 'loading' });
    void loadContactSummary(request, externalUserId)
      .then((contact) => {
        if (active) setState({ kind: 'success', contact });
      })
      .catch((error: unknown) => {
        if (!active) return;
        if (error instanceof MobileApiError && error.kind === 'unauthorized') {
          setState({ kind: 'unauthorized' });
          onReauthenticate();
          return;
        }
        setState({ kind: 'failure', ...contactFailure(error) });
      });
    return () => {
      active = false;
    };
  }, [attempt, externalUserId, onReauthenticate, request]);

  if (externalUserId.length === 0) {
    return (
      <MobileShell appName="MoChat 客户侧边栏" title="客户资料">
        <MobileState
          kind="error"
          title="客户参数错误"
          description="缺少 wxExternalUserid，无法识别当前客户。"
        />
      </MobileShell>
    );
  }

  if (state.kind === 'loading') {
    return (
      <MobileShell appName="MoChat 客户侧边栏" title="客户资料">
        <MobileState kind="loading" description="正在读取当前客户资料。" />
      </MobileShell>
    );
  }

  if (state.kind === 'unauthorized') {
    return (
      <MobileShell appName="MoChat 客户侧边栏" title="客户资料">
        <MobileState
          kind="error"
          title="登录状态已失效"
          description="正在重新进入企业微信员工授权。"
        />
      </MobileShell>
    );
  }

  if (state.kind === 'failure') {
    return (
      <MobileShell appName="MoChat 客户侧边栏" title="客户资料">
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

  return (
    <MobileShell
      appName="MoChat 客户侧边栏"
      title="客户资料"
      subtitle="当前会话客户摘要"
    >
      <article className="contact-summary">
        {state.contact.avatar === null ? (
          <div aria-label="客户暂无头像" className="contact-summary__avatar-placeholder" />
        ) : (
          <img
            alt={`${state.contact.name}的头像`}
            className="contact-summary__avatar"
            src={state.contact.avatar}
          />
        )}
        <div>
          <h2>{state.contact.name}</h2>
          <p>客户编号：{state.contact.id}</p>
        </div>
      </article>
    </MobileShell>
  );
}
