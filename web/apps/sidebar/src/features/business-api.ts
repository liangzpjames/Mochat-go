import { MobileApiError } from '@mochat/mobile-foundation';

export type BusinessRequest = <T>(path: string, init?: RequestInit) => Promise<T>;
export type MediumKind = 'text' | 'image' | 'link' | 'video' | 'file' | 'unknown';
export type MediumGroup = { id: number; name: string };
export type MediumItem = { id: number; kind: MediumKind; typeLabel: string; mediaId: string; content: Record<string, string> };
export type SopContent = { type: string; value: string };
export type ContactSop = { taskId: number; customerName: string; avatar: string | null; creator: string; time: string; tipTime: string; content: SopContent[] };
export type RoomSop = { taskId: number; roomName: string; creator: string; time: string; state: number; content: SopContent[] };

function object(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : null;
}

function integer(value: unknown): value is number { return typeof value === 'number' && Number.isSafeInteger(value); }
function fail(message: string): never { throw new MobileApiError('validation', message); }
function positive(id: number, label: string): void { if (!Number.isSafeInteger(id) || id <= 0) fail(`${label}无效。`); }
function put(body: unknown): RequestInit {
  return { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) };
}

export async function loadMediumGroups(request: BusinessRequest): Promise<MediumGroup[]> {
  const raw = await request<unknown>('/mediumGroup/index', { method: 'GET' });
  if (!Array.isArray(raw)) fail('素材分组响应格式无效。');
  return raw.map((entry) => {
    const item = object(entry);
    if (item === null || !integer(item.id) || item.id < 0 || typeof item.name !== 'string') fail('素材分组响应格式无效。');
    return { id: item.id, name: item.name };
  });
}

function mediumKind(label: string): MediumKind {
  return ({ '文本': 'text', '图片': 'image', '图文': 'link', '视频': 'video', '文件': 'file' } as Record<string, MediumKind>)[label] ?? 'unknown';
}

function stringContent(value: unknown): Record<string, string> {
  const raw = object(value);
  if (raw === null) fail('素材内容响应格式无效。');
  const content: Record<string, string> = {};
  for (const [key, entry] of Object.entries(raw)) {
    if (typeof entry === 'string') content[key] = entry;
  }
  return content;
}

export async function loadMediums(request: BusinessRequest, filter: { groupId: number | null; keyword: string; page: number }): Promise<{ items: MediumItem[]; total: number; totalPage: number }> {
  if (!Number.isSafeInteger(filter.page) || filter.page <= 0) fail('页码无效。');
  const query = new URLSearchParams();
  if (filter.groupId !== null) query.set('mediumGroupId', String(filter.groupId));
  if (filter.keyword.trim()) query.set('searchStr', filter.keyword.trim());
  query.set('page', String(filter.page)); query.set('perPage', '20');
  const raw = object(await request<unknown>(`/medium/index?${query.toString()}`, { method: 'GET' }));
  const page = raw === null ? null : object(raw.page);
  if (raw === null || page === null || !integer(page.total) || !integer(page.totalPage) || !Array.isArray(raw.list)) fail('素材列表响应格式无效。');
  const items = raw.list.map((entry): MediumItem => {
    const item = object(entry);
    if (item === null || !integer(item.id) || item.id <= 0 || typeof item.type !== 'string' || typeof item.mediaId !== 'string') fail('素材列表响应格式无效。');
    return { id: item.id, kind: mediumKind(item.type), typeLabel: item.type, mediaId: item.mediaId, content: stringContent(item.content) };
  });
  return { items, total: page.total, totalPage: page.totalPage };
}

export async function refreshMediumMediaId(request: BusinessRequest, id: number): Promise<string> {
  positive(id, '素材 ID');
  const raw = object(await request<unknown>(`/medium/mediaIdUpdate?mediumId=${id}`, { method: 'GET' }));
  if (raw === null || typeof raw.mediaId !== 'string') fail('素材临时凭证响应格式无效。');
  return raw.mediaId;
}

function sopContent(task: unknown): SopContent[] {
  const raw = object(task);
  if (raw === null || !Array.isArray(raw.content)) fail('SOP 任务响应格式无效。');
  return raw.content.map((entry) => {
    const item = object(entry);
    if (item === null || !(typeof item.type === 'string' || typeof item.type === 'number') || typeof item.value !== 'string') fail('SOP 任务响应格式无效。');
    return { type: String(item.type), value: item.value };
  });
}

function parseContactSop(value: unknown): ContactSop {
  const raw = object(value); const contact = raw === null ? null : object(raw.contact);
  if (raw === null || contact === null || !integer(raw.id) || raw.id <= 0 || typeof raw.creator !== 'string' || typeof raw.time !== 'string' || typeof raw.tipTime !== 'string' || typeof contact.name !== 'string' || !(typeof contact.avatar === 'string' || contact.avatar === null)) fail('个人 SOP 响应格式无效。');
  return { taskId: raw.id, customerName: contact.name, avatar: contact.avatar, creator: raw.creator, time: raw.time, tipTime: raw.tipTime, content: sopContent(raw.task) };
}

export async function loadContactSop(request: BusinessRequest, id: number): Promise<ContactSop> {
  positive(id, '任务 ID');
  return parseContactSop(await request<unknown>(`/contactSop/getSopInfo?id=${id}`, { method: 'GET' }));
}

export async function loadContactSopTips(request: BusinessRequest, contactId: number): Promise<ContactSop[]> {
  positive(contactId, '客户 ID');
  const raw = await request<unknown>(`/contactSop/getSopTipInfo?contactId=${contactId}`, { method: 'GET' });
  if (!Array.isArray(raw)) fail('个人 SOP 提醒响应格式无效。');
  return raw.map(parseContactSop);
}

export async function loadRoomSop(request: BusinessRequest, id: number): Promise<RoomSop> {
  positive(id, '任务 ID');
  const raw = object(await request<unknown>(`/roomSop/getSopInfo?id=${id}`, { method: 'GET' }));
  const room = raw === null ? null : object(raw.room);
  if (raw === null || room === null || !integer(raw.id) || raw.id <= 0 || !integer(raw.state) || typeof raw.creator !== 'string' || typeof raw.time !== 'string' || typeof room.name !== 'string') fail('群 SOP 响应格式无效。');
  return { taskId: raw.id, roomName: room.name, creator: raw.creator, time: raw.time, state: raw.state, content: sopContent(raw.task) };
}

export async function updateRoomSopState(request: BusinessRequest, id: number): Promise<void> {
  positive(id, '任务 ID');
  await request<unknown>('/roomSop/logState', put({ id }));
}

export async function loadBatchAdd(request: BusinessRequest, batchId: number, status: number): Promise<{ employeeName: string; contacts: Array<{ id: number; phone: string; status: string }> }> {
  positive(batchId, '批次 ID');
  if (!Number.isSafeInteger(status) || status < 0 || status > 4) fail('筛选状态无效。');
  const raw = object(await request<unknown>(`/contactBatchAdd/detail?batchId=${batchId}&status=${status}`, { method: 'GET' }));
  if (raw === null || typeof raw.employeeName !== 'string' || !Array.isArray(raw.list)) fail('批量加好友响应格式无效。');
  const contacts = raw.list.map((entry) => {
    const item = object(entry);
    if (item === null || !integer(item.id) || typeof item.phone !== 'string' || typeof item.status !== 'string') fail('批量加好友响应格式无效。');
    return { id: item.id, phone: item.phone, status: item.status };
  });
  return { employeeName: raw.employeeName, contacts };
}
