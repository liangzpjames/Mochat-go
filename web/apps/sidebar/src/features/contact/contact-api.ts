import { MobileApiError } from '@mochat/mobile-foundation';

export type ContactSummary = {
  id: number;
  name: string;
  avatar: string | null;
  corpId: number;
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
