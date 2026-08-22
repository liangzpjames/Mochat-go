import { MobileApiError, MobileCard, MobileState } from '@mochat/mobile-foundation';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';

import {
  loadContactPortrait,
  loadContactSummary,
  updateContactPortrait,
  uploadPortraitImage,
  type ContactPortraitField,
  type ContactRequest,
} from './contact-api';

export function ContactEditPage(props: {
  request: ContactRequest;
  onDone: () => void;
  onReauthenticate: () => void;
}) {
  const [params] = useSearchParams();
  const externalUserId = params.get('wxExternalUserid')?.trim() ?? '';
  const [contactId, setContactId] = useState(0);
  const [fields, setFields] = useState<ContactPortraitField[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState('');
  const [reloadVersion, setReloadVersion] = useState(0);

  useEffect(() => {
    if (!externalUserId) { setMessage('缺少 wxExternalUserid，无法识别当前客户。'); setLoading(false); return; }
    let active = true;
    void loadContactSummary(props.request, externalUserId)
      .then(async (summary) => ({ summary, fields: await loadContactPortrait(props.request, summary.id) }))
      .then((result) => { if (active) { setContactId(result.summary.id); setFields(result.fields); setLoading(false); } })
      .catch((error: unknown) => {
        if (!active) return;
        if (error instanceof MobileApiError && error.kind === 'unauthorized') props.onReauthenticate();
        else { setMessage(error instanceof Error ? error.message : '客户画像加载失败。'); setLoading(false); }
      });
    return () => { active = false; };
  }, [externalUserId, props.onReauthenticate, props.request, reloadVersion]);

  const updateField = (id: number, value: string | string[], pictureUrl?: string) => {
    setFields((items) => items.map((field) => field.contactFieldId === id
      ? { ...field, value, ...(pictureUrl === undefined ? {} : { pictureUrl }) }
      : field));
  };
  const upload = async (field: ContactPortraitField, file: File | undefined) => {
    if (file === undefined) return;
    if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type) || file.size > 5 * 1024 * 1024) {
      setMessage('请选择不超过 5MB 的 PNG、JPG 或 WebP 图片。'); return;
    }
    try { const result = await uploadPortraitImage(props.request, file); updateField(field.contactFieldId, result.path, result.fullPath); }
    catch (error) {
      if (error instanceof MobileApiError && error.kind === 'unauthorized') props.onReauthenticate();
      else setMessage(error instanceof Error ? error.message : '图片上传失败。');
    }
  };
  const submit = async () => {
    if (submitting || contactId <= 0) return;
    setSubmitting(true); setMessage('');
    try { await updateContactPortrait(props.request, contactId, fields); props.onDone(); }
    catch (error) {
      if (error instanceof MobileApiError && error.kind === 'unauthorized') props.onReauthenticate();
      else setMessage(error instanceof Error ? error.message : '画像保存失败，请重试。');
    } finally { setSubmitting(false); }
  };

  if (loading) return <MobileCard padding="comfortable" tone="surface"><MobileState kind="loading" description="正在加载客户画像。" /></MobileCard>;
  if (contactId === 0 && message) return <MobileCard padding="comfortable" tone="surface"><MobileState actionLabel="重试" description={message} kind="error" onAction={() => { setMessage(''); setLoading(true); setReloadVersion((value) => value + 1); }} title="客户画像加载失败" /></MobileCard>;
  if (fields.length === 0 && !message) return <MobileCard padding="comfortable" tone="surface"><MobileState kind="empty" title="暂无可编辑画像字段" /></MobileCard>;
  return (
    <MobileCard padding="comfortable" tone="surface">
      <form className="sidebar-form" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
        {fields.map((field) => {
          if (field.typeText === '多选') return (
            <fieldset className="sidebar-choice-grid" key={field.contactFieldId}>
              <legend>{field.name}</legend>
              {field.options.map((option) => {
                const values = Array.isArray(field.value) ? field.value : [];
                return <label key={option}><input aria-label={option} checked={values.includes(option)} onChange={(event) => updateField(field.contactFieldId, event.target.checked ? [...values, option] : values.filter((item) => item !== option))} type="checkbox" />{option}</label>;
              })}
            </fieldset>
          );
          if (field.typeText === '单选') return (
            <fieldset className="sidebar-choice-grid" key={field.contactFieldId}>
              <legend>{field.name}</legend>
              {field.options.map((option) => <label key={option}><input aria-label={option} checked={field.value === option} name={`field-${field.contactFieldId}`} onChange={() => updateField(field.contactFieldId, option)} type="radio" />{option}</label>)}
            </fieldset>
          );
          if (field.typeText === '下拉') return <label key={field.contactFieldId}>{field.name}<select aria-label={field.name} onChange={(event) => updateField(field.contactFieldId, event.target.value)} value={String(field.value)}><option value="">请选择</option>{field.options.map((option) => <option key={option}>{option}</option>)}</select></label>;
          if (field.typeText === '图片') return <label key={field.contactFieldId}>{field.name}<input accept="image/png,image/jpeg,image/webp" aria-label={field.name} onChange={(event) => { void upload(field, event.target.files?.[0]); }} type="file" />{field.pictureUrl ? <img alt={`${field.name}当前图片`} className="sidebar-portrait-image" src={field.pictureUrl} /> : null}</label>;
          return <label key={field.contactFieldId}>{field.name}<input aria-label={field.name} onChange={(event) => updateField(field.contactFieldId, event.target.value)} type={field.typeText === '日期' ? 'date' : 'text'} value={String(field.value)} /></label>;
        })}
        {message ? <p className="sidebar-form__error" role="alert">{message}</p> : null}
        <div className="sidebar-form__actions">
          <button className="sidebar-form__secondary" disabled={submitting} onClick={props.onDone} type="button">取消</button>
          <button className="sidebar-form__primary" disabled={submitting} type="submit">{submitting ? '保存中' : '保存画像'}</button>
        </div>
      </form>
    </MobileCard>
  );
}
