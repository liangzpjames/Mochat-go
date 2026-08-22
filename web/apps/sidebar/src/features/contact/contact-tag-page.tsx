import { MobileApiError, MobileCard, MobileState } from '@mochat/mobile-foundation';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';

import {
  appendContactTags,
  loadContactSummary,
  loadContactWorkspace,
  loadTagGroups,
  loadTags,
  type ContactRequest,
  type ContactTag,
} from './contact-api';

export function ContactTagPage(props: {
  request: ContactRequest;
  onDone: () => void;
  onReauthenticate: () => void;
}) {
  const [params] = useSearchParams();
  const externalUserId = params.get('wxExternalUserid')?.trim() ?? '';
  const [contactId, setContactId] = useState(0);
  const [groups, setGroups] = useState<ContactTag[]>([]);
  const [tags, setTags] = useState<ContactTag[]>([]);
  const [existing, setExisting] = useState<number[]>([]);
  const [selected, setSelected] = useState<number[]>([]);
  const [groupId, setGroupId] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState('');
  const [reloadVersion, setReloadVersion] = useState(0);

  useEffect(() => {
    if (!externalUserId) {
      setMessage('缺少 wxExternalUserid，无法识别当前客户。');
      setLoading(false);
      return;
    }
    let active = true;
    void (async () => {
      const summary = await loadContactSummary(props.request, externalUserId);
      const workspace = await loadContactWorkspace(props.request, summary.id);
      const loadedGroups = await loadTagGroups(props.request);
      const loadedTags = await loadTags(props.request, null);
      if (!active) return;
      const current = workspace.tags.map((tag) => tag.id);
      setContactId(summary.id);
      setExisting(current);
      setSelected(current);
      setGroups(loadedGroups);
      setTags(loadedTags);
      setLoading(false);
    })().catch((error: unknown) => {
      if (!active) return;
      if (error instanceof MobileApiError && error.kind === 'unauthorized') props.onReauthenticate();
      else { setMessage(error instanceof Error ? error.message : '标签加载失败。'); setLoading(false); }
    });
    return () => { active = false; };
  }, [externalUserId, props.onReauthenticate, props.request, reloadVersion]);

  const changeGroup = async (value: string) => {
    const next = value === '' ? null : Number(value);
    setGroupId(next);
    setLoading(true);
    try { setTags(await loadTags(props.request, next)); }
    catch (error) {
      if (error instanceof MobileApiError && error.kind === 'unauthorized') props.onReauthenticate();
      else setMessage(error instanceof Error ? error.message : '标签加载失败。');
    }
    finally { setLoading(false); }
  };

  const submit = async () => {
    if (submitting || contactId <= 0) return;
    const added = selected.filter((id) => !existing.includes(id));
    if (added.length === 0) { setMessage('请选择至少一个新标签。'); return; }
    setSubmitting(true);
    setMessage('');
    try { await appendContactTags(props.request, contactId, added); props.onDone(); }
    catch (error) {
      if (error instanceof MobileApiError && error.kind === 'unauthorized') props.onReauthenticate();
      else setMessage(error instanceof Error ? error.message : '标签保存失败，请重试。');
    } finally { setSubmitting(false); }
  };

  if (loading && contactId === 0) return <MobileCard padding="comfortable" tone="surface"><MobileState kind="loading" description="正在加载企业标签。" /></MobileCard>;
  if (!loading && contactId === 0 && message) return <MobileCard padding="comfortable" tone="surface"><MobileState actionLabel="重试" description={message} kind="error" onAction={() => { setMessage(''); setLoading(true); setReloadVersion((value) => value + 1); }} title="标签加载失败" /></MobileCard>;
  return (
    <div className="sidebar-form-stack">
      <MobileCard padding="comfortable" tone="surface">
        <label className="sidebar-form__label" htmlFor="tag-group">标签分组</label>
        <select id="tag-group" onChange={(event) => { void changeGroup(event.target.value); }} value={groupId ?? ''}>
          <option value="">全部分组</option>
          {groups.map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}
        </select>
        <p className="sidebar-form__hint">现有标签由后端契约保留，本页仅追加新标签。</p>
      </MobileCard>
      <MobileCard padding="comfortable" tone="surface">
        {loading ? <MobileState kind="loading" title="正在筛选标签" /> : tags.length === 0 ? <MobileState kind="empty" title="该分组暂无标签" /> : (
          <fieldset className="sidebar-choice-grid">
            <legend>选择标签</legend>
            {tags.map((tag) => {
              const locked = existing.includes(tag.id);
              return (
                <label key={tag.id}>
                  <input
                    aria-label={tag.name}
                    checked={selected.includes(tag.id)}
                    disabled={locked}
                    onChange={(event) => setSelected((value) => event.target.checked ? [...new Set([...value, tag.id])] : value.filter((id) => id !== tag.id))}
                    type="checkbox"
                  />
                  {locked ? `${tag.name}（已存在）` : tag.name}
                </label>
              );
            })}
          </fieldset>
        )}
        {message ? <p className="sidebar-form__error" role="alert">{message}</p> : null}
        <div className="sidebar-form__actions">
          <button className="sidebar-form__secondary" disabled={submitting} onClick={props.onDone} type="button">取消</button>
          <button className="sidebar-form__primary" disabled={submitting || loading || selected.every((id) => existing.includes(id))} onClick={() => { void submit(); }} type="button">
            {submitting ? '保存中' : '保存标签'}
          </button>
        </div>
      </MobileCard>
    </div>
  );
}
