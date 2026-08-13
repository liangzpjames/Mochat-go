import { MobileApiError } from '@mochat/mobile-foundation';

export type WorkFissionReward = {
  type: number;
  url: string | null;
};

export type WorkFissionTask = {
  level: number;
  target: number;
  completed: boolean;
  received: boolean;
  reward: WorkFissionReward;
};

export type WorkFissionProgress = {
  inviteCount: number;
  differCount: number;
  endTime: number;
  tasks: WorkFissionTask[];
};

export type WorkFissionParticipant = {
  openid: string;
  unionid: string;
  nickname: string;
  headimgurl: string;
};

export type WorkFissionRequest = <T>(path: string, init?: RequestInit) => Promise<T>;

type RawWorkFissionTask = {
  count: number;
  status: 0 | 1;
  receive_status: 0 | 1;
  gift_type: number;
  gift_url: string;
};

type RawWorkFissionProgress = {
  invite_count: number;
  differ_count: number;
  end_time: number;
  task: RawWorkFissionTask[];
};

function isNonNegativeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0;
}

function isPositiveInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value > 0;
}

function isBinaryStatus(value: unknown): value is 0 | 1 {
  return value === 0 || value === 1;
}

function isRawTask(value: unknown): value is RawWorkFissionTask {
  if (typeof value !== 'object' || value === null) return false;
  const task = value as Record<string, unknown>;
  return (
    isNonNegativeInteger(task.count)
    && isBinaryStatus(task.status)
    && isBinaryStatus(task.receive_status)
    && isBinaryStatus(task.gift_type)
    && typeof task.gift_url === 'string'
  );
}

function isRawProgress(value: unknown): value is RawWorkFissionProgress {
  if (typeof value !== 'object' || value === null) return false;
  const progress = value as Record<string, unknown>;
  return (
    isNonNegativeInteger(progress.invite_count)
    && isNonNegativeInteger(progress.differ_count)
    && isPositiveInteger(progress.end_time)
    && Array.isArray(progress.task)
    && progress.task.every(isRawTask)
  );
}

function isWorkFissionParticipant(value: unknown): value is WorkFissionParticipant {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false;
  const participant = value as Record<string, unknown>;
  return (
    typeof participant.openid === 'string'
    && participant.openid.trim().length > 0
    && typeof participant.unionid === 'string'
    && participant.unionid.trim().length > 0
    && typeof participant.nickname === 'string'
    && typeof participant.headimgurl === 'string'
  );
}

export function safeRewardUrl(raw: string): string | null {
  const value = raw.trim();
  const hasControlCharacter = Array.from(value).some((character) => {
    const codePoint = character.codePointAt(0) ?? 0;
    return codePoint <= 0x1f || codePoint === 0x7f;
  });
  if (
    value === ''
    || value.startsWith('//')
    || value.includes('\\')
    || hasControlCharacter
  ) {
    return null;
  }

  if (value.startsWith('/')) {
    try {
      const decodedPath = decodeURIComponent(value.split(/[?#]/, 1)[0] ?? '');
      if (decodedPath.includes('\\')) return null;
      new URL(value, 'https://operation.internal');
      return value;
    } catch {
      return null;
    }
  }

  try {
    const parsed = new URL(value);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? value : null;
  } catch {
    return null;
  }
}

export async function loadWorkFissionParticipant(
  request: WorkFissionRequest,
  fissionId: number,
): Promise<WorkFissionParticipant | null> {
  if (!Number.isInteger(fissionId) || fissionId <= 0) {
    throw new MobileApiError('validation', 'fission_id 必须为正整数');
  }
  const query = new URLSearchParams({ id: String(fissionId) });
  const payload = await request<unknown>(`/openUserInfo/workFission?${query.toString()}`, {
    method: 'GET',
  });
  if (Array.isArray(payload) && payload.length === 0) return null;
  if (!isWorkFissionParticipant(payload)) {
    throw new MobileApiError('validation', '活动参与者会话响应格式无效。');
  }
  return {
    openid: payload.openid.trim(),
    unionid: payload.unionid.trim(),
    nickname: payload.nickname,
    headimgurl: payload.headimgurl,
  };
}

export async function loadWorkFissionProgress(
  request: WorkFissionRequest,
  params: { unionId: string; fissionId: number },
): Promise<WorkFissionProgress> {
  if (params.unionId.trim() === '') {
    throw new MobileApiError('validation', '缺少 union_id');
  }
  if (!Number.isInteger(params.fissionId) || params.fissionId <= 0) {
    throw new MobileApiError('validation', 'fission_id 必须为正整数');
  }
  const query = new URLSearchParams({
    union_id: params.unionId.trim(),
    fission_id: String(params.fissionId),
  });
  const payload = await request<unknown>(`/workFission/taskData?${query.toString()}`, {
    method: 'GET',
  });
  if (!isRawProgress(payload)) {
    throw new MobileApiError('validation', '任务进度响应格式无效。');
  }
  return {
    inviteCount: payload.invite_count,
    differCount: payload.differ_count,
    endTime: payload.end_time,
    tasks: payload.task.map((task, index) => ({
      level: index + 1,
      target: task.count,
      completed: task.status === 1,
      received: task.receive_status === 1,
      reward: {
        type: task.gift_type,
        url: safeRewardUrl(task.gift_url),
      },
    })),
  };
}
