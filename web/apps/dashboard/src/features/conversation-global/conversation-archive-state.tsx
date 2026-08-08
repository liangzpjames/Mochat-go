import { ApiError } from '@mochat/api-client';
import { Link } from 'react-router';

import { DashboardDataState } from '../../components/dashboard-data-state';

export function isConversationArchiveUnavailable(error: unknown): boolean {
  return error instanceof ApiError && error.status === 403 && error.code === 40301;
}

export function ConversationArchiveUnavailableState() {
  return (
    <DashboardDataState
      action={<Link to="/company-setting/website">去配置会话归档</Link>}
      description="请先完成企业微信会话内容存档授权并启用归档同步。"
      state="provider-unavailable"
      title="会话归档未开通"
    />
  );
}
