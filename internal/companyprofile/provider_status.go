package companyprofile

import (
	"context"
	"sort"
	"strings"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
	"jiyi/mochat-go/internal/wecomcapability"
)

// ProviderStatusSource binds the global runtime registry to the authenticated
// tenant's company profile. Credential values are never read into the result;
// only the store's non-secret configured/verified facts affect status.
type ProviderStatusSource struct {
	store    Store
	registry *providers.Registry
}

// ArchiveSourceStatusStore is optional so deployments that have not applied
// the archive-source migration can still serve the truthful provider
// baseline. Implementations must scope the lookup to the authenticated
// principal and must never return credential values.
type ArchiveSourceStatusStore interface {
	GetArchiveSourceStatus(context.Context, dashboardprincipal.DashboardPrincipal) (providers.Status, error)
}

type CapabilityOperationReader interface {
	LatestCapabilityOperations(context.Context, dashboardprincipal.DashboardPrincipal, []string) (map[string]wecomcapability.Operation, error)
}

func NewProviderStatusSource(store Store, registry *providers.Registry) *ProviderStatusSource {
	return &ProviderStatusSource{store: store, registry: registry}
}

func (s *ProviderStatusSource) Statuses(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) ([]providers.Status, error) {
	if s == nil || s.store == nil || s.registry == nil {
		return nil, ErrStoreUnavailable
	}
	profile, err := s.store.GetProfile(ctx, principal)
	if err != nil {
		return nil, err
	}
	syncStatus := SyncStatus{Status: "idle"}
	if syncStore, ok := s.store.(SyncStore); ok && eligibleForEmployeeSyncStatus(profile) {
		syncStatus, err = syncStore.GetSyncStatus(ctx, principal)
		if err != nil {
			return nil, err
		}
	}
	operations := map[string]wecomcapability.Operation{}
	if reader, ok := s.store.(CapabilityOperationReader); ok && profile.BindingStatus == "active" && profile.TenantID > 0 && profile.CorpID > 0 {
		operations, err = reader.LatestCapabilityOperations(ctx, principal, wecomcapability.All)
		if err != nil {
			return nil, err
		}
	}
	statuses := s.registry.Snapshot(ctx)
	statuses = replaceStatus(statuses, tenantWeComStandardStatus(profile, findStatus(statuses, "wecom_standard"), syncStatus, operations))
	archiveStatus := tenantWeComArchiveStatus(profile, findStatus(statuses, "wecom_archive"))
	if archiveStore, ok := s.store.(ArchiveSourceStatusStore); ok && profile.BindingStatus == "active" && profile.TenantID > 0 && profile.CorpID > 0 {
		runtimeArchiveStatus, statusErr := archiveStore.GetArchiveSourceStatus(ctx, principal)
		if statusErr != nil {
			return nil, statusErr
		}
		if strings.TrimSpace(runtimeArchiveStatus.Code) != "" {
			archiveStatus = mergeArchiveRuntimeStatus(archiveStatus, runtimeArchiveStatus)
		}
	}
	statuses = replaceStatus(statuses, archiveStatus)
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Kind < statuses[j].Kind })
	return statuses, nil
}

func mergeArchiveRuntimeStatus(fallback, runtime providers.Status) providers.Status {
	if runtime.Kind != "wecom_archive" || runtime.Source != providers.SourceSimulated {
		fallback.Kind = "wecom_archive"
		fallback.State = providers.StateLimited
		fallback.Source = providers.SourceExternal
		fallback.Code = "archive.getchatdata_unimplemented"
		fallback.Reason = "real getchatdata source is not implemented"
		fallback.Action = "connect and verify the real archive source"
		return fallback
	}
	if runtime.State == providers.StateReady {
		runtime.State = providers.StateLimited
		if strings.TrimSpace(runtime.Code) == "" || runtime.Code == "provider.runtime_verified" {
			runtime.Code = "archive.simulation_ready"
		}
	}
	if runtime.Capabilities == nil {
		runtime.Capabilities = fallback.Capabilities
	}
	if runtime.Action == "" {
		runtime.Action = fallback.Action
	}
	return runtime
}

func eligibleForEmployeeSyncStatus(profile Profile) bool {
	return profile.BindingStatus == "active" &&
		profile.Credentials.WeCom.EmployeeConfigured &&
		strings.TrimSpace(profile.WXCorpID) != "" &&
		profile.VerifiedAt != nil
}

