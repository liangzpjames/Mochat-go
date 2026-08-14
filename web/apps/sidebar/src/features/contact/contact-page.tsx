import {
  MobileApiError,
  MobileCard,
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
      <MobileCard padding="comfortable" tone="surface">
        <MobileState
          kind="error"
          title="客户参数错误"
          description="缺少 wxExternalUserid，无法识别当前客户。"
        />
      </MobileCard>
    );
  }

  if (state.kind === 'loading') {
    return (
      <MobileCard padding="comfortable" tone="surface">
        <MobileState kind="loading" description="正在读取当前客户资料。" />
      </MobileCard>
    );
  }

  if (state.kind === 'unauthorized') {
    return (
      <MobileCard padding="comfortable" tone="surface">
        <MobileState
          kind="error"
          title="登录状态已失效"
          description="正在重新进入企业微信员工授权。"
        />
      </MobileCard>
    );
  }

  if (state.kind === 'failure') {
    return (
      <MobileCard padding="comfortable" tone="surface">
        <MobileState
          kind={state.stateKind}
          title={state.title}
          description={state.message}
          {...(state.retryable ? {
            actionLabel: '重试',
            onAction: () => setAttempt((value) => value + 1),
          } : {})}
        />
      </MobileCard>
    );
  }

  return (
    <MobileCard padding="comfortable" tone="surface">
      <article className="contact-summary">
        {state.contact.avatar === null ? (
          <div aria-label="客户暂无头像" className="contact-summary__avatar-placeholder" role="img">
            <svg aria-hidden="true" viewBox="0 0 24 24">
              <circle cx="12" cy="8" r="3" />
              <path d="M6.5 19c.7-3.15 2.55-4.75 5.5-4.75s4.8 1.6 5.5 4.75" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
            </svg>
          </div>
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
          <p>企业编号：{state.contact.corpId}</p>
        </div>
      </article>
    </MobileCard>
  );
}
