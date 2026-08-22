import { MobileApiError, MobileCard, MobileState } from '@mochat/mobile-foundation';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';

import {
  loadContactSummary,
  loadContactWorkspace,
  updateContactRemark,
  type ContactRequest,
} from './contact-api';

export function ContactRemarkPage(props: {
  request: ContactRequest;
  onDone: () => void;
  onReauthenticate: () => void;
}) {
  const [params] = useSearchParams();
  const externalUserId = params.get('wxExternalUserid')?.trim() ?? '';
  const [state, setState] = useState<
    | { kind: 'loading' }
    | { kind: 'error'; message: string }
    | { kind: 'ready'; contactId: number; remark: string }
  >({ kind: 'loading' });
  const [message, setMessage] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [reloadVersion, setReloadVersion] = useState(0);

  useEffect(() => {
    if (!externalUserId) {
      setState({ kind: 'error', message: '缺少 wxExternalUserid，无法识别当前客户。' });
      return;
    }
    let active = true;
    void loadContactSummary(props.request, externalUserId)
      .then(async (summary) => ({ summary, workspace: await loadContactWorkspace(props.request, summary.id) }))
      .then(({ summary, workspace }) => {
        if (active) setState({ kind: 'ready', contactId: summary.id, remark: workspace.remark });
      })
      .catch((error: unknown) => {
        if (!active) return;
        if (error instanceof MobileApiError && error.kind === 'unauthorized') props.onReauthenticate();
        else setState({ kind: 'error', message: error instanceof Error ? error.message : '备注加载失败。' });
      });
    return () => { active = false; };
  }, [externalUserId, props.onReauthenticate, props.request, reloadVersion]);

  if (state.kind === 'loading') return <MobileCard padding="comfortable" tone="surface"><MobileState kind="loading" description="正在加载当前备注。" /></MobileCard>;
  if (state.kind === 'error') return <MobileCard padding="comfortable" tone="surface"><MobileState actionLabel="重试" kind="error" onAction={() => { setState({ kind: 'loading' }); setReloadVersion((value) => value + 1); }} title="备注加载失败" description={state.message} /></MobileCard>;

  const submit = async () => {
    const remark = state.remark.trim();
    if (Array.from(remark).length < 1 || Array.from(remark).length > 10) {
      setMessage('请输入 1 至 10 个字符的备注名。');
      return;
    }
    if (submitting) return;
    setSubmitting(true);
    setMessage('');
    try {
      await updateContactRemark(props.request, state.contactId, remark);
      props.onDone();
    } catch (error) {
      if (error instanceof MobileApiError && error.kind === 'unauthorized') props.onReauthenticate();
      else setMessage(error instanceof Error ? error.message : '备注保存失败，请重试。');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <MobileCard padding="comfortable" tone="surface">
      <form className="sidebar-form" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
        <label htmlFor="contact-remark">备注名</label>
        <input
          id="contact-remark"
          maxLength={10}
          onChange={(event) => setState({ ...state, remark: event.target.value })}
          value={state.remark}
        />
        <p className="sidebar-form__hint">最多 10 个字符，保存后同步到当前员工的客户备注。</p>
        {message ? <p className="sidebar-form__error" role="alert">{message}</p> : null}
        <div className="sidebar-form__actions">
          <button className="sidebar-form__secondary" disabled={submitting} onClick={props.onDone} type="button">取消</button>
          <button className="sidebar-form__primary" disabled={submitting} type="submit">
            {submitting ? '保存中' : '保存备注'}
          </button>
        </div>
      </form>
    </MobileCard>
  );
}
