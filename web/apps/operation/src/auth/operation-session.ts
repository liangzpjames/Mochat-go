import { safeInternalTarget } from '@mochat/mobile-foundation';

export type OperationActivityKind = 'workFission';

export type OperationAuthParams = {
  fissionId: number;
};

export function operationAuthHref(
  activityKind: OperationActivityKind,
  target: string,
  params: OperationAuthParams,
): string {
  if (!Number.isInteger(params.fissionId) || params.fissionId <= 0) {
    throw new Error('活动 ID 必须为正整数。');
  }
  const query = new URLSearchParams({
    id: String(params.fissionId),
    target: safeInternalTarget(target, '/'),
  });
  return `/auth/${activityKind}?${query.toString()}`;
}
