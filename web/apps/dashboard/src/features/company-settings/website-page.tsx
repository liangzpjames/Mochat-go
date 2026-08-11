import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { ApiError } from '@mochat/api-client';

import { useOptionalDashboardAccess } from '../../app/access-context';
import { ConfirmAction } from '../../components/confirm-action';
import { PageState, pageStateForError } from '../../components/page-state/page-state';
import { Phase35PageShell } from '../phase35/components/phase35-page-shell';
import type {
  CompanyAudit,
  CompanyProfile,
  CompanyProfileApi,
  EmployeeSyncSnapshot,
  RotateAgentCredentialsInput,
  RotateArchiveCredentialsInput,
  RotateWeComCredentialsInput,
  UpdateCompanyProfileInput,
  VerifyCompanyInput,
} from './company-profile-api';

export type CompanyWebsitePageProps = {
  api: CompanyProfileApi;
  isSuperAdmin?: boolean;
  onTenantAccessDenied?: () => void;
  onNavigate?: (path: string) => void;
};

const statusText: Record<CompanyProfile['bindingStatus'], string> = {
  pending: '待配置',
  verified: '已验证',
  suspended: '已暂停',
};

const syncStatusText: Record<EmployeeSyncSnapshot['status'], string> = {
  queued: '排队中',
  syncing: '同步中',
  failed: '同步失败',
  completed: '已完成',
};

