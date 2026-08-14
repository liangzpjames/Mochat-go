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
	for _, status := range statuses {
		view.Providers = append(view.Providers, projectStatus(status, principal.IsSuperAdmin))
	}
	return view, nil
}

func validReadPrincipal(principal dashboardprincipal.DashboardPrincipal) bool {
	if principal.UserID <= 0 || principal.TenantID <= 0 || principal.CorpID <= 0 || principal.AuthVersion == 0 {
		return false
	}
	return principal.CorpStatus == dashboardprincipal.CorpBindingStatusPending || principal.CorpStatus == dashboardprincipal.CorpBindingStatusActive
}

func projectStatus(status providers.Status, superadmin bool) ProviderStatus {
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
		Action:        strings.TrimSpace(status.Action),
		Capabilities:  append([]string(nil), status.Capabilities...),
		Missing:       append([]string(nil), status.Missing...),
		LastSyncAt:    status.LastSyncAt,
		LastSuccessAt: status.LastSuccessAt,
		LastFailureAt: status.LastFailureAt,
		LastErrorCode: optionalPublicMachineCode(status.LastErrorCode),
	}
	if superadmin {
		result.Reason = strings.TrimSpace(status.Reason)
		return result
	}
	result.Reason = ""
	result.Missing = nil
	result.Action = "请联系管理员配置或验证 Provider"
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