func tenantWeComStandardStatusBase(profile Profile, runtime providers.Status, syncStatus SyncStatus) providers.Status {
	if status, unavailable := runtimeUnavailable(runtime, "wecom_standard", "employee_sync"); unavailable {
		return status
	}
	capabilities := append([]string(nil), runtime.Capabilities...)
	if len(capabilities) == 0 {
		capabilities = append([]string(nil), wecomcapability.All...)
	}
	base := providers.Status{
		Kind:         "wecom_standard",
		Source:       providers.SourceExternal,
		Capabilities: capabilities,
	}
	if !hasAnyStandardCredentialEvidence(profile) {
		base.State = providers.StateLimited
		base.Code = "wecom.credentials_missing"
		base.Reason = "当前企业尚未配置可证明的企业微信标准凭据"
		base.Action = "在企业设置中配置并验证企业微信凭据"
		base.Missing = []string{"wecom_credential_evidence"}
		return base
	}
	if strings.TrimSpace(profile.WXCorpID) == "" || profile.VerifiedAt == nil || profile.BindingStatus != "active" {
		base.State = providers.StateLimited
		base.Code = "wecom.runtime_unverified"
		base.Reason = "企业微信凭据已保存，但当前企业尚未完成运行时验证"
		base.Action = "在企业设置中完成企业微信验证"
		return base
	}
	base.State = providers.StateReady
	base.Code = "wecom.runtime_verified"
	base.Action = "企业微信员工同步可用"
	applySyncStatus(&base, syncStatus, credentialVersionForCapability(profile, wecomcapability.EmployeeSync))
	if strings.EqualFold(strings.TrimSpace(syncStatus.Status), "failed") {
		base.State = providers.StateLimited
		base.Code = "wecom.sync_failed"
		base.Reason = "最近一次企业微信员工同步失败，当前状态已降级"
		base.Action = "修复企业微信同步后重试"
		if strings.TrimSpace(base.LastErrorCode) == "" {
			base.LastErrorCode = "wecom.sync_failed"
		}
	}
	return base
}

func hasAnyStandardCredentialEvidence(profile Profile) bool {
	wecom := profile.Credentials.WeCom
	return wecom.EmployeeConfigured || wecom.ContactConfigured ||
		(wecom.CallbackTokenConfigured && wecom.CallbackAESConfigured) ||
		(profile.Credentials.Agent.AgentIDConfigured && profile.Credentials.Agent.AgentSecretConfigured)
}

func tenantWeComStandardStatus(profile Profile, runtime providers.Status, syncStatus SyncStatus, operations map[string]wecomcapability.Operation) providers.Status {
	status := tenantWeComStandardStatusBase(profile, runtime, syncStatus)
	if status.State != providers.StateReady && status.State != providers.StateLimited {
		if status.Kind == "wecom_standard" {
			status.Capabilities = append([]string(nil), wecomcapability.All...)
			status.CapabilityStatuses = unavailableCapabilityStatuses(status.Code)
		}
		return status
	}
	status.Capabilities = append([]string(nil), wecomcapability.All...)
	verified := profile.BindingStatus == "active" && strings.TrimSpace(profile.WXCorpID) != "" && profile.VerifiedAt != nil
	status.CapabilityStatuses = standardCapabilityStatuses(profile, runtime, syncStatus, operations, verified)
	readyCapability := false
	for _, capabilityStatus := range status.CapabilityStatuses {
		if capabilityStatus.State == providers.StateReady {
			readyCapability = true
			break
		}
	}
	if readyCapability {
		status.State = providers.StateReady
		status.Code = "wecom.capability_ready"
		status.Reason = "至少一个企业微信标准能力已有当前凭据版本的成功证据"
		status.Action = "继续查看各项能力的独立状态"
	} else if status.State == providers.StateReady {
		status.State = providers.StateLimited
		status.Code = "wecom.capabilities_pending"
		status.Reason = "企业微信运行时已配置，但尚无标准能力的当前成功证据"
		status.Action = "执行对应能力的真实操作并等待结果"
	}
	return status
}

func unavailableCapabilityStatuses(code string) []providers.CapabilityStatus {
	statuses := make([]providers.CapabilityStatus, 0, len(wecomcapability.All))
	for _, capability := range wecomcapability.All {
		statuses = append(statuses, providers.CapabilityStatus{
			Capability: capability, State: providers.StateUnavailable, Source: providers.SourceExternal,
			Code: code, Action: "启用 WeCom runtime component",
		})
	}
	return statuses
}