export function CompanyWebsitePage({ api, isSuperAdmin, onTenantAccessDenied, onNavigate }: CompanyWebsitePageProps) {
  const access = useOptionalDashboardAccess();
  const queryClient = useQueryClient();
  const canView = isSuperAdmin ?? access?.profile?.isSuperAdmin ?? true;
  const [displayName, setDisplayName] = useState('');
  const [verifyCorpId, setVerifyCorpId] = useState('');
  const [employeeSecret, setEmployeeSecret] = useState('');
  const [contactSecret, setContactSecret] = useState('');
  const [callbackToken, setCallbackToken] = useState('');
  const [encodingAESKey, setEncodingAESKey] = useState('');
  const [archiveChatSecret, setArchiveChatSecret] = useState('');
  const [agentId, setAgentId] = useState('');
  const [wxAgentId, setWxAgentId] = useState('');
  const [wxSecret, setWxSecret] = useState('');
  const [operationError, setOperationError] = useState('');
  const requestSequence = useRef(0);

  const profileQuery = useQuery({
    queryKey: ['company-profile'],
    queryFn: () => api.getProfile(),
    enabled: canView,
    retry: false,
  });
  const profile = profileQuery.data;
  const syncQuery = useQuery({
    queryKey: ['company-sync-status'],
    queryFn: () => api.getSyncStatus(),
    enabled: canView && profile !== undefined,
    retry: false,
  });
  const auditsQuery = useQuery({
    queryKey: ['company-audits'],
    queryFn: () => api.listAudits({ page: 1, perPage: 20 }),
    enabled: canView && profile !== undefined,
    retry: false,
  });

  useEffect(() => {
    if (profile === undefined) return;
    setDisplayName(profile.displayName);
    setVerifyCorpId(profile.wxCorpId ?? '');
  }, [profile]);

  const nextRequestId = (kind: string): string => {
    requestSequence.current += 1;
    return `${kind}-${Date.now()}-${requestSequence.current}`;
  };

  const refreshProfile = (nextProfile: CompanyProfile) => {
    queryClient.setQueryData(['company-profile'], nextProfile);
    void queryClient.invalidateQueries({ queryKey: ['company-sync-status'] });
    void queryClient.invalidateQueries({ queryKey: ['company-audits'] });
    setOperationError('');
    setEmployeeSecret('');
    setContactSecret('');
    setCallbackToken('');
    setEncodingAESKey('');
    setArchiveChatSecret('');
    setWxSecret('');
  };

  const showMutationError = (error: unknown, fallback: string) => {
    if (error instanceof ApiError && error.machineCode === 'TENANT_ACCESS_DENIED') {
      onTenantAccessDenied?.();
      return;
    }
    if (error instanceof ApiError && error.machineCode === 'CORP_CONFIGURATION_REQUIRED') {
      onNavigate?.('/company-setting/website');
      setOperationError('请先完成企业配置，当前登录状态已保留。');
      return;
    }
    if (error instanceof ApiError && error.machineCode === 'VERSION_CONFLICT') {
      setOperationError('资料已被其他操作更新，请刷新后重试；当前填写内容已保留。');
      return;
    }
    if (error instanceof ApiError && error.machineCode === 'WECOM_CREDENTIAL_INVALID') {
      setOperationError('企业微信凭据校验失败，请检查后重试。');
      return;
    }
    setOperationError(fallback);
  };

  useEffect(() => {
    if (!(profileQuery.error instanceof ApiError)) return;
    if (profileQuery.error.machineCode === 'TENANT_ACCESS_DENIED') onTenantAccessDenied?.();
    if (profileQuery.error.machineCode === 'CORP_CONFIGURATION_REQUIRED') onNavigate?.('/company-setting/website');
  }, [onNavigate, onTenantAccessDenied, profileQuery.error]);

  const profileMutation = useMutation({
    mutationFn: (input: UpdateCompanyProfileInput) => api.updateProfile(input),
    onSuccess: refreshProfile,
    onError: (error) => showMutationError(error, '企业资料保存失败，请稍后重试。'),
  });
  const weComMutation = useMutation({
    mutationFn: (input: RotateWeComCredentialsInput) => api.rotateWeComCredentials(input),
    onSuccess: refreshProfile,
    onError: (error) => showMutationError(error, '企业微信配置保存失败，请稍后重试。'),
  });
  const agentMutation = useMutation({
    mutationFn: (input: RotateAgentCredentialsInput) => api.rotateAgentCredentials(input),
    onSuccess: refreshProfile,
    onError: (error) => showMutationError(error, '应用配置保存失败，请稍后重试。'),
  });
  const archiveMutation = useMutation({
    mutationFn: (input: RotateArchiveCredentialsInput) => api.rotateArchiveCredentials(input),
    onSuccess: refreshProfile,
    onError: (error) => showMutationError(error, '会话存档配置保存失败，请稍后重试。'),
  });
  const verifyMutation = useMutation({
    mutationFn: (input: VerifyCompanyInput) => api.verify(input),
    onSuccess: refreshProfile,
    onError: (error) => showMutationError(error, '企业微信验证失败，请稍后重试。'),
  });
  const syncMutation = useMutation({
    mutationFn: () => api.startEmployeeSync(),
    onSuccess: (result) => {
      queryClient.setQueryData(['company-sync-status'], {
        status: result.status,
        cursor: result.cursor,
        departments: result.departmentsCreated + result.departmentsUpdated,
        employees: result.employeesCreated + result.employeesUpdated,
        startedAt: result.startedAt,
        finishedAt: result.finishedAt,
        errorCode: result.errorCode,
      });
      void queryClient.invalidateQueries({ queryKey: ['company-sync-status'] });
      void queryClient.invalidateQueries({ queryKey: ['company-audits'] });
      setOperationError('');
    },
    onError: (error) => showMutationError(error, '员工同步任务未能排队，请稍后重试。'),
  });

  if (!canView) {
    return (
      <Phase35PageShell title="唯一企业资料" description="仅企业超级管理员可查看和维护企业配置。">
        <PageState state="forbidden" title="暂无权限查看企业资料" description="只有企业超级管理员可以查看和维护企业配置。" />
      </Phase35PageShell>
    );
  }
  if (profileQuery.isPending) {
    return <Phase35PageShell title="唯一企业资料" description="读取服务端绑定的企业资料。"><PageState state="loading" /></Phase35PageShell>;
  }
  if (profileQuery.isError) {
    return (
      <Phase35PageShell title="唯一企业资料" description="读取服务端绑定的企业资料。">
        <PageState
          state={pageStateForError(profileQuery.error)}
          title={profileQuery.error instanceof ApiError && profileQuery.error.machineCode === 'CORP_CONFIGURATION_REQUIRED' ? '需要完成企业配置' : undefined}
          description={profileQuery.error instanceof ApiError && profileQuery.error.machineCode === 'CORP_CONFIGURATION_REQUIRED' ? '请完成企业配置后再访问其他 Dashboard 功能。' : undefined}
          onRetry={() => void profileQuery.refetch()}
        />
      </Phase35PageShell>
    );
  }
  if (profile === undefined) {
    return <Phase35PageShell title="唯一企业资料"><PageState state="empty" title="暂无企业资料" description="当前租户还没有可维护的企业绑定。" /></Phase35PageShell>;
  }

  const expectedVersion = profile.bindingVersion;
  const hasProfileChanges = displayName.trim() !== profile.displayName;
  const hasWeComChanges = [employeeSecret, contactSecret, callbackToken, encodingAESKey].some((value) => value !== '');
  const hasAgentChanges = agentId !== '' || wxAgentId !== '' || wxSecret !== '';
  const hasArchiveChanges = archiveChatSecret !== '';
  const syncStatus = syncQuery.data;
  const syncIsRunning = syncStatus?.status === 'queued' || syncStatus?.status === 'syncing' || syncMutation.isPending;

  return (
    <Phase35PageShell title="唯一企业资料" description="企业绑定由服务端确定，当前页面不提供新建、切换或删除企业。">
      <div className="phase35-page company-profile-page">
        {operationError && <p className="phase35-notice-error" role="alert">{operationError}</p>}

        <section className="phase35-card company-profile-card" aria-labelledby="company-identity-heading">
          <header className="company-profile-card-header">
            <div>
              <p className="company-profile-eyebrow">企业身份</p>
              <h2 id="company-identity-heading">唯一企业绑定</h2>
              <p>企业、租户和当前操作者均由服务端认证事实确定。</p>
            </div>
            <span className={`company-binding-status company-binding-status-${profile.bindingStatus}`}>{statusText[profile.bindingStatus]}</span>
          </header>
          <div className="company-profile-facts">
            <div><span>企业名称</span><strong>{profile.authoritativeCorpName || profile.displayName || '待验证'}</strong></div>
            <div><span>企业微信 CorpID</span><strong>{profile.wxCorpId || '待验证'}</strong></div>
            <div><span>绑定版本</span><strong>{profile.bindingVersion}</strong></div>
            <div><span>资料状态</span><strong>{statusText[profile.bindingStatus]}</strong></div>
          </div>
          <div className="company-profile-form-row">
            <label>展示名称<input value={displayName} onChange={(event) => setDisplayName(event.target.value)} maxLength={255} /></label>
            <ConfirmAction
              title="确认保存企业资料？"
              description={`变更摘要：展示名称改为“${displayName.trim() || '（空）'}”，版本 ${expectedVersion}。`}
              onConfirm={() => profileMutation.mutate({ displayName: displayName.trim(), expectedVersion, requestId: nextRequestId('profile') })}
            >
              <button type="button" disabled={!hasProfileChanges || profileMutation.isPending}>保存企业资料</button>
            </ConfirmAction>
          </div>
        </section>

        <section className="phase35-card company-profile-card" aria-labelledby="company-wecom-heading">
          <header className="company-profile-card-header">
            <div><p className="company-profile-eyebrow">企业微信与应用</p><h2 id="company-wecom-heading">凭据配置</h2><p>密钥只在输入框中接收并由服务端加密保存，页面不会回显明文或密文。</p></div>
          </header>
          <div className="company-credential-status-grid" aria-label="凭据配置状态">
            <CredentialStatus label="企业微信凭据" configured={profile.credentials.wecom.configured} />
            <CredentialStatus label="应用凭据" configured={profile.credentials.agent.configured} />
            <CredentialStatus label="会话存档凭据" configured={profile.credentials.archive.configured} />
          </div>
          <div className="company-profile-form-grid">
            <SecretField label="员工密钥" value={employeeSecret} onChange={setEmployeeSecret} />
            <SecretField label="客户联系密钥" value={contactSecret} onChange={setContactSecret} />
            <SecretField label="回调 Token" value={callbackToken} onChange={setCallbackToken} />
            <SecretField label="EncodingAESKey" value={encodingAESKey} onChange={setEncodingAESKey} />
          </div>
          <p className="company-profile-help">留空表示不修改。当前页面只显示“已配置/未配置”，不会填入或展示任何 Secret。</p>
          <ConfirmAction
            title="确认保存企业微信凭据？"
            description={`变更摘要：保存已填写的企业微信字段，空字段不修改；版本 ${expectedVersion}。`}
            onConfirm={() => weComMutation.mutate({ employeeSecret, contactSecret, callbackToken, encodingAESKey, expectedVersion, requestId: nextRequestId('wecom') })}
          >
            <button type="button" disabled={!hasWeComChanges || weComMutation.isPending}>保存企业微信凭据</button>
          </ConfirmAction>
          <div className="company-profile-form-grid company-profile-agent-fields">
            <label>AgentID<input inputMode="numeric" value={agentId} onChange={(event) => setAgentId(event.target.value.replace(/[^0-9]/g, ''))} /></label>
            <label>应用 AgentID<input value={wxAgentId} onChange={(event) => setWxAgentId(event.target.value)} /></label>
            <SecretField label="应用 Secret" value={wxSecret} onChange={setWxSecret} />
          </div>
          <ConfirmAction
            title="确认保存应用凭据？"
            description={`变更摘要：更新应用标识或 Secret，空 Secret 不修改；版本 ${expectedVersion}。`}
              onConfirm={() => {
                const input: RotateAgentCredentialsInput = { wxAgentId, wxSecret, expectedVersion, requestId: nextRequestId('agent') };
                if (agentId !== '') input.agentId = Number(agentId);
                agentMutation.mutate(input);
              }}
          >
            <button type="button" disabled={!hasAgentChanges || agentMutation.isPending}>保存应用凭据</button>
          </ConfirmAction>
          <div className="company-profile-archive-row">
            <SecretField label="会话存档 Secret" value={archiveChatSecret} onChange={setArchiveChatSecret} />
            <ConfirmAction
              title="确认保存会话存档配置？"
              description={`变更摘要：更新会话存档 Secret，空 Secret 不修改；版本 ${expectedVersion}。`}
              onConfirm={() => archiveMutation.mutate({ chatSecret: archiveChatSecret, expectedVersion, requestId: nextRequestId('archive') })}
            >
              <button type="button" disabled={!hasArchiveChanges || archiveMutation.isPending}>保存会话存档</button>
            </ConfirmAction>
          </div>
        </section>

        <section className="phase35-card company-profile-card" aria-labelledby="company-verify-heading">
          <header className="company-profile-card-header">
            <div><p className="company-profile-eyebrow">绑定验证</p><h2 id="company-verify-heading">验证企业微信</h2><p>验证成功后，企业微信 CorpID 和名称以服务端探测到的权威事实为准。</p></div>
          </header>
          <div className="company-profile-form-row">
            <label>待验证 CorpID<input value={verifyCorpId} onChange={(event) => setVerifyCorpId(event.target.value)} disabled={profile.bindingStatus === 'suspended'} /></label>
            <ConfirmAction
              title="确认验证企业微信？"
              description={`变更摘要：验证 CorpID “${verifyCorpId.trim() || '（空）'}”，版本 ${expectedVersion}。`}
              onConfirm={() => verifyMutation.mutate({ wxCorpId: verifyCorpId.trim(), expectedVersion, requestId: nextRequestId('verify') })}
            >
              <button type="button" disabled={profile.bindingStatus !== 'pending' || verifyCorpId.trim() === '' || verifyMutation.isPending}>验证企业微信</button>
            </ConfirmAction>
          </div>
        </section>

        <section className="phase35-card company-profile-card" aria-labelledby="company-sync-heading">
          <header className="company-profile-card-header">
            <div><p className="company-profile-eyebrow">员工同步</p><h2 id="company-sync-heading">从企业微信同步员工</h2><p>员工可从企业微信同步到 MoChat。此操作不会创建 Dashboard 登录，也不会授予角色或页面权限。</p></div>
            {syncStatus && <span className={`company-sync-status company-sync-status-${syncStatus.status}`}>{syncStatusText[syncStatus.status]}</span>}
          </header>
          <div className="company-sync-summary">
            <span>部门：{syncStatus?.departments ?? 0}</span>
            <span>员工：{syncStatus?.employees ?? 0}</span>
            {syncStatus?.errorCode && <span role="alert">错误：{syncStatus.errorCode}</span>}
          </div>
          {syncQuery.isError && <div className="company-inline-error"><span>同步状态读取失败。</span><button type="button" onClick={() => void syncQuery.refetch()}>重试</button></div>}
          <div className="company-profile-actions">
            <ConfirmAction
              title="确认发起员工同步？"
              description="变更摘要：提交当前唯一企业的员工同步任务；不会创建登录身份或授予权限。"
              onConfirm={() => syncMutation.mutate()}
            >
              <button type="button" disabled={profile.bindingStatus !== 'verified' || syncIsRunning || syncMutation.isPending}>开始员工同步</button>
            </ConfirmAction>
            <button type="button" onClick={() => void syncQuery.refetch()}>刷新同步状态</button>
          </div>
        </section>

        <section className="phase35-card company-profile-card" aria-labelledby="company-audit-heading">
          <header className="company-profile-card-header"><div><p className="company-profile-eyebrow">变更记录</p><h2 id="company-audit-heading">企业配置审计</h2><p>仅展示变更动作和版本事实，不包含 Secret、密文或凭据正文。</p></div></header>
          {auditsQuery.isPending ? <PageState state="loading" /> : auditsQuery.isError ? <div className="company-inline-error"><span>审计记录读取失败。</span><button type="button" onClick={() => void auditsQuery.refetch()}>重试</button></div> : (auditsQuery.data?.items.length ?? 0) === 0 ? <PageState state="empty" title="暂无配置变更" description="完成一次企业配置操作后，审计记录会显示在这里。" /> : <AuditList items={auditsQuery.data?.items ?? []} />}
        </section>
      </div>
    </Phase35PageShell>
  );
}

function CredentialStatus({ label, configured }: { label: string; configured: boolean }) {
  return <div className="company-credential-status"><span>{label}</span><strong>{configured ? '已配置（加密保存）' : '未配置'}</strong></div>;
}

function SecretField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return <label>{label}<input type="password" autoComplete="new-password" value={value} onChange={(event) => onChange(event.target.value)} placeholder="留空表示不修改" /></label>;
}

function AuditList({ items }: { items: CompanyAudit[] }) {
  return <div className="company-audit-list">{items.map((item) => <article key={item.id}><strong>{item.action}</strong><span>{item.changedFields?.join('、') || '配置变更'}</span><time dateTime={item.createdAt}>{item.createdAt || '—'}</time></article>)}</div>;
}
