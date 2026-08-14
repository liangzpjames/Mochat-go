package companyprofile

import (
	"context"
	"sort"
	"strings"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
)

// ProviderStatusSource binds the global runtime registry to the authenticated
// tenant's company profile. Credential values are never read into the result;
// only the store's non-secret configured/verified facts affect status.
type ProviderStatusSource struct {
	store    Store
	registry *providers.Registry
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
	if syncStore, ok := s.store.(SyncStore); ok {
		syncStatus, err = syncStore.GetSyncStatus(ctx, principal)
		if err != nil {
			return nil, err
		}
	}
	statuses := s.registry.Snapshot(ctx)
	statuses = replaceStatus(statuses, tenantWeComStandardStatus(profile, findStatus(statuses, "wecom_standard"), syncStatus))
	statuses = replaceStatus(statuses, tenantWeComArchiveStatus(profile, findStatus(statuses, "wecom_archive")))
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Kind < statuses[j].Kind })
	return statuses, nil
}

func tenantWeComStandardStatus(profile Profile, runtime providers.Status, syncStatus SyncStatus) providers.Status {
	if status, unavailable := runtimeUnavailable(runtime, "wecom_standard", "employee_sync"); unavailable {
		return status
	}
	base := providers.Status{
		Kind:         "wecom_standard",
		Source:       providers.SourceExternal,
		Capabilities: []string{"employee_sync"},
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
	applySyncStatus(&base, syncStatus)
	return base
}

func applySyncStatus(status *providers.Status, syncStatus SyncStatus) {
	if status == nil {
		return
	}
	status.LastSyncAt = syncStatus.FinishedAt
	if status.LastSyncAt == nil {
		status.LastSyncAt = syncStatus.StartedAt
	}
	switch strings.ToLower(strings.TrimSpace(syncStatus.Status)) {
	case "completed", "succeeded", "success":
		status.LastSuccessAt = syncStatus.FinishedAt
	case "failed":
		status.LastFailureAt = syncStatus.FinishedAt
		if status.LastFailureAt == nil {
			status.LastFailureAt = syncStatus.StartedAt
		}
		status.LastErrorCode = syncStatus.ErrorCode
	}
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