func standardCapabilityStatuses(profile Profile, runtime providers.Status, syncStatus SyncStatus, operations map[string]wecomcapability.Operation, verified bool) []providers.CapabilityStatus {
	statuses := make([]providers.CapabilityStatus, 0, len(wecomcapability.All))
	for _, capability := range wecomcapability.All {
		item := providers.CapabilityStatus{
			Capability: capability, State: providers.StateLimited, Source: providers.SourceExternal,
			Code: "wecom.capability_operation_pending", Action: "执行该 WeCom capability 的真实操作并等待结果",
		}
		if !verified {
			item.Code = "wecom.runtime_unverified"
			item.Reason = "企业凭据已保存但尚未完成运行时验证"
			item.Action = "完成企业 WeCom 验证"
			statuses = append(statuses, item)
			continue
		}
		if configured, code, reason, action, missing := capabilityCredentialGate(profile, runtime, capability); !configured {
			item.Code = code
			item.Reason = reason
			item.Action = action
			item.Missing = missing
			statuses = append(statuses, item)
			continue
		}
		operation, hasOperation := operations[capability]
		hasCurrentOperation := hasOperation && wecomcapability.IsCurrentOperationEvidence(operation, profile.TenantID, profile.CorpID, capability, credentialVersionForCapability(profile, capability))
		if hasOperation && !hasCurrentOperation {
			item.Code = "wecom.capability_evidence_invalid"
			item.Reason = "最近的能力操作证据缺少当前租户、凭据版本或外部请求证明"
			item.Action = "重新执行该能力并等待完整结果"
		}
		if capability == wecomcapability.EmployeeSync {
			applyCapabilitySyncStatus(&item, syncStatus, credentialVersionForCapability(profile, wecomcapability.EmployeeSync))
		} else if capability == wecomcapability.Callback && !hasOperation {
			item.Code = "wecom.callback_evidence_pending"
			item.Reason = "尚未收到当前凭据版本的回调成功证据"
			item.Action = "使用当前凭据版本接收并验证回调"
		} else if hasCurrentOperation {
			applyCapabilityOperation(&item, operation)
		}
		statuses = append(statuses, item)
	}
	return statuses
}

func capabilityCredentialGate(profile Profile, runtime providers.Status, capability string) (bool, string, string, string, []string) {
	wecom := profile.Credentials.WeCom
	switch capability {
	case wecomcapability.EmployeeSync, wecomcapability.DepartmentSync:
		if !wecom.EmployeeConfigured {
			return false, "wecom.employee_credentials_missing", "员工与部门同步所需企业微信员工凭据未配置", "配置并验证企业微信员工凭据", []string{"employee_credential"}
		}
	case wecomcapability.ExternalContactSync, wecomcapability.ContactTagSync, wecomcapability.RoomSync,
		wecomcapability.ContactWay, wecomcapability.WelcomeMessage, wecomcapability.ContactTransfer,
		wecomcapability.ContactBatchSend, wecomcapability.RoomBatchSend:
		if !wecom.ContactConfigured {
			return false, "wecom.contact_credentials_missing", "客户与客户群能力所需企业微信客户凭据未配置", "配置并验证企业微信客户凭据", []string{"contact_credential"}
		}
	case wecomcapability.AgentMessage:
		if !profile.Credentials.Agent.AgentIDConfigured || !profile.Credentials.Agent.AgentSecretConfigured {
			return false, "wecom.agent_credentials_missing", "应用消息所需 agent ID 与 secret 未同时配置", "配置并验证应用消息凭据", []string{"agent_id", "agent_secret"}
		}
	case wecomcapability.Callback:
		if !wecom.CallbackTokenConfigured || !wecom.CallbackAESConfigured {
			missing := make([]string, 0, 2)
			if !wecom.CallbackTokenConfigured {
				missing = append(missing, "callback_token")
			}
			if !wecom.CallbackAESConfigured {
				missing = append(missing, "callback_aes_key")
			}
			return false, "wecom.callback_credentials_missing", "回调所需 token 与 AES key 未同时配置", "配置并验证企业微信回调凭据", missing
		}
		if !runtime.CallbackRouteConfigured || !runtime.CallbackWorkerConfigured {
			missing := make([]string, 0, 2)
			if !runtime.CallbackRouteConfigured {
				missing = append(missing, "callback_route")
			}
			if !runtime.CallbackWorkerConfigured {
				missing = append(missing, "callback_worker")
			}
			return false, "wecom.callback_runtime_unconfigured", "回调运行时路由或消费者未启用", "启用企业微信回调路由和消费者", missing
		}
	}
	return true, "", "", "", nil
}

