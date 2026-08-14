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
		view.Providers = append(view.Providers, projectStatus(status, principal.IsSuperAdmin, visibleCapabilities))
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
		Capabilities:  append([]string(nil), status.Capabilities...),
		Missing:       append([]string(nil), status.Missing...),
		LastSyncAt:    status.LastSyncAt,
		LastSuccessAt: status.LastSuccessAt,
		LastFailureAt: status.LastFailureAt,
		LastErrorCode: optionalPublicMachineCode(status.LastErrorCode),
	}
	result.CapabilityStatuses = projectCapabilityStatuses(status.CapabilityStatuses, superadmin, visibleCapabilities)
	if superadmin {
		result.Reason = safeDiagnosticText(status.Reason, "")
		result.Action = safeDiagnosticText(status.Action, "请检查 Provider 运行时与租户配置")
		result.Missing = safeDiagnosticNames(status.Missing)
		return result
	}
	result.Reason = ""
	result.Missing = nil
	result.Action = "请联系管理员配置或验证 Provider"
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
			item.Reason = safeDiagnosticText(status.Reason, "")
			item.Action = safeDiagnosticText(status.Action, "请检查该 capability 的运行时与租户配置")
			item.Missing = safeDiagnosticNames(status.Missing)
		} else {
			item.Action = "请联系管理员配置或验证 WeCom capability"
		}
		result = append(result, item)
	}
	return result
}

func safeDiagnosticText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{"plaintext-secret", "access_token", "private_key", "authorization:", "bearer ", "secret=", "token="} {
		if strings.Contains(lower, marker) {
			return fallback
		}
	}
	return value
}

func safeDiagnosticNames(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if safe := safeDiagnosticText(value, ""); safe != "" {
			result = append(result, safe)
		}
	}
	return result
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
