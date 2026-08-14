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
  ConfigureApplicationInput,
  EmployeeSyncSnapshot,
  RotateArchiveCredentialsInput,
  UpdateCompanyProfileInput,
  VerifyCompanyInput,
} from './company-profile-api';
import type { ProviderStatusApi } from '../provider-status/provider-status-api';
import { ProviderStatusPage } from '../provider-status/provider-status-page';

export type CompanyWebsitePageProps = {
  api: CompanyProfileApi;
  isSuperAdmin?: boolean;
  onTenantAccessDenied?: () => void;
  onNavigate?: (path: string) => void;
  providerStatusApi?: ProviderStatusApi;
};

const statusText: Record<CompanyProfile['bindingStatus'], string> = {
  pending: '待配置',
  verified: '已验证',
  suspended: '已暂停',
};

const syncStatusText: Record<EmployeeSyncSnapshot['status'], string> = {
  idle: '尚未同步',
  queued: '排队中',
  syncing: '同步中',
  failed: '同步失败',
  completed: '已完成',
};

type FeedbackScope = 'profile' | 'application' | 'callback' | 'archive' | 'verify' | 'sync';
type OperationFeedback = { scope: FeedbackScope; tone: 'error' | 'notice'; message: string };

export function CompanyWebsitePage({ api, isSuperAdmin, onTenantAccessDenied, onNavigate, providerStatusApi }: CompanyWebsitePageProps) {
  const access = useOptionalDashboardAccess();
  const queryClient = useQueryClient();
  const canView = isSuperAdmin ?? access?.profile?.isSuperAdmin ?? false;
  const [displayName, setDisplayName] = useState('');
  const [verifyCorpId, setVerifyCorpId] = useState('');
  const [archiveChatSecret, setArchiveChatSecret] = useState('');
	const [archiveRSAPublicKey, setArchiveRSAPublicKey] = useState('');
	const [archiveRSAPrivateKey, setArchiveRSAPrivateKey] = useState('');
  const [wxAgentId, setWxAgentId] = useState('');
  const [wxSecret, setWxSecret] = useState('');
  const [operationFeedback, setOperationFeedback] = useState<OperationFeedback | null>(null);
  const requestSequence = useRef(0);
	const applicationInputRef = useRef<ConfigureApplicationInput | null>(null);
  const archiveInputRef = useRef<RotateArchiveCredentialsInput | null>(null);

  const profileQuery = useQuery({
    queryKey: ['company-profile'],
    queryFn: () => api.getProfile(),
    enabled: canView,
    retry: false,
  });
  const profile = profileQuery.data;
	const callbackQuery = useQuery({
	  queryKey: ['company-callback-configuration'],
	  queryFn: () => api.getCallbackConfiguration(),
	  enabled: canView && profile !== undefined,
	  retry: false,
	});
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
    setWxAgentId(profile.applicationAgentId ?? '');
  }, [profile]);

  const nextRequestId = (kind: string): string => {
    requestSequence.current += 1;
    return `${kind}-${Date.now()}-${requestSequence.current}`;
  };

  const refreshProfile = (nextProfile: CompanyProfile, scope: FeedbackScope, message: string) => {
    queryClient.setQueryData(['company-profile'], nextProfile);
    void queryClient.invalidateQueries({ queryKey: ['company-sync-status'] });
    void queryClient.invalidateQueries({ queryKey: ['company-audits'] });
    setOperationFeedback({ scope, tone: 'notice', message });
    setArchiveChatSecret('');
	setArchiveRSAPublicKey('');
	setArchiveRSAPrivateKey('');
    setWxSecret('');
  };

  const showMutationError = (error: unknown, fallback: string, scope: FeedbackScope) => {
    if (error instanceof ApiError && error.machineCode === 'TENANT_ACCESS_DENIED') {
      onTenantAccessDenied?.();
      return;
    }
    if (error instanceof ApiError && error.machineCode === 'CORP_CONFIGURATION_REQUIRED') {
      onNavigate?.('/company-setting/website');
      setOperationFeedback({ scope, tone: 'error', message: '请先完成企业配置，当前登录状态已保留。' });
      return;
    }
    if (error instanceof ApiError && error.machineCode === 'VERSION_CONFLICT') {
      setOperationFeedback({ scope, tone: 'error', message: '资料已被其他操作更新，请刷新后重试；当前填写内容已保留。' });
      return;
    }
    if (error instanceof ApiError && error.machineCode === 'WECOM_CREDENTIAL_INVALID') {
      setOperationFeedback({ scope, tone: 'error', message: '企业微信凭据校验失败，请检查后重试。' });
      return;
    }
    if (error instanceof ApiError && error.machineCode === 'INVALID_REQUEST') {
      setOperationFeedback({ scope, tone: 'error', message: scope === 'sync' ? '同步请求格式不正确，任务未创建，请刷新页面后重试。' : '提交内容格式不正确，请检查后重试。' });
      return;
    }
    setOperationFeedback({ scope, tone: 'error', message: fallback });
  };

  useEffect(() => {
    if (!(profileQuery.error instanceof ApiError)) return;
    if (profileQuery.error.machineCode === 'TENANT_ACCESS_DENIED') onTenantAccessDenied?.();
    if (profileQuery.error.machineCode === 'CORP_CONFIGURATION_REQUIRED') onNavigate?.('/company-setting/website');
  }, [onNavigate, onTenantAccessDenied, profileQuery.error]);

  const profileMutation = useMutation({
    mutationFn: (input: UpdateCompanyProfileInput) => api.updateProfile(input),
    onSuccess: (nextProfile) => refreshProfile(nextProfile, 'profile', '企业资料已保存。'),
    onError: (error) => showMutationError(error, '企业资料保存失败，请稍后重试。', 'profile'),
  });
	const applicationMutation = useMutation({
    mutationFn: () => {
	  const input = applicationInputRef.current;
	  if (input === null) throw new Error('company application input is unavailable');
	  return api.configureApplication(input);
    },
    onSuccess: (nextProfile) => {
	  applicationInputRef.current = null;
      refreshProfile(nextProfile, 'application', '应用配置已保存。');
	  void queryClient.invalidateQueries({ queryKey: ['company-callback-configuration'] });
    },
    onError: (error) => {
	  applicationInputRef.current = null;
	  showMutationError(error, '应用配置保存失败，请稍后重试。', 'application');
    },
  });
	const callbackMutation = useMutation({
	  mutationFn: (input: { expectedVersion: number; requestId: string }) => api.regenerateCallbackConfiguration(input),
	  onSuccess: (configuration) => {
		queryClient.setQueryData(['company-callback-configuration'], configuration);
		queryClient.setQueryData<CompanyProfile>(['company-profile'], (current) => current === undefined ? current : { ...current, bindingVersion: configuration.bindingVersion });
		void queryClient.invalidateQueries({ queryKey: ['company-audits'] });
		setOperationFeedback({ scope: 'callback', tone: 'notice', message: '回调 Token 和 EncodingAESKey 已重新生成，请及时更新企业微信后台。' });
	  },
	  onError: (error) => showMutationError(error, '回调配置重新生成失败，请稍后重试。', 'callback'),
	});
  const archiveMutation = useMutation({
    mutationFn: () => {
      const input = archiveInputRef.current;
      if (input === null) throw new Error('company archive input is unavailable');
      return api.rotateArchiveCredentials(input);
    },
    onSuccess: (nextProfile) => {
      archiveInputRef.current = null;
      refreshProfile(nextProfile, 'archive', '会话存档配置已保存。');
    },
    onError: (error) => {
      archiveInputRef.current = null;
      showMutationError(error, '会话存档配置保存失败，请稍后重试。', 'archive');
    },
  });
  const verifyMutation = useMutation({
    mutationFn: (input: VerifyCompanyInput) => api.verify(input),
    onSuccess: (nextProfile) => refreshProfile(nextProfile, 'verify', '企业微信验证成功。'),
    onError: (error) => showMutationError(error, '企业微信验证失败，请稍后重试。', 'verify'),
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
      setOperationFeedback({ scope: 'sync', tone: 'notice', message: '员工同步任务已进入队列。' });
    },
    onError: (error) => showMutationError(error, '同步服务暂时不可用，任务未创建，请稍后重试。', 'sync'),
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
  const applicationConfigured = profile.credentials.wecom.configured && profile.credentials.agent.configured;
  const hasProfileChanges = displayName.trim() !== '' && displayName.trim() !== profile.displayName;
	const hasApplicationChanges = wxAgentId.trim() !== '' && wxSecret.trim() !== '';
	const hasArchiveChanges = archiveChatSecret.trim() !== '' || archiveRSAPublicKey.trim() !== '' || archiveRSAPrivateKey.trim() !== '';
	const archiveKeyPairComplete = (archiveRSAPublicKey.trim() === '') === (archiveRSAPrivateKey.trim() === '');
	const callbackConfiguration = callbackQuery.data;
  const syncStatus = syncQuery.data;
  const syncIsRunning = syncStatus?.status === 'queued' || syncStatus?.status === 'syncing' || syncMutation.isPending;
  const syncConfigurationRequired =
    syncQuery.isError &&
    syncQuery.error instanceof ApiError &&
    syncQuery.error.machineCode === 'CORP_CONFIGURATION_REQUIRED';

  return (
    <Phase35PageShell title="唯一企业资料" description="企业绑定由服务端确定，当前页面不提供新建、切换或删除企业。">
      <div className="phase35-page company-profile-page">
        {providerStatusApi === undefined ? null : <ProviderStatusPage api={providerStatusApi} isSuperAdmin={canView} />}
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
          <ScopedFeedback feedback={operationFeedback} scope="profile" />
        </section>

        <section className="phase35-card company-profile-card" aria-labelledby="company-application-heading">
		  <header className="company-profile-card-header">
			<div>
			  <p className="company-profile-eyebrow">企业微信应用</p>
			  <h2 id="company-application-heading">应用配置</h2>
			  <p>只需填写一次应用 Secret，服务端会在同一事务中用于通讯录同步、客户同步和应用消息。</p>
			</div>
			<CredentialStatus label="应用配置" configured={applicationConfigured} />
		  </header>
		  <div className="company-profile-form-grid company-profile-agent-fields">
			<label>应用 AgentID<input inputMode="numeric" value={wxAgentId} onChange={(event) => setWxAgentId(event.target.value.replace(/[^0-9]/g, ''))} placeholder="例如：1000010" /></label>
			<SecretField
			  label="应用 Secret"
			  value={wxSecret}
			  onChange={setWxSecret}
			  placeholder={applicationConfigured ? '已加密保存；输入新 Secret 可替换' : '请输入应用 Secret'}
			/>
		  </div>
		  <p className="company-profile-help">
			{applicationConfigured
			  ? '应用 Secret 已加密保存，出于安全原因不会回显。'
			  : 'AgentID 和 Secret 均为必填；Secret 提交后由服务端加密保存。'}
		  </p>
		  <ConfirmAction
			title="确认保存应用配置？"
			description={`变更摘要：更新应用 AgentID，并将同一 Secret 用于通讯录、客户联系和应用消息；版本 ${expectedVersion}。`}
			onConfirm={() => {
			  applicationInputRef.current = {
				wxAgentId: wxAgentId.trim(), secret: wxSecret.trim(), expectedVersion, requestId: nextRequestId('application'),
			  };
			  applicationMutation.mutate();
			}}
		  >
			<button type="button" disabled={!hasApplicationChanges || applicationMutation.isPending}>保存应用配置</button>
		  </ConfirmAction>
		  <ScopedFeedback feedback={operationFeedback} scope="application" />
		</section>

		<section className="phase35-card company-profile-card" aria-labelledby="company-callback-heading">
		  <header className="company-profile-card-header">
			<div>
			  <p className="company-profile-eyebrow">设置 API 接收</p>
			  <h2 id="company-callback-heading">回调配置</h2>
			  <p>以下值由系统生成，请复制到企业微信应用的“设置 API 接收”中。</p>
			</div>
			{callbackConfiguration && <CredentialStatus label="回调配置" configured={callbackConfiguration.configured} />}
		  </header>
		  {callbackQuery.isPending ? <PageState state="loading" /> : callbackQuery.isError ? (
			<div className="company-inline-error"><span>回调配置读取失败。</span><button type="button" onClick={() => void callbackQuery.refetch()}>重试</button></div>
		  ) : callbackConfiguration === undefined ? <PageState state="empty" title="尚未生成回调配置" /> : (
			<div className="company-readonly-config" aria-label="回调配置（系统生成，只读）">
			  <ReadonlyCopyField label="回调 URL" value={callbackConfiguration.callbackUrl} onCopied={() => setOperationFeedback({ scope: 'callback', tone: 'notice', message: '回调 URL 已复制。' })} />
			  <ReadonlyCopyField label="Token" value={callbackConfiguration.token} onCopied={() => setOperationFeedback({ scope: 'callback', tone: 'notice', message: 'Token 已复制。' })} />
			  <ReadonlyCopyField label="EncodingAESKey" value={callbackConfiguration.encodingAESKey} onCopied={() => setOperationFeedback({ scope: 'callback', tone: 'notice', message: 'EncodingAESKey 已复制。' })} />
			</div>
		  )}
		  <ConfirmAction
			title="确认重新生成回调配置？"
			description="重新生成后，旧 Token 和 EncodingAESKey 立即失效，必须同步更新企业微信后台。"
			onConfirm={() => callbackMutation.mutate({ expectedVersion, requestId: nextRequestId('callback') })}
		  >
			<button type="button" disabled={callbackQuery.isPending || callbackMutation.isPending}>重新生成回调配置</button>
		  </ConfirmAction>
		  <ScopedFeedback feedback={operationFeedback} scope="callback" />
		</section>

		<section className="phase35-card company-profile-card" aria-labelledby="company-archive-heading">
		  <header className="company-profile-card-header">
			<div>
			  <p className="company-profile-eyebrow">会话内容存档</p>
			  <h2 id="company-archive-heading">会话存档配置</h2>
			  <p>会话存档需要 Secret 与匹配的 RSA 公私钥；服务端验证密钥对后加密保存。</p>
			</div>
			<CredentialStatus label="会话存档" configured={profile.credentials.archive.configured} />
		  </header>
		  <div className="company-profile-archive-fields">
			<SecretField label="会话存档 Secret" value={archiveChatSecret} onChange={setArchiveChatSecret} />
			<PEMField label="会话存档 RSA 公钥" value={archiveRSAPublicKey} onChange={setArchiveRSAPublicKey} placeholder="-----BEGIN PUBLIC KEY-----" />
			<PEMField label="会话存档 RSA 私钥" value={archiveRSAPrivateKey} onChange={setArchiveRSAPrivateKey} placeholder="-----BEGIN RSA PRIVATE KEY-----" />
		  </div>
		  <p className="company-profile-help">更新 RSA 密钥时，公钥和私钥必须同时填写且相互匹配；私钥不会回显。</p>
		  <ConfirmAction
			title="确认保存会话存档配置？"
			description={`变更摘要：更新已填写的会话存档 Secret/RSA 密钥对；版本 ${expectedVersion}。`}
			onConfirm={() => {
			  const input: RotateArchiveCredentialsInput = { expectedVersion, requestId: nextRequestId('archive') };
			  if (archiveChatSecret.trim() !== '') input.chatSecret = archiveChatSecret.trim();
			  if (archiveRSAPublicKey.trim() !== '') input.rsaPublicKey = archiveRSAPublicKey.trim();
			  if (archiveRSAPrivateKey.trim() !== '') input.rsaPrivateKey = archiveRSAPrivateKey.trim();
			  archiveInputRef.current = input;
			  archiveMutation.mutate();
			}}
		  >
			<button type="button" disabled={!hasArchiveChanges || !archiveKeyPairComplete || archiveMutation.isPending}>保存会话存档</button>
		  </ConfirmAction>
		  <ScopedFeedback feedback={operationFeedback} scope="archive" />
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
          <ScopedFeedback feedback={operationFeedback} scope="verify" />
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
          {syncConfigurationRequired ? (
            <p className="company-profile-help">完成企业微信验证后即可同步员工</p>
          ) : syncQuery.isError ? (
            <div className="company-inline-error"><span>同步状态读取失败。</span><button type="button" onClick={() => void syncQuery.refetch()}>重试</button></div>
          ) : null}
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
          <ScopedFeedback feedback={operationFeedback} scope="sync" />
        </section>

        <section className="phase35-card company-profile-card" aria-labelledby="company-audit-heading">
          <header className="company-profile-card-header"><div><p className="company-profile-eyebrow">变更记录</p><h2 id="company-audit-heading">企业配置审计</h2><p>仅展示变更动作和版本事实，不包含 Secret、密文或凭据正文。</p></div></header>
          {auditsQuery.isPending ? <PageState state="loading" /> : auditsQuery.isError ? <div className="company-inline-error"><span>审计记录读取失败。</span><button type="button" onClick={() => void auditsQuery.refetch()}>重试</button></div> : (auditsQuery.data?.items.length ?? 0) === 0 ? <PageState state="empty" title="暂无配置变更" description="完成一次企业配置操作后，审计记录会显示在这里。" /> : <AuditList items={auditsQuery.data?.items ?? []} />}
        </section>
      </div>
    </Phase35PageShell>
  );
}

