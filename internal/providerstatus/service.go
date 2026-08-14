package providerstatus

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
)

var publicMachineCodePattern = regexp.MustCompile(`^(?:provider|archive|wecom|ai|audio_storage)(?:[._-][a-z0-9]+)*$|^WECOM_(?:API|HTTP)_ERROR_[0-9]+$`)

type Service struct {
	source StatusSource
	now    func() time.Time
}

func NewService(source StatusSource) *Service {
	return &Service{source: source, now: time.Now}
}

func (s *Service) WithClock(now func() time.Time) *Service {
	if s != nil && now != nil {
		s.now = now
	}
	return s
}

func (s *Service) Resolve(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) (View, error) {
	if !validReadPrincipal(principal) {
		return View{}, ErrScopeDenied
	}
	if s == nil || s.source == nil {
		return View{}, ErrSourceUnavailable
	}
	statuses, err := s.source.Statuses(ctx, principal)
	if err != nil {
		if errors.Is(err, ErrScopeDenied) || errors.Is(err, ErrSourceUnavailable) {
			return View{}, err
		}
		return View{}, fmt.Errorf("%w: %v", ErrSourceUnavailable, err)
	}
	view := View{Providers: make([]ProviderStatus, 0, len(statuses)), FreshAt: s.now().UTC()}
	visibleCapabilities := dashboardprincipal.VisibleProviderCapabilities(ctx, principal)
	for _, status := range statuses {
		projected := projectStatus(status, principal.IsSuperAdmin, visibleCapabilities)
		if !principal.IsSuperAdmin && len(projected.Capabilities) == 0 && len(projected.CapabilityStatuses) == 0 {
			continue
		}
		view.Providers = append(view.Providers, projected)
	}
	return view, nil
}

func validReadPrincipal(principal dashboardprincipal.DashboardPrincipal) bool {
	if principal.UserID <= 0 || principal.TenantID <= 0 || principal.CorpID <= 0 || principal.AuthVersion == 0 {
		return false
	}
	return principal.CorpStatus == dashboardprincipal.CorpBindingStatusPending || principal.CorpStatus == dashboardprincipal.CorpBindingStatusActive
}

func projectStatus(status providers.Status, superadmin bool, visibleCapabilities []string) ProviderStatus {
	state := status.State
	code := publicMachineCode(status.Code, "provider.status_unclassified")
	if state != providers.StateReady && state != providers.StateLimited && state != providers.StateUnavailable {
		state = providers.StateUnavailable
		code = "provider.invalid_state"
	}
	result := ProviderStatus{
		Kind:          strings.TrimSpace(status.Kind),
		State:         state,
		Code:          code,
		Source:        status.Source,
		Action:        safeDiagnosticText(status.Action, "请联系管理员配置或验证 Provider"),
		Capabilities:  projectProviderCapabilities(status.Capabilities, superadmin, visibleCapabilities),
		Missing:       append([]string(nil), status.Missing...),
		LastSyncAt:    status.LastSyncAt,
		LastSuccessAt: status.LastSuccessAt,
		LastFailureAt: status.LastFailureAt,
		LastErrorCode: optionalPublicMachineCode(status.LastErrorCode),
	}
	result.CapabilityStatuses = projectCapabilityStatuses(status.CapabilityStatuses, superadmin, visibleCapabilities)
	if superadmin {
		result.Reason = diagnosticReason(code)
		result.Action = diagnosticAction(code)
		result.Missing = safeDiagnosticNames(status.Missing)
		return result
	}
	result.Reason = ""
	result.Missing = nil
	result.Action = "请联系管理员配置或验证 Provider"
	return result
}

func projectProviderCapabilities(capabilities []string, superadmin bool, visibleCapabilities []string) []string {
	if superadmin {
		return append([]string(nil), capabilities...)
	}
	visible := make(map[string]struct{}, len(visibleCapabilities))
	for _, capability := range visibleCapabilities {
		visible[capability] = struct{}{}
	}
	result := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		if _, ok := visible[capability]; ok {
			result = append(result, capability)
		}
	}
	return result
}

func projectCapabilityStatuses(statuses []providers.CapabilityStatus, superadmin bool, visibleCapabilities []string) []CapabilityStatus {
	if len(statuses) == 0 {
		return nil
	}
	result := make([]CapabilityStatus, 0, len(statuses))
	visible := make(map[string]struct{}, len(visibleCapabilities))
	for _, capability := range visibleCapabilities {
		visible[capability] = struct{}{}
	}
	for _, status := range statuses {
		if !superadmin {
			if _, ok := visible[status.Capability]; !ok {
				continue
			}
		}
		state := status.State
		code := publicMachineCode(status.Code, "provider.status_unclassified")
		if state != providers.StateReady && state != providers.StateLimited && state != providers.StateUnavailable {
			state = providers.StateUnavailable
			code = "provider.invalid_state"
		}
		item := CapabilityStatus{
			Capability: strings.TrimSpace(status.Capability), State: state, Code: code, Source: status.Source,
			LastSyncAt: status.LastSyncAt, LastSuccessAt: status.LastSuccessAt,
			LastFailureAt: status.LastFailureAt, LastErrorCode: optionalPublicMachineCode(status.LastErrorCode),
		}
		if superadmin {
			item.Reason = diagnosticReason(code)
			item.Action = diagnosticAction(code)
			item.Missing = safeDiagnosticNames(status.Missing)
		} else {
			item.Action = "请联系管理员配置或验证 WeCom capability"
		}
		result = append(result, item)
	}
	return result
}