func credentialVersionForCapability(profile Profile, capability string) uint64 {
	switch wecomcapability.CredentialGroupForCapability(capability) {
	case wecomcapability.CredentialGroupEmployee:
		return profile.CredentialGenerations.Employee
	case wecomcapability.CredentialGroupContact:
		return profile.CredentialGenerations.Contact
	case wecomcapability.CredentialGroupAgent:
		return profile.CredentialGenerations.Agent
	case wecomcapability.CredentialGroupCallback:
		return profile.CredentialGenerations.Callback
	default:
		return 0
	}
}

func applyCapabilitySyncStatus(status *providers.CapabilityStatus, syncStatus SyncStatus, expectedVersion uint64) {
	if status == nil {
		return
	}
	status.LastSyncAt = syncStatus.FinishedAt
	if status.LastSyncAt == nil {
		status.LastSyncAt = syncStatus.StartedAt
	}
	normalized := strings.ToLower(strings.TrimSpace(syncStatus.Status))
	if syncStatusVersionStale(normalized, syncStatus.CredentialVersion, expectedVersion) {
		applySyncStaleCapabilityStatus(status)
		return
	}
	switch normalized {
	case "completed", "succeeded", "success":
		if syncStatus.CredentialVersion == 0 || expectedVersion == 0 || syncStatus.CredentialVersion != expectedVersion {
			status.Code = "wecom.sync_stale"
			status.Reason = "最近的员工同步证据属于旧的企业凭据版本"
			status.Action = "使用当前企业凭据再次完成员工同步"
			status.LastErrorCode = "wecom.credential_version_stale"
			break
		}
		status.State = providers.StateReady
		status.Code = "wecom.employee_sync_ready"
		status.Action = "企业 WeCom 员工同步可用"
		status.LastSuccessAt = syncStatus.FinishedAt
	case "queued", "running", "syncing":
		status.Code = "wecom.capability_syncing"
		status.Action = "等待企业 WeCom 员工同步完成"
	case "failed":
		status.Code = "wecom.capability_operation_failed"
		status.Reason = "最近一次员工同步失败"
		status.Action = "修复员工同步后重试"
		status.LastFailureAt = syncStatus.FinishedAt
		if status.LastFailureAt == nil {
			status.LastFailureAt = syncStatus.StartedAt
		}
		status.LastErrorCode = stableSyncErrorCode(syncStatus.ErrorCode)
	}
}

func syncStatusVersionStale(status string, markerVersion, expectedVersion uint64) bool {
	switch status {
	case "stale":
		return true
	case "completed", "succeeded", "success", "queued", "running", "syncing", "failed":
		return markerVersion == 0 || expectedVersion == 0 || markerVersion != expectedVersion
	default:
		return false
	}
}

func applySyncStaleCapabilityStatus(status *providers.CapabilityStatus) {
	status.State = providers.StateLimited
	status.Code = "wecom.sync_stale"
	status.Reason = "同步证据属于旧的企业凭据版本"
	status.Action = "使用当前企业凭据再次完成同步"
	status.LastErrorCode = "wecom.credential_version_stale"
}

func applyCapabilityOperation(status *providers.CapabilityStatus, operation wecomcapability.Operation) {
	if status == nil {
		return
	}
	status.LastSyncAt = operation.FinishedAt
	if status.LastSyncAt == nil {
		status.LastSyncAt = operation.UpdatedAt
	}
	switch strings.ToLower(strings.TrimSpace(operation.Status)) {
	case wecomcapability.OperationSucceeded:
		status.State = providers.StateReady
		status.Code = "wecom.capability_ready"
		status.Action = "该 WeCom capability 最近一次操作成功"
		status.LastSuccessAt = operation.FinishedAt
	case wecomcapability.OperationPending, wecomcapability.OperationClaimed, wecomcapability.OperationSubmitting, wecomcapability.OperationSubmitted, wecomcapability.OperationPolling:
		status.Code = "wecom.capability_syncing"
		status.Action = "等待该 WeCom capability 操作完成"
	case wecomcapability.OperationPartialFailed:
		status.Code = "wecom.capability_operation_partial"
		status.Reason = "最近一次 WeCom 操作部分成功"
		status.Action = "查看失败明细并重试失败目标"
		status.LastFailureAt = operation.FinishedAt
		status.LastErrorCode = stableCapabilityErrorCode(operation.ErrorCode)
	case wecomcapability.OperationFailed:
		status.Code = "wecom.capability_operation_failed"
		status.Reason = "最近一次 WeCom 操作失败"
		status.Action = "修复 WeCom 权限或凭据后重试"
		status.LastFailureAt = operation.FinishedAt
		status.LastErrorCode = stableCapabilityErrorCode(operation.ErrorCode)
	case wecomcapability.OperationCancelled:
		status.State = providers.StateLimited
		status.Code = "wecom.capability_operation_cancelled"
		status.Reason = "WeCom 能力操作已取消，未形成完成证据"
		status.Action = "确认目标状态后重新执行 WeCom capability"
		status.LastFailureAt = operation.FinishedAt
		if status.LastFailureAt == nil {
			status.LastFailureAt = operation.UpdatedAt
		}
		status.LastErrorCode = "wecom.capability_operation_cancelled"
	}
}

func stableCapabilityErrorCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "wecom.capability_operation_failed"
	}
	return stableSyncErrorCode(value)
}

func applySyncStatus(status *providers.Status, syncStatus SyncStatus, expectedVersion uint64) {
	if status == nil {
		return
	}
	status.LastSyncAt = syncStatus.FinishedAt
	if status.LastSyncAt == nil {
		status.LastSyncAt = syncStatus.StartedAt
	}
	normalized := strings.ToLower(strings.TrimSpace(syncStatus.Status))
	if syncStatusVersionStale(normalized, syncStatus.CredentialVersion, expectedVersion) {
		applySyncStaleProviderStatus(status)
		return
	}
	switch normalized {
	case "completed", "succeeded", "success":
		if syncStatus.CredentialVersion == 0 || expectedVersion == 0 || syncStatus.CredentialVersion != expectedVersion {
			status.State = providers.StateLimited
			status.Code = "wecom.sync_stale"
			status.Reason = "最近的员工同步证据属于旧的企业凭据版本"
			status.Action = "使用当前企业凭据再次完成员工同步"
			status.LastErrorCode = "wecom.credential_version_stale"
			break
		}
		status.LastSuccessAt = syncStatus.FinishedAt
	case "failed":
		status.LastFailureAt = syncStatus.FinishedAt
		if status.LastFailureAt == nil {
			status.LastFailureAt = syncStatus.StartedAt
		}
		status.LastErrorCode = stableSyncErrorCode(syncStatus.ErrorCode)
	}
}

func applySyncStaleProviderStatus(status *providers.Status) {
	status.State = providers.StateLimited
	status.Code = "wecom.sync_stale"
	status.Reason = "同步证据属于旧的企业凭据版本"
	status.Action = "使用当前企业凭据再次完成同步"
	status.LastErrorCode = "wecom.credential_version_stale"
}

func stableSyncErrorCode(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "wecom.") && value != "wecom." {
		for _, character := range value {
			if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '.' || character == '_' || character == '-') {
				return "wecom.sync_failed"
			}
		}
		return value
	}
	return "wecom.sync_failed"
}

func tenantWeComArchiveStatus(profile Profile, runtime providers.Status) providers.Status {
	if status, unavailable := runtimeUnavailable(runtime, "wecom_archive", "archive_sync"); unavailable {
		return status
	}
	base := providers.Status{
		Kind:         "wecom_archive",
		Source:       providers.SourceExternal,
		Capabilities: []string{"archive_sync"},
		State:        providers.StateLimited,
	}
	if !profile.Credentials.Archive.Configured {
		base.Code = "archive.credentials_missing"
		base.Reason = "当前企业尚未配置会话存档凭据"
		base.Action = "在企业设置中配置会话存档凭据；真实同步仍需接入 getchatdata"
		base.Missing = []string{"会话存档凭据"}
		return base
	}
	base.Code = "archive.getchatdata_unimplemented"
	base.Reason = "真实会话存档 getchatdata source 尚未实现"
	base.Action = "接入并验证真实会话存档 source 后再启用同步"
	return base
}

func runtimeUnavailable(runtime providers.Status, kind, capability string) (providers.Status, bool) {
	if runtime.State == providers.StateReady || runtime.State == providers.StateLimited {
		return providers.Status{}, false
	}
	runtime.Kind = kind
	runtime.Source = providers.SourceExternal
	runtime.State = providers.StateUnavailable
	if strings.TrimSpace(runtime.Code) == "" || runtime.Code == "provider.status_unclassified" {
		runtime.Code = "provider.runtime_component_missing"
	}
	if len(runtime.Capabilities) == 0 {
		runtime.Capabilities = []string{capability}
	}
	return runtime, true
}

func findStatus(statuses []providers.Status, kind string) providers.Status {
	for _, status := range statuses {
		if status.Kind == kind {
			return status
		}
	}
	return providers.Status{Kind: kind}
}

func replaceStatus(statuses []providers.Status, replacement providers.Status) []providers.Status {
	for index := range statuses {
		if statuses[index].Kind == replacement.Kind {
			statuses[index] = replacement
			return statuses
		}
	}
	return append(statuses, replacement)
}
