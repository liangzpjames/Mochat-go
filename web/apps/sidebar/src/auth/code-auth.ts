export type LegacyCodeAuthResult =
  | { ok: true; agentId: string; target: string }
  | { ok: false; message: string };

const TARGETS: Readonly<Record<string, string>> = {
  customer: '/contact',
  contact: '/contact',
  mediumGroup: '/medium',
  contactSop: '/contactSop',
  roomSop: '/roomSop',
  contactBatchAdd: '/contactBatchAdd',
};

function decodePayload(value: string): Record<string, unknown> | null {
  try {
    const bytes = Uint8Array.from(atob(value), (character) => character.charCodeAt(0));
    const decoded: unknown = JSON.parse(new TextDecoder().decode(bytes));
    return typeof decoded === 'object' && decoded !== null
      ? decoded as Record<string, unknown>
      : null;
  } catch {
    return null;
  }
}

function positiveString(value: unknown): string | null {
  const text = typeof value === 'number' ? String(value) : value;
  return typeof text === 'string' && /^[1-9]\d*$/.test(text) ? text : null;
}

export function parseLegacyCodeAuth(params: URLSearchParams): LegacyCodeAuthResult {
  const raw = params.get('callValues');
  const envelope = raw === null ? null : decodePayload(raw);
  const data = envelope?.data;
  if (
    envelope === null
    || envelope.code !== 200
    || typeof data !== 'object'
    || data === null
  ) {
    return { ok: false, message: '兼容授权参数无效，请重新进入企业微信应用。' };
  }
  const fields = data as Record<string, unknown>;
  const agentId = positiveString(fields.agentId);
  const act = typeof fields.act === 'string' ? fields.act : '';
  const baseTarget = TARGETS[act];
  if (agentId === null || baseTarget === undefined) {
    return { ok: false, message: '兼容授权参数无效，请重新进入企业微信应用。' };
  }

  const contextKey = act === 'contactBatchAdd' ? 'batchId' : 'id';
  const contextValue = positiveString(fields[contextKey]);
  const needsContext = act === 'contactSop' || act === 'roomSop' || act === 'contactBatchAdd';
  const target = needsContext && contextValue !== null
    ? `${baseTarget}?${contextKey}=${contextValue}`
    : baseTarget;
  return { ok: true, agentId, target };
}
