import {
  MobileApiError,
  MobileShell,
  MobileState,
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

type ContactPageState =
  | { kind: 'loading' }
  | { kind: 'success'; contact: ContactSummary }
  | { kind: 'unauthorized' }
  | { kind: 'error'; message: string };

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
        const message = error instanceof Error ? error.message : '客户资料加载失败。';
        setState({ kind: 'error', message });
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

  if (state.kind === 'error') {
    return (
      <MobileShell appName="MoChat 客户侧边栏" title="客户资料">
        <MobileState
          kind="error"
          title="客户资料加载失败"
          description={state.message}
          actionLabel="重试"
          onAction={() => setAttempt((value) => value + 1)}
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
