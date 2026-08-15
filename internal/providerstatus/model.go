package providerstatus

import (
	"context"
	"errors"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
)

const (
	CodeSessionInvalid    = "SESSION_INVALID"
	CodeScopeDenied       = "PROVIDER_SCOPE_DENIED"
	CodeSourceUnavailable = "PROVIDER_SOURCE_UNAVAILABLE"
	CodeInternal          = "PROVIDER_STATUS_INTERNAL"
)

var (
	ErrScopeDenied       = errors.New("provider status scope denied")
	ErrSourceUnavailable = errors.New("provider status source unavailable")
)

// StatusSource is the runtime boundary. Implementations must resolve status
// from the authenticated tenant/corp and actual runtime components; HTTP
// request identifiers are never passed as authorization facts.
type StatusSource interface {
	Statuses(context.Context, dashboardprincipal.DashboardPrincipal) ([]providers.Status, error)
}

type ProviderStatus struct {
	Kind               string             `json:"kind"`
	State              providers.State    `json:"state"`
	Code               string             `json:"code"`
	Source             providers.Source   `json:"source"`
	Reason             string             `json:"reason,omitempty"`
	Action             string             `json:"action,omitempty"`
	Capabilities       []string           `json:"capabilities,omitempty"`
	Missing            []string           `json:"missing,omitempty"`
	LastSyncAt         *time.Time         `json:"lastSyncAt,omitempty"`
	LastSuccessAt      *time.Time         `json:"lastSuccessAt,omitempty"`
	LastFailureAt      *time.Time         `json:"lastFailureAt,omitempty"`
	LastErrorCode      string             `json:"lastErrorCode,omitempty"`
	CapabilityStatuses []CapabilityStatus `json:"capabilityStatuses,omitempty"`
}

type CapabilityStatus struct {
	Capability    string           `json:"capability"`
	State         providers.State  `json:"state"`
	Code          string           `json:"code,omitempty"`
	Source        providers.Source `json:"source,omitempty"`
	Reason        string           `json:"reason,omitempty"`
	Action        string           `json:"action,omitempty"`
	Missing       []string         `json:"missing,omitempty"`
	LastSyncAt    *time.Time       `json:"lastSyncAt,omitempty"`
	LastSuccessAt *time.Time       `json:"lastSuccessAt,omitempty"`
	LastFailureAt *time.Time       `json:"lastFailureAt,omitempty"`
	LastErrorCode string           `json:"lastErrorCode,omitempty"`
}

type View struct {
	Providers []ProviderStatus `json:"providers"`
	FreshAt   time.Time        `json:"freshAt"`
}
