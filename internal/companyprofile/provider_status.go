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
		profile.Credentials.WeCom.Configured &&
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
	if !profile.Credentials.WeCom.Configured {
		base.State = providers.StateLimited
		base.Code = "wecom.credentials_missing"
		base.Reason = "当前企业尚未配置企业微信标准同步凭据"
		base.Action = "在企业设置中配置并验证企业微信凭据"
		base.Missing = []string{"企业微信标准同步凭据"}
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
	applySyncStatus(&base, syncStatus, profile.BindingVersion)
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
	verified := profile.Credentials.WeCom.Configured && profile.BindingStatus == "active" && strings.TrimSpace(profile.WXCorpID) != "" && profile.VerifiedAt != nil
	status.CapabilityStatuses = standardCapabilityStatuses(profile, syncStatus, operations, verified)
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

func standardCapabilityStatuses(profile Profile, syncStatus SyncStatus, operations map[string]wecomcapability.Operation, verified bool) []providers.CapabilityStatus {
	statuses := make([]providers.CapabilityStatus, 0, len(wecomcapability.All))
	for _, capability := range wecomcapability.All {
		item := providers.CapabilityStatus{
			Capability: capability, State: providers.StateLimited, Source: providers.SourceExternal,
			Code: "wecom.capability_operation_pending", Action: "执行该 WeCom capability 的真实操作并等待结果",
		}
		if !profile.Credentials.WeCom.Configured {
			item.Code = "wecom.credentials_missing"
			item.Reason = "企业 WeCom 标准凭据未配置"
			item.Action = "配置并验证企业 WeCom 标准凭据"
			item.Missing = []string{"wecom_credentials"}
			statuses = append(statuses, item)
			continue
		}
		if !verified {
			item.Code = "wecom.runtime_unverified"
			item.Reason = "企业凭据已保存但尚未完成运行时验证"
			item.Action = "完成企业 WeCom 验证"
			statuses = append(statuses, item)
			continue
		}
		if capability == wecomcapability.AgentMessage && !profile.Credentials.Agent.Configured {
			item.Code = "wecom.agent_credentials_missing"
			item.Reason = "应用消息所需 agent 凭据未配置"
			item.Action = "配置并验证应用消息凭据"
			item.Missing = []string{"agent_credentials"}
			statuses = append(statuses, item)
			continue
		}
		if capability == wecomcapability.EmployeeSync {
			applyCapabilitySyncStatus(&item, syncStatus, profile.BindingVersion)
		} else if operation, ok := operations[capability]; ok && operation.CredentialVersion != 0 && operation.CredentialVersion == credentialVersionForCapability(profile, capability) {
			applyCapabilityOperation(&item, operation)
		}
		statuses = append(statuses, item)
	}
	return statuses
}

func credentialVersionForCapability(profile Profile, _ string) uint64 {
	// BindingVersion is incremented transactionally by every credential/config
	// mutation. The first milestone intentionally uses one conservative
	// generation for all standard capabilities; it never relies on KeyID or
	// timestamps, and a rotation therefore fences all old evidence.
	return profile.BindingVersion
}

func applyCapabilitySyncStatus(status *providers.CapabilityStatus, syncStatus SyncStatus, expectedVersion uint64) {
	if status == nil {
		return
	}
	status.LastSyncAt = syncStatus.FinishedAt
	if status.LastSyncAt == nil {
		status.LastSyncAt = syncStatus.StartedAt
	}
	switch strings.ToLower(strings.TrimSpace(syncStatus.Status)) {
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
	case "queued", "running":
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
	switch strings.ToLower(strings.TrimSpace(syncStatus.Status)) {
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