function ScopedFeedback({ feedback, scope }: { feedback: OperationFeedback | null; scope: FeedbackScope }) {
  if (feedback === null || feedback.scope !== scope) return null;
  return feedback.tone === 'error' ? (
    <p className="phase35-notice-error company-profile-inline-feedback" role="alert">{feedback.message}</p>
  ) : (
    <p className="company-profile-notice company-profile-inline-feedback" role="status">{feedback.message}</p>
  );
}

function CredentialStatus({ label, configured }: { label: string; configured: boolean }) {
  return <div className="company-credential-status"><span>{label}</span><strong>{configured ? '已配置（加密保存）' : '未配置'}</strong></div>;
}

function SecretField({
  label,
  value,
  onChange,
  placeholder = '留空表示不修改',
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}) {
  return (
    <label>
      {label}
      <input
        type="password"
        autoComplete="new-password"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
      />
    </label>
  );
}

function PEMField({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder: string }) {
	return <label>{label}<textarea autoComplete="off" spellCheck={false} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} /></label>;
}

function ReadonlyCopyField({ label, value, onCopied }: { label: string; value: string; onCopied: () => void }) {
  const copy = async () => {
    if (value === '') return;
    if (navigator.clipboard !== undefined) {
      await navigator.clipboard.writeText(value);
      onCopied();
      return;
    }
    const input = document.createElement('textarea');
    input.value = value;
    input.setAttribute('readonly', '');
    input.style.position = 'fixed';
    input.style.opacity = '0';
    document.body.append(input);
    input.select();
    const copied = document.execCommand('copy');
    input.remove();
    if (copied) onCopied();
  };
	return (
		<label>
			<span>{label}</span>
			<div className="company-readonly-field">
				<input value={value || '尚未生成'} readOnly aria-label={label} />
				<button type="button" onClick={() => void copy()} disabled={value === ''} aria-label={`复制${label}`}>复制</button>
			</div>
		</label>
	);
}

function AuditList({ items }: { items: CompanyAudit[] }) {
  return <div className="company-audit-list">{items.map((item) => <article key={item.id}><strong>{item.action}</strong><span>{item.changedFields?.join('、') || '配置变更'}</span><time dateTime={item.createdAt}>{item.createdAt || '—'}</time></article>)}</div>;
}
