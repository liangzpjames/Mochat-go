import {
  MobileApiError,
  MobileCard,
  MobileState,
  type MobileStateKind,
} from '@mochat/mobile-foundation';
import { useEffect, useState } from 'react';
import { useHref, useSearchParams } from 'react-router';

import {
  loadContactSummary,
  loadContactPortrait,
  loadContactTrack,
  loadContactSopReminders,
  loadContactWorkspace,
  type ContactRequest,
  type ContactPortraitField,
  type ContactSummary,
  type ContactTrack,
  type ContactSopReminder,
  type ContactWorkspace,
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

type ContactSecondaryState =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | {
      kind: 'settled';
      workspace: SecondaryPart<ContactWorkspace>;
      tracks: SecondaryPart<ContactTrack[]>;
      portrait: SecondaryPart<ContactPortraitField[]>;
      reminders: SecondaryPart<ContactSopReminder[]>;
    };

type SecondaryPart<T> = { kind: 'success'; value: T } | { kind: 'failure'; message: string };

function secondaryPart<T>(result: PromiseSettledResult<T>): SecondaryPart<T> {
  return result.status === 'fulfilled'
    ? { kind: 'success', value: result.value }
    : { kind: 'failure', message: result.reason instanceof Error ? result.reason.message : '请稍后重试。' };
}

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
  const [secondaryAttempt, setSecondaryAttempt] = useState(0);
  const [secondary, setSecondary] = useState<ContactSecondaryState>({ kind: 'idle' });
  const context = new URLSearchParams();
  if (externalUserId.length > 0) context.set('wxExternalUserid', externalUserId);
  const agentId = params.get('agentId')?.trim();
  if (agentId) context.set('agentId', agentId);
  const suffix = context.size === 0 ? '' : `?${context.toString()}`;
  const remarkHref = useHref(`/contact/remark${suffix}`);
  const tagHref = useHref(`/contact/settingTag${suffix}`);
  const portraitHref = useHref(`/contact/editDetail${suffix}`);
  const sopContext = new URLSearchParams(context);
  if (state.kind === 'success') sopContext.set('contactId', String(state.contact.id));
  const sopHref = useHref(`/contactSop${sopContext.size === 0 ? '' : `?${sopContext.toString()}`}`);

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

  useEffect(() => {
    if (state.kind !== 'success') return undefined;
    let active = true;
    setSecondary({ kind: 'loading' });
    void Promise.allSettled([
      loadContactWorkspace(request, state.contact.id),
      loadContactTrack(request, state.contact.id),
      loadContactPortrait(request, state.contact.id),
      loadContactSopReminders(request, state.contact.id),
    ]).then(([workspace, tracks, portrait, reminders]) => {
      if (!active) return;
      const unauthorized = [workspace, tracks, portrait, reminders].find((result) => result.status === 'rejected' && result.reason instanceof MobileApiError && result.reason.kind === 'unauthorized');
      if (unauthorized) {
        onReauthenticate();
        return;
      }
      setSecondary({ kind: 'settled', workspace: secondaryPart(workspace), tracks: secondaryPart(tracks), portrait: secondaryPart(portrait), reminders: secondaryPart(reminders) });
    });
    return () => {
      active = false;
    };
  }, [onReauthenticate, request, secondaryAttempt, state]);

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
    <div className="contact-workspace">
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
      <nav aria-label="客户资料操作" className="contact-workspace__actions">
        <a href={remarkHref}>修改备注</a>
        <a href={tagHref}>设置标签</a>
        <a href={portraitHref}>编辑画像</a>
      </nav>
      {secondary.kind === 'loading' || secondary.kind === 'idle' ? (
        <MobileCard padding="comfortable" tone="surface">
          <MobileState kind="loading" title="正在加载客户扩展资料" />
        </MobileCard>
      ) : (
        <>
          <MobileCard padding="comfortable" tone="surface">
            {secondary.workspace.kind === 'failure' ? <MobileState actionLabel="重试客户概览" description={secondary.workspace.message} kind="error" onAction={() => setSecondaryAttempt((value) => value + 1)} title="客户概览加载失败" /> : (
            <section aria-labelledby="contact-basics-heading" className="contact-workspace__section">
              <h3 id="contact-basics-heading">客户概览</h3>
              <dl className="contact-workspace__details">
                <div><dt>备注</dt><dd>{secondary.workspace.value.remark || '暂无备注'}</dd></div>
                <div><dt>性别</dt><dd>{secondary.workspace.value.genderText || '未知'}</dd></div>
                <div><dt>客户编号</dt><dd>{secondary.workspace.value.businessNo || '暂无'}</dd></div>
                <div><dt>描述</dt><dd>{secondary.workspace.value.description || '暂无描述'}</dd></div>
                <div><dt>所在群</dt><dd>{secondary.workspace.value.roomNames.join('、') || '暂未加入客户群'}</dd></div>
                <div><dt>归属员工</dt><dd>{secondary.workspace.value.employeeNames.join('、') || '暂无'}</dd></div>
              </dl>
              <div aria-label="客户标签" className="contact-workspace__tags">
                {secondary.workspace.value.tags.length === 0
                  ? <span className="contact-workspace__muted">暂无标签</span>
                  : secondary.workspace.value.tags.map((tag) => <span key={tag.id}>{tag.name}</span>)}
              </div>
            </section>)}
          </MobileCard>
          <MobileCard padding="comfortable" tone="surface">
            {secondary.tracks.kind === 'failure' ? <MobileState actionLabel="重试互动轨迹" description={secondary.tracks.message} kind="error" onAction={() => setSecondaryAttempt((value) => value + 1)} title="互动轨迹加载失败" /> : <section aria-labelledby="contact-track-heading" className="contact-workspace__section">
              <h3 id="contact-track-heading">互动轨迹</h3>
              {secondary.tracks.value.length === 0 ? <p className="contact-workspace__muted">暂无互动轨迹</p> : (
                <ol className="contact-workspace__timeline">
                  {secondary.tracks.value.map((track, index) => (
                    <li key={track.id ?? `${track.createdAt}-${index}`}>
                      <time>{track.createdAt}</time><p>{track.content}</p>
                    </li>
                  ))}
                </ol>
              )}
            </section>}
          </MobileCard>
          <MobileCard padding="comfortable" tone="surface">
            {secondary.portrait.kind === 'failure' ? <MobileState actionLabel="重试客户画像" description={secondary.portrait.message} kind="error" onAction={() => setSecondaryAttempt((value) => value + 1)} title="客户画像加载失败" /> : <section aria-labelledby="contact-portrait-heading" className="contact-workspace__section">
              <h3 id="contact-portrait-heading">客户画像</h3>
              {secondary.portrait.value.length === 0 ? <p className="contact-workspace__muted">暂无画像字段</p> : (
                <ul className="contact-workspace__portrait">
                  {secondary.portrait.value.map((field) => (
                    <li key={field.contactFieldId}>
                      {field.name}：{Array.isArray(field.value) ? field.value.join('、') || '暂无' : field.value || '暂无'}
                    </li>
                  ))}
                </ul>
              )}
            </section>}
          </MobileCard>
          <MobileCard padding="comfortable" tone="surface">
            {secondary.reminders.kind === 'failure' ? <MobileState actionLabel="重试 SOP 提醒" description={secondary.reminders.message} kind="error" onAction={() => setSecondaryAttempt((value) => value + 1)} title="SOP 提醒加载失败" /> : <section aria-labelledby="contact-sop-heading" className="contact-workspace__section"><h3 id="contact-sop-heading">SOP 提醒</h3><p className="contact-workspace__muted">{secondary.reminders.value.length === 0 ? '当前客户暂无待办提醒' : `当前有 ${secondary.reminders.value.length} 条待办提醒`}</p><a className="sidebar-primary-link" href={sopHref}>查看 SOP 提醒</a></section>}
          </MobileCard>
        </>
      )}
    </div>
  );
}
