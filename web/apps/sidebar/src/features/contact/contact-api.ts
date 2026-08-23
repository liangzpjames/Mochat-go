import { MobileApiError } from '@mochat/mobile-foundation';

export type ContactSummary = {
  id: number;
  name: string;
  avatar: string | null;
  corpId: number;
};

export type ContactTag = { id: number; name: string };

export type ContactWorkspace = {
  name: string;
  avatar: string | null;
  gender: number;
  genderText: string;
  businessNo: string;
  remark: string;
  description: string;
  tags: ContactTag[];
  roomNames: string[];
  employeeNames: string[];
};

export type ContactTrack = {
  id: number | null;
  content: string;
  createdAt: string;
};
export type ContactSopReminder = { id: number; time: string };

export type ContactPortraitValue = string | string[];

export type ContactPortraitField = {
  contactFieldId: number;
  pivotId: number | null;
  name: string;
  type: number;
  typeText: string;
  options: string[];
  value: ContactPortraitValue;
  pictureUrl: string | null;
};

export type ContactRequest = <T>(path: string, init?: RequestInit) => Promise<T>;

function isContactSummary(value: unknown): value is ContactSummary {
  if (typeof value !== 'object' || value === null) return false;
  const contact = value as Record<string, unknown>;
  return (
    typeof contact.id === 'number'
    && Number.isInteger(contact.id)
    && typeof contact.name === 'string'
    && contact.name.trim().length > 0
    && (typeof contact.avatar === 'string' || contact.avatar === null)
    && typeof contact.corpId === 'number'
    && Number.isInteger(contact.corpId)
  );
}

function record(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null ? value as Record<string, unknown> : null;
}

function integer(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value);
}

function strings(value: unknown): string[] | null {
  if (!Array.isArray(value)) return null;
  const result: string[] = [];
  for (const item of value) {
    if (typeof item !== 'string') return null;
    result.push(item);
  }
  return result;
}

function validation(message: string): never {
  throw new MobileApiError('validation', message);
}

function jsonRequest(body: unknown): RequestInit {
  return {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  };
}

export async function loadContactSummary(
  request: ContactRequest,
  externalUserId: string,
): Promise<ContactSummary> {
  const query = new URLSearchParams({ wxExternalUserid: externalUserId });
  const payload = await request<unknown>(`/workContact/detail?${query.toString()}`, {
    method: 'GET',
  });
  if (!isContactSummary(payload)) {
    throw new MobileApiError('validation', '客户资料响应格式无效。');
  }
  return payload;
}

export async function loadContactWorkspace(
  request: ContactRequest,
  contactId: number,
): Promise<ContactWorkspace> {
  const query = new URLSearchParams({ contactId: String(contactId) });
  const raw = record(await request<unknown>(`/workContact/show?${query.toString()}`, { method: 'GET' }));
  if (raw === null) validation('客户详情响应格式无效。');
  const rawTags = raw.tag;
  const roomNames = strings(raw.roomName);
  const employeeNames = strings(raw.employeeName);
  if (
    typeof raw.name !== 'string'
    || raw.name.trim().length === 0
    || !(typeof raw.avatar === 'string' || raw.avatar === null)
    || !integer(raw.gender)
    || typeof raw.genderText !== 'string'
    || typeof raw.businessNo !== 'string'
    || typeof raw.remark !== 'string'
    || typeof raw.description !== 'string'
    || !Array.isArray(rawTags)
    || roomNames === null
    || employeeNames === null
  ) validation('客户详情响应格式无效。');

  const tags = rawTags.map((item): ContactTag => {
    const tag = record(item);
    if (tag === null || !integer(tag.tagId) || tag.tagId <= 0 || typeof tag.tagName !== 'string') {
      return validation('客户标签响应格式无效。');
    }
    return { id: tag.tagId, name: tag.tagName };
  });
  return {
    name: raw.name,
    avatar: raw.avatar,
    gender: raw.gender,
    genderText: raw.genderText,
    businessNo: raw.businessNo,
    remark: raw.remark,
    description: raw.description,
    tags,
    roomNames,
    employeeNames,
  };
}

export async function loadContactTrack(
  request: ContactRequest,
  contactId: number,
): Promise<ContactTrack[]> {
  const query = new URLSearchParams({ contactId: String(contactId) });
  const raw = await request<unknown>(`/workContact/track?${query.toString()}`, { method: 'GET' });
  if (!Array.isArray(raw)) validation('互动轨迹响应格式无效。');
  return raw.map((item) => {
    const track = record(item);
    if (track === null || typeof track.content !== 'string' || typeof track.createdAt !== 'string') {
      return validation('互动轨迹响应格式无效。');
    }
    if (!(track.id === undefined || integer(track.id))) validation('互动轨迹响应格式无效。');
    return { id: integer(track.id) ? track.id : null, content: track.content, createdAt: track.createdAt };
  });
}

export async function loadContactSopReminders(request: ContactRequest, contactId: number): Promise<ContactSopReminder[]> {
  if (!Number.isSafeInteger(contactId) || contactId <= 0) validation('客户 ID 无效。');
  const raw = await request<unknown>(`/contactSop/getSopTipInfo?contactId=${contactId}`, { method: 'GET' });
  if (!Array.isArray(raw)) validation('客户 SOP 提醒响应格式无效。');
  return raw.map((entry) => {
    const item = record(entry);
    if (item === null || !integer(item.id) || item.id <= 0 || typeof item.time !== 'string') validation('客户 SOP 提醒响应格式无效。');
    return { id: item.id, time: item.time };
  });
}