func safeDiagnosticText(value, fallback string) string {
	// Provider text is never an API contract. Keep this helper for callers
	// outside this file, but never pass through arbitrary runtime text.
	_ = value
	return fallback
}

func safeDiagnosticNames(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	allowed := map[string]struct{}{
		"wecom_credential_evidence": {}, "employee_credential": {}, "contact_credential": {},
		"agent_id": {}, "agent_secret": {}, "callback_token": {}, "callback_aes_key": {},
		"callback_route": {}, "callback_worker": {}, "archive_secret": {}, "archive_rsa_key": {},
	}
	for _, value := range values {
		if _, ok := allowed[strings.TrimSpace(value)]; ok {
			result = append(result, strings.TrimSpace(value))
		}
	}
	return result
}

var diagnosticReasons = map[string]string{
	"wecom.credentials_missing":           "企业微信凭据尚未配置",
	"wecom.employee_credentials_missing":  "员工同步凭据尚未配置",
	"wecom.contact_credentials_missing":   "客户能力凭据尚未配置",
	"wecom.agent_credentials_missing":     "应用消息凭据尚未配置",
	"wecom.callback_credentials_missing":  "回调凭据尚未配置完整",
	"wecom.callback_runtime_unconfigured": "回调运行时尚未启用",
	"wecom.callback_evidence_pending":     "尚未收到当前凭据版本的回调成功证据",
	"wecom.runtime_unverified":            "企业微信运行时尚未完成验证",
	"wecom.capabilities_pending":          "尚无标准能力的当前成功证据",
	"wecom.sync_stale":                    "最近同步证据属于旧凭据版本",
	"wecom.sync_failed":                   "最近一次同步失败",
	"wecom.capability_operation_failed":   "最近一次能力操作失败",
	"wecom.capability_operation_partial":  "最近一次能力操作部分成功",
	"wecom.capability_evidence_invalid":   "能力操作证据不完整或不属于当前凭据版本",
	"provider.runtime_component_missing":  "Provider 运行时组件不可用",
	"archive.credentials_missing":         "会话存档凭据尚未配置",
	"archive.getchatdata_unimplemented":   "真实会话存档同步尚未接入",
}

var diagnosticActions = map[string]string{
	"wecom.credentials_missing":           "在企业设置中配置并验证企业微信凭据",
	"wecom.employee_credentials_missing":  "配置并验证员工同步凭据",
	"wecom.contact_credentials_missing":   "配置并验证客户能力凭据",
	"wecom.agent_credentials_missing":     "配置并验证应用消息凭据",
	"wecom.callback_credentials_missing":  "配置并验证回调 token 与 AES key",
	"wecom.callback_runtime_unconfigured": "启用并验证企业微信回调运行时",
	"wecom.callback_evidence_pending":     "使用当前凭据版本接收并验证回调",
	"wecom.runtime_unverified":            "完成企业微信运行时验证",
	"wecom.capabilities_pending":          "执行对应能力的真实操作并等待结果",
	"wecom.sync_stale":                    "使用当前凭据再次完成同步",
	"wecom.sync_failed":                   "修复同步后重试",
	"wecom.capability_operation_failed":   "修复权限或凭据后重试",
	"wecom.capability_operation_partial":  "查看失败明细并重试失败目标",
	"wecom.capability_evidence_invalid":   "重新执行该能力并等待完整结果",
	"provider.runtime_component_missing":  "检查 Provider 运行时配置",
	"archive.credentials_missing":         "配置会话存档凭据",
	"archive.getchatdata_unimplemented":   "接入并验证真实会话存档 source",
}

func diagnosticReason(code string) string { return diagnosticReasons[code] }

func diagnosticAction(code string) string {
	if action := diagnosticActions[code]; action != "" {
		return action
	}
	return "检查 Provider 运行时与租户配置"
}

func publicMachineCode(value, fallback string) string {
	value = strings.TrimSpace(value)
	if publicMachineCodePattern.MatchString(value) {
		return value
	}
	return fallback
}

func optionalPublicMachineCode(value string) string {
	value = strings.TrimSpace(value)
	if value != "" && publicMachineCodePattern.MatchString(value) {
		return value
	}
	return ""
}