export async function loadContactPortrait(
  request: ContactRequest,
  contactId: number,
): Promise<ContactPortraitField[]> {
  const query = new URLSearchParams({ contactId: String(contactId) });
  const raw = await request<unknown>(`/contactFieldPivot/index?${query.toString()}`, { method: 'GET' });
  if (!Array.isArray(raw)) validation('客户画像响应格式无效。');
  return raw.map((item) => {
    const field = record(item);
    if (field === null) return validation('客户画像响应格式无效。');
    const options = strings(field.options);
    const pivotId = field.contactFieldPivotId === '' ? null : field.contactFieldPivotId;
    const value = field.value;
    if (
      !integer(field.contactFieldId)
      || field.contactFieldId <= 0
      || !(pivotId === null || (integer(pivotId) && pivotId > 0))
      || typeof field.name !== 'string'
      || !integer(field.type)
      || typeof field.typeText !== 'string'
      || options === null
      || !(typeof value === 'string' || strings(value) !== null)
      || !(field.pictureFlag === undefined || typeof field.pictureFlag === 'string')
    ) return validation('客户画像响应格式无效。');
    return {
      contactFieldId: field.contactFieldId,
      pivotId,
      name: field.name,
      type: field.type,
      typeText: field.typeText,
      options,
      value: typeof value === 'string' ? value : value as string[],
      pictureUrl: typeof field.pictureFlag === 'string' && field.pictureFlag.length > 0
        ? field.pictureFlag
        : null,
    };
  });
}

export async function updateContactPortrait(
  request: ContactRequest,
  contactId: number,
  fields: readonly ContactPortraitField[],
): Promise<void> {
  await request<unknown>('/contactFieldPivot/update', jsonRequest({
    contactId,
    userPortrait: fields.map((field) => ({
      contactFieldPivotId: field.pivotId ?? '',
      contactFieldId: field.contactFieldId,
      name: field.name,
      type: field.type,
      value: field.value,
    })),
  }));
}

export type ContactUpdateOutcome = {
  savedLocally: boolean;
  wecomSynced: boolean;
  retryable: boolean;
};

function contactUpdateOutcome(raw: unknown): ContactUpdateOutcome {
  if (Array.isArray(raw)) {
    return { savedLocally: true, wecomSynced: true, retryable: false };
  }
  const outcome = record(raw);
  if (
    outcome === null
    || typeof outcome.savedLocally !== 'boolean'
    || typeof outcome.wecomSynced !== 'boolean'
    || typeof outcome.retryable !== 'boolean'
  ) validation('客户更新响应格式无效。');
  return {
    savedLocally: outcome.savedLocally,
    wecomSynced: outcome.wecomSynced,
    retryable: outcome.retryable,
  };
}

export async function updateContactRemark(
  request: ContactRequest,
  contactId: number,
  remark: string,
): Promise<ContactUpdateOutcome> {
  return contactUpdateOutcome(await request<unknown>('/workContact/update', jsonRequest({ contactId, remark })));
}

export async function appendContactTags(
  request: ContactRequest,
  contactId: number,
  tagIds: number[],
): Promise<ContactUpdateOutcome> {
  return contactUpdateOutcome(await request<unknown>('/workContact/update', jsonRequest({
    contactId,
    tag: [...new Set(tagIds)],
  })));
}

export async function loadTagGroups(request: ContactRequest): Promise<ContactTag[]> {
  const raw = await request<unknown>('/workContactTagGroup/index', { method: 'GET' });
  if (!Array.isArray(raw)) validation('标签分组响应格式无效。');
  return raw.map((item) => {
    const group = record(item);
    if (group === null || !integer(group.groupId) || typeof group.groupName !== 'string') {
      return validation('标签分组响应格式无效。');
    }
    return { id: group.groupId, name: group.groupName };
  });
}

export async function loadTags(
  request: ContactRequest,
  groupId: number | null,
): Promise<ContactTag[]> {
  const query = new URLSearchParams();
  if (groupId !== null) query.set('groupId', String(groupId));
  const suffix = query.size === 0 ? '' : `?${query.toString()}`;
  const raw = await request<unknown>(`/workContactTag/allTag${suffix}`, { method: 'GET' });
  if (!Array.isArray(raw)) validation('标签响应格式无效。');
  return raw.map((item) => {
    const tag = record(item);
    if (tag === null || !integer(tag.id) || tag.id <= 0 || typeof tag.name !== 'string') {
      return validation('标签响应格式无效。');
    }
    return { id: tag.id, name: tag.name };
  });
}

export async function uploadPortraitImage(
  request: ContactRequest,
  file: File,
): Promise<{ path: string; fullPath: string }> {
  const body = new FormData();
  body.append('file', file);
  const raw = record(await request<unknown>('/common/upload', { method: 'POST', body }));
  if (raw === null || typeof raw.path !== 'string' || typeof raw.fullPath !== 'string') {
    validation('图片上传响应格式无效。');
  }
  return { path: raw.path, fullPath: raw.fullPath };
}
